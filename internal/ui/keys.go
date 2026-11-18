package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// chordTimeout is how long the first key of a chord waits for the second.
const chordTimeoutMS = 600

// keymap resolves pressed keys to actions.
type keymap struct {
	single   map[string]*action
	chords   map[string]*action // two-key sequences such as "yy"
	prefixes map[string]bool    // first keys of chords
	keys     map[string][]string
}

var namedKeys = map[string]string{
	"enter": "enter", "return": "enter", "esc": "esc", "escape": "esc", "tab": "tab",
	"space": " ", "backspace": "backspace", "delete": "delete", "del": "delete", "insert": "insert",
	"up": "up", "down": "down", "left": "left", "right": "right", "home": "home", "end": "end",
	"pgup": "pgup", "pageup": "pgup", "page-up": "pgup", "pgdown": "pgdown", "pagedown": "pgdown", "page-down": "pgdown",
}

func init() {
	for i := 1; i <= 20; i++ {
		f := fmt.Sprintf("f%d", i)
		namedKeys[f] = f
	}
}

// parseKey normalizes a key name from the config to the form Bubble Tea's
// KeyMsg.String() produces. chord reports a two-key sequence.
func parseKey(s string) (k string, chord bool, err error) {
	if s == "" {
		return "", false, fmt.Errorf("empty key")
	}
	if utf8.RuneCountInString(s) == 1 {
		return s, false, nil
	}
	if n, ok := namedKeys[strings.ToLower(s)]; ok {
		return n, false, nil
	}
	if i := strings.LastIndex(s, "+"); i > 0 && i < len(s)-1 {
		mods := strings.Split(strings.ToLower(s[:i]), "+")
		for _, mod := range mods {
			if mod != "ctrl" && mod != "alt" && mod != "shift" {
				return "", false, fmt.Errorf("unknown modifier %q", mod)
			}
		}
		last := s[i+1:]
		if n, ok := namedKeys[strings.ToLower(last)]; ok {
			last = n
			if last == " " {
				last = "space"
			}
		} else if utf8.RuneCountInString(last) != 1 {
			return "", false, fmt.Errorf("unknown key %q", last)
		} else if mods[0] == "ctrl" {
			last = strings.ToLower(last)
		}
		return strings.Join(mods, "+") + "+" + last, false, nil
	}
	if utf8.RuneCountInString(s) == 2 {
		return s, true, nil
	}
	return "", false, fmt.Errorf("unknown key %q", s)
}

// displayKey is how a key is shown in help.
func displayKey(k string) string {
	if k == " " {
		return "space"
	}
	return k
}

// buildKeymap combines each action's defaults with the [keys] overrides. An
// overridden action uses exactly the keys given ([] unbinds it); a key given
// to one action is removed from every other action's defaults. An override
// whose keys are all invalid is reported and the defaults are kept.
func buildKeymap(overrides map[string][]string) (keymap, []error) {
	var errs []error
	want := map[string][]string{}
	taken := map[string]bool{}

	names := make([]string, 0, len(overrides))
	for n := range overrides {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := actionByName[name]; !ok {
			errs = append(errs, fmt.Errorf("keys.%s: unknown action (press : to list them)", name))
			continue
		}
		var ks []string
		for _, raw := range overrides[name] {
			k, _, err := parseKey(raw)
			if err != nil {
				errs = append(errs, fmt.Errorf("keys.%s: %v", name, err))
				continue
			}
			ks = append(ks, k)
		}
		if len(ks) == 0 && len(overrides[name]) > 0 {
			continue // all invalid: keep the defaults rather than lose the action
		}
		want[name] = ks
		for _, k := range ks {
			taken[k] = true
		}
	}
	for _, a := range actions {
		if _, ok := want[a.name]; ok {
			continue
		}
		for _, raw := range a.defaults {
			k, _, _ := parseKey(raw)
			if !taken[k] {
				want[a.name] = append(want[a.name], k)
			}
		}
	}

	km := keymap{single: map[string]*action{}, chords: map[string]*action{}, prefixes: map[string]bool{}, keys: want}
	for _, a := range actions {
		for _, k := range want[a.name] {
			if utf8.RuneCountInString(k) == 2 && !strings.Contains(k, "+") && namedKeys[k] == "" {
				km.chords[k] = a
				r, _ := utf8.DecodeRuneInString(k)
				km.prefixes[string(r)] = true
			} else {
				km.single[k] = a
			}
		}
	}
	return km, errs
}

// keysFor returns the display form of an action's keys.
func (km keymap) keysFor(name string) []string {
	var out []string
	for _, k := range km.keys[name] {
		out = append(out, displayKey(k))
	}
	return out
}
