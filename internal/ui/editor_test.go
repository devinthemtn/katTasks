package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPaletteFindsSettingsAndConfig(t *testing.T) {
	for _, q := range []string{"settings", "config"} {
		m := scriptModel(t, nil)
		m.openPalette()
		m = typeText(m, q)
		if got := m.palItems[m.palShown[0]].name; got != q {
			t.Errorf(":%s picks %q first", q, got)
		}
	}
}

func TestEditorVar(t *testing.T) {
	m := scriptModel(t, nil)
	if v := m.editorVar(); v != "EDITOR" {
		t.Errorf("terminal uses $%s, want $EDITOR", v)
	}
	if v := NewHeadless(m, HeadlessOptions{}).m.editorVar(); v != "VISUAL" {
		t.Errorf("GUI uses $%s, want $VISUAL", v)
	}
}

// The GUI runs $VISUAL on the settings file or folder in the background,
// then re-reads the settings when it exits.
func TestGUIOpensVisual(t *testing.T) {
	for _, tc := range []struct{ action, want string }{{"settings", "config.toml"}, {"config", ""}} {
		m := scriptModel(t, nil)
		m.opts.ConfigPath = m.cfg.Path
		dir := filepath.Dir(m.opts.ConfigPath)
		out := filepath.Join(t.TempDir(), "opened")
		visual := "printf '%s' >" + out
		if runtime.GOOS == "windows" {
			visual = "echo>" + out // writes the quoted path and a line break
		}
		t.Setenv("VISUAL", visual)
		t.Setenv("EDITOR", "false")
		done := make(chan Snapshot, 1)
		h := NewHeadless(m, HeadlessOptions{Changed: func(s Snapshot) { done <- s }})
		h.Run(tc.action)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the editor never finished", tc.action)
		}
		b, _ := os.ReadFile(out)
		got := strings.Trim(string(b), "\" \r\n")
		if want := filepath.Join(dir, tc.want); filepath.Clean(got) != filepath.Clean(want) {
			t.Errorf("%s opened %q, want %q", tc.action, b, want)
		}
		if m := h.model(); m.statusErr || m.status == "" {
			t.Errorf("%s: after the editor, status %q (err %v); want the settings reloaded", tc.action, m.status, m.statusErr)
		}
	}
}
