package ui

import (
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	cases := map[string]struct {
		want  string
		chord bool
	}{
		"x": {"x", false}, "G": {"G", false}, "+": {"+", false},
		"space": {" ", false}, "Enter": {"enter", false}, "Page-Down": {"pgdown", false},
		"ctrl+U": {"ctrl+u", false}, "alt+x": {"alt+x", false}, "shift+tab": {"shift+tab", false},
		"gg": {"gg", true}, "yb": {"yb", true}, "f5": {"f5", false},
	}
	for in, c := range cases {
		got, chord, err := parseKey(in)
		if err != nil || got != c.want || chord != c.chord {
			t.Errorf("parseKey(%q) = %q %v %v, want %q %v", in, got, chord, err, c.want, c.chord)
		}
	}
	for _, bad := range []string{"", "ggg", "hyper+x", "ctrl+foo"} {
		if _, _, err := parseKey(bad); err == nil {
			t.Errorf("parseKey(%q) accepted", bad)
		}
	}
}

func TestBuildKeymapDefaults(t *testing.T) {
	km, errs := buildKeymap(nil)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	for k, want := range map[string]string{"j": "down", "g": "top", " ": "toggle_done", "ctrl+r": "reload_config", "a": "add"} {
		if a := km.single[k]; a == nil || a.name != want {
			t.Errorf("%q -> %v, want %s", k, a, want)
		}
	}
	// No two actions may share a default key.
	seen := map[string]string{}
	for _, a := range actions {
		for _, raw := range a.defaults {
			k, _, _ := parseKey(raw)
			if other, dup := seen[k]; dup {
				t.Errorf("key %q is a default for both %s and %s", k, other, a.name)
			}
			seen[k] = a.name
		}
	}
}

func TestBuildKeymapOverrides(t *testing.T) {
	km, errs := buildKeymap(map[string][]string{
		"top":    {"gg"},
		"undo":   {"x"}, // takes x away from toggle_done
		"nope":   {"z"},
		"delete": {"ctrl+nope"},
	})
	msgs := ""
	for _, e := range errs {
		msgs += e.Error() + "\n"
	}
	if !strings.Contains(msgs, "keys.nope: unknown action") || !strings.Contains(msgs, "keys.delete") {
		t.Errorf("errors:\n%s", msgs)
	}
	if a := km.chords["gg"]; a == nil || a.name != "top" || !km.prefixes["g"] {
		t.Error("gg chord not bound to top")
	}
	if km.single["g"] != nil {
		t.Error("g still bound after top was rebound")
	}
	if a := km.single["x"]; a == nil || a.name != "undo" {
		t.Errorf("x -> %v, want undo", a)
	}
	if a := km.single[" "]; a == nil || a.name != "toggle_done" {
		t.Error("toggle_done lost its other default key")
	}
	if a := km.single["u"]; a != nil {
		t.Errorf("u still bound to %s after undo was rebound", a.name)
	}
	if a := km.single["d"]; a == nil || a.name != "delete" {
		t.Error("delete lost its default after an invalid override")
	}
	km, _ = buildKeymap(map[string][]string{"quit": {}})
	if km.single["q"] != nil {
		t.Error("quit = [] should unbind q")
	}
}
