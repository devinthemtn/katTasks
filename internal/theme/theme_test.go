package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinsComplete(t *testing.T) {
	for _, b := range Builtins {
		for _, f := range Fields {
			c, ok := b.Colors[f.Key]
			if !ok {
				t.Errorf("%s: missing %s", b.Name, f.Key)
			}
			if !validColor(c.Light) || !validColor(c.Dark) {
				t.Errorf("%s.%s: bad color %v", b.Name, f.Key, c)
			}
		}
		if len(b.Colors) != len(Fields) {
			t.Errorf("%s: %d colors, want %d", b.Name, len(b.Colors), len(Fields))
		}
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(dir), name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCustom(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mine.toml", "base = \"nord\"\naccent = \"#ff0000\"\nfg = { light = \"#000\", dark = \"#fff\" }\n")
	write(t, dir, "child.toml", "base = \"mine\"\nproject = \"2\"\n")
	write(t, dir, "broken.toml", "accent = \"red\"\nsparkle = \"#fff\"\n")
	write(t, dir, "loop.toml", "base = \"loop2\"\n")
	write(t, dir, "loop2.toml", "base = \"loop\"\n")
	write(t, dir, "nord.toml", "base = \"nord\"\ndim = \"#111111\"\n")

	themes, warns := Load(dir)
	names := []string{}
	for _, th := range themes {
		names = append(names, th.Name)
	}
	if got := strings.Join(names, ","); got != "default,terminal,gruvbox,child,mine,nord" {
		t.Errorf("themes = %s", got)
	}
	w := ""
	for _, e := range warns {
		w += e.Error() + "\n"
	}
	for _, want := range []string{"broken.toml", `"red" is not a color`, `unknown color "sparkle"`, "loop"} {
		if !strings.Contains(w, want) {
			t.Errorf("warnings missing %q:\n%s", want, w)
		}
	}

	mine, _ := Find(themes, "mine")
	if mine.Get("accent") != Plain("#ff0000") || mine.Get("fg") != (Color{"#000", "#fff"}) || mine.Get("project") != Plain("#A3BE8C") {
		t.Errorf("mine = %+v", mine.Colors)
	}
	child, _ := Find(themes, "child")
	if child.Get("accent") != Plain("#ff0000") || child.Get("project") != Plain("2") {
		t.Errorf("child did not inherit: %+v", child.Colors)
	}
	nord, _ := Find(themes, "nord")
	if nord.Get("dim") != Plain("#111111") || nord.Get("accent") != Plain("#88C0D0") {
		t.Errorf("nord override: %+v", nord.Colors)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	def, _ := Find(Builtins, "default")
	if _, err := Write(dir, "copy", def); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "copy", def); err == nil {
		t.Error("overwrote existing theme")
	}
	themes, warns := Load(dir)
	if len(warns) != 0 {
		t.Fatal(warns)
	}
	cp, ok := Find(themes, "copy")
	if !ok {
		t.Fatal("copy not loaded")
	}
	for _, f := range Fields {
		if cp.Get(f.Key) != def.Get(f.Key) {
			t.Errorf("%s: %v != %v", f.Key, cp.Get(f.Key), def.Get(f.Key))
		}
	}
}
