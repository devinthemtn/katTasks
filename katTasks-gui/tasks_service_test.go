package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// noApp is the window before it exists (or in tests): the service must
// cope with no clipboard, events or quit.
func noApp() *application.App { return nil }

func newService(t *testing.T, todo string) (*TasksService, string, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config", "config.toml")
	todoPath := filepath.Join(dir, "todo.txt")
	os.WriteFile(todoPath, []byte(todo), 0o644)
	return NewTasksService(cfg, todoPath, "test", noApp), cfg, todoPath
}

func TestServiceStartsAndWritesDefaultSettings(t *testing.T) {
	s, cfg, _ := newService(t, "(A) first\nsecond\n")
	if s.StartupError() != "" {
		t.Fatal(s.StartupError())
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Errorf("default settings not written: %v", err)
	}
	snap := s.Resize(100, 20)
	if snap.Total != 2 || len(snap.Rows) != 2 || snap.Rows[0].Priority != "A" {
		t.Fatalf("total %d rows %+v", snap.Total, snap.Rows)
	}
	var footer strings.Builder
	for _, line := range snap.Footer {
		for _, seg := range line {
			footer.WriteString(seg.Text)
		}
	}
	if !strings.Contains(footer.String(), "wrote default settings") {
		t.Errorf("footer %q doesn't say the settings were written", footer.String())
	}
	if s.ConfigPath() != cfg {
		t.Errorf("ConfigPath %q", s.ConfigPath())
	}
}

// Keys from the window change todo.txt, as in the terminal.
func TestServiceKeysEditTheFile(t *testing.T) {
	s, _, todoPath := newService(t, "one\n")
	s.Resize(100, 20)
	s.Key("a")
	s.Paste("two  +home\n") // one line, spaces collapsed
	if snap := s.Key("enter"); snap.Mode != "normal" || snap.Total != 2 {
		t.Fatalf("after adding: mode %q total %d", snap.Mode, snap.Total)
	}
	b, _ := os.ReadFile(todoPath)
	if !strings.Contains(string(b), "two +home") {
		t.Errorf("todo.txt:\n%s", b)
	}

	// Clicking a row's box marks it done.
	snap := s.State()
	idx := -1
	for _, r := range snap.Rows {
		if len(r.Words) > 0 && r.Words[0].Text == "one" {
			idx = r.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no row for one in %+v", snap.Rows)
	}
	s.ClickTask(idx, false, true)
	b, _ = os.ReadFile(todoPath)
	if !strings.Contains(string(b), "x ") || !strings.Contains(string(b), " one") {
		t.Errorf("after clicking the box, todo.txt:\n%s", b)
	}
}

// If the todo file can't be read, the window shows why, and every call
// still answers instead of crashing the app.
func TestServiceStartupError(t *testing.T) {
	dir := t.TempDir()
	todoPath := filepath.Join(dir, "todo.txt")
	os.Mkdir(todoPath, 0o755) // a folder, not a file
	s := NewTasksService(filepath.Join(dir, "config.toml"), todoPath, "test", noApp)
	if !strings.Contains(s.StartupError(), "couldn't open") {
		t.Fatalf("StartupError %q", s.StartupError())
	}
	for name, call := range map[string]func(){
		"State": func() { s.State() }, "Resize": func() { s.Resize(80, 20) }, "Key": func() { s.Key("j") },
		"Paste": func() { s.Paste("x") }, "ClickTask": func() { s.ClickTask(0, true, true) },
		"ClickCalendarTask": func() { s.ClickCalendarTask(0, true) }, "AgendaWeek": func() { s.AgendaWeek(1) },
		"CalendarMonth": func() { s.CalendarMonth(1) }, "ClickSidebar": func() { s.ClickSidebar(0) },
		"Scroll": func() { s.Scroll(3, false) }, "Run": func() { s.Run("help") },
		"Pick": func() { s.Pick(0, true) }, "SetInputCursor": func() { s.SetInputCursor(0) },
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", name, r)
				}
			}()
			call()
		}()
	}
}

// -file replaces files.todo from the settings.
func TestServiceFileFlagOverridesSettings(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	other := filepath.Join(dir, "elsewhere", "todo.txt")
	os.WriteFile(cfg, []byte("[files]\ntodo = '"+filepath.Join(dir, "settings-todo.txt")+"'\n"), 0o644)
	os.MkdirAll(filepath.Dir(other), 0o755)
	os.WriteFile(other, []byte("from the flag\n"), 0o644)
	s := NewTasksService(cfg, other, "test", noApp)
	if snap := s.Resize(100, 20); snap.Total != 1 {
		t.Fatalf("total %d, want the -file todo.txt's one task", snap.Total)
	}
}
