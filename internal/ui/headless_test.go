package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/theme"
)

func TestParseKeyMsgRoundTrip(t *testing.T) {
	for _, k := range []string{"j", "G", " ", "enter", "esc", "tab", "shift+tab", "ctrl+d", "ctrl+r", "up", "pgdown", "backspace", "alt+b", "alt+backspace", "ctrl+left", ":", "?", "f5"} {
		msg, ok := ParseKeyMsg(k)
		if !ok {
			t.Errorf("%q: not parsed", k)
			continue
		}
		if got := msg.String(); got != k {
			t.Errorf("%q: round trip gave %q", k, got)
		}
	}
	for _, k := range []string{"", "jj", "ctrl+nope"} {
		if _, ok := ParseKeyMsg(k); ok {
			t.Errorf("%q: parsed", k)
		}
	}
}

func headless(t *testing.T) (*Headless, *[]string) {
	t.Helper()
	var copied []string
	h := NewHeadless(scriptModel(t, nil), HeadlessOptions{Copy: func(s string) error { copied = append(copied, s); return nil }})
	h.Resize(100, 20)
	return h, &copied
}

func keys(h *Headless, ks ...string) Snapshot {
	var s Snapshot
	for _, k := range ks {
		s = h.Key(k)
	}
	return s
}

func TestHeadlessAddAndToggle(t *testing.T) {
	h, _ := headless(t)
	s := keys(h, "a")
	if s.Mode != "add" || s.Input == nil || s.Input.Label != "New task" {
		t.Fatalf("after a: mode %q input %+v", s.Mode, s.Input)
	}
	for _, r := range "(B) Call mom +home" {
		s = h.Key(string(r))
	}
	if s.Input.Value != "(B) Call mom +home" || s.Input.Pos != len("(B) Call mom +home") {
		t.Fatalf("typed %+v", s.Input)
	}
	s = h.Key("enter")
	if s.Mode != "normal" || s.Total != 2 {
		t.Fatalf("after enter: mode %q total %d", s.Mode, s.Total)
	}
	// Default sort is priority, so the new (B) task is first and has the cursor.
	r := s.Rows[0]
	if !r.Cursor || r.Priority != "B" || r.PriStyle != "pri_b" || r.Words[len(r.Words)-1] != (Word{Text: "+home", Style: "project"}) {
		t.Fatalf("row %+v", r)
	}
	s = h.Key("x")
	if !strings.HasPrefix(s.Footer[0][0].Text, "completed") || s.Footer[0][0].Style != "ok" {
		t.Fatalf("status %+v", s.Footer[0])
	}
	data, _ := os.ReadFile(h.m.cfg.Files.Todo)
	if !strings.Contains(string(data), "x ") {
		t.Fatalf("not saved as done:\n%s", data)
	}
}

func TestHeadlessChordAndCopy(t *testing.T) {
	h, copied := headless(t)
	changed := make(chan Snapshot, 4)
	h.hop.Changed = func(s Snapshot) { changed <- s }

	s := h.Key("y")
	if s.Footer[1][0].Text != "y…" {
		t.Fatalf("pending chord not shown: %+v", s.Footer[1])
	}
	h.Key("y")
	select {
	case s = <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("no update after copy")
	}
	if len(*copied) != 1 || (*copied)[0] != "Old task" {
		t.Fatalf("copied %q", *copied)
	}
	if s.Footer[0][0].Text != "copied 1 line" {
		t.Fatalf("status %+v", s.Footer[0])
	}
}

func TestHeadlessClicks(t *testing.T) {
	h, _ := headless(t)
	keys(h, "a", "N", "e", "w", " ", "@", "w", "o", "r", "k", "enter")
	s := h.State()
	ctx := -1
	for _, r := range s.Side {
		if r.Kind == "context" && r.Label == "@work" {
			ctx = r.Index
		}
	}
	if ctx < 0 {
		t.Fatalf("no @work in sidebar: %+v", s.Side)
	}
	s = h.ClickSidebar(ctx)
	if s.Focus != "sidebar" || s.Total != 1 {
		t.Fatalf("after sidebar click: focus %q total %d", s.Focus, s.Total)
	}
	s = h.ClickTask(0, true, false)
	if s.Mode != "edit" || s.Input.Value != "New @work" {
		t.Fatalf("double click: mode %q input %+v", s.Mode, s.Input)
	}
	s = h.Key("esc")
	s = h.Run("palette")
	if s.Mode != "palette" || len(s.Palette) == 0 {
		t.Fatalf("palette: %q %d", s.Mode, len(s.Palette))
	}
}

func TestCSSColor(t *testing.T) {
	for in, want := range map[string]string{"": "", "#abc": "#AABBCC", "#a1b2c3": "#A1B2C3", "1": "#CD0000", "16": "#000000", "231": "#FFFFFF", "244": "#808080"} {
		if got := cssColor(in); got != want {
			t.Errorf("cssColor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCSSColorsBackground(t *testing.T) {
	themes, _ := theme.Load(t.TempDir())
	for name, want := range map[string]string{"dracula": "#282A36", "nord": "#2E3440"} {
		th, _ := theme.Find(themes, name)
		if c := cssColors(th); c.Dark["bg"] != want || c.Light["bg"] != want {
			t.Errorf("%s: bg %q / %q, want %q", name, c.Light["bg"], c.Dark["bg"], want)
		}
	}
	th, _ := theme.Find(themes, "default")
	if c := cssColors(th); c.Light["bg"] != "#FFFFFF" || c.Dark["bg"] != "#18181F" {
		t.Errorf("default: bg %q / %q", c.Light["bg"], c.Dark["bg"])
	}
}

func TestCSSFontFamily(t *testing.T) {
	if got := cssFontFamily(""); !strings.HasPrefix(got, `"JetBrains Mono", `) || !strings.HasSuffix(got, ", monospace") {
		t.Errorf("default: %s", got)
	}
	got := cssFontFamily(` Fira Code , 'Iosevka Term', Monospace, menlo`)
	if !strings.HasPrefix(got, `"Fira Code", "Iosevka Term", monospace, "menlo", "JetBrains Mono"`) || strings.Count(got, "monospace") != 1 || strings.Contains(got, `"Menlo"`) {
		t.Errorf("custom: %s", got)
	}
}

func TestHeadlessDoingAndProgress(t *testing.T) {
	h, _ := headless(t) // one task, "Old task"
	s := keys(h, "i", ">", ">", ">", ">")
	r := s.Rows[0]
	if r.Status != "▶ doing" || !r.Doing || r.Progress != "▰▰▱▱▱ 40%" {
		t.Fatalf("row %+v", r)
	}
	for _, w := range r.Words {
		if strings.Contains(w.Text, ":") {
			t.Errorf("tag %q shown in the text as well as the badge", w.Text)
		}
	}
	data, _ := os.ReadFile(h.m.cfg.Files.Todo)
	if strings.TrimSpace(string(data)) != "Old task status:doing progress:40" {
		t.Fatalf("saved %q", data)
	}
	if !strings.Contains(h.m.renderTask(h.m.tasks[0], row{}, 100, time.Now()), "▶ doing ▰▰▱▱▱ 40%") {
		t.Error("terminal row has no badge")
	}

	s = keys(h, "<", "<", "<", "<", "i")
	if r := s.Rows[0]; r.Status != "" || r.Progress != "" {
		t.Fatalf("after stopping: %+v", r)
	}
	data, _ = os.ReadFile(h.m.cfg.Files.Todo)
	if strings.TrimSpace(string(data)) != "Old task" {
		t.Fatalf("saved %q", data)
	}

	// Done tasks keep the tags but show them as words, not badges.
	s = keys(h, "i", ">", "x", "H") // H: completed tasks are hidden by default
	if r := s.Rows[0]; !r.Done || r.Status != "" || r.Progress != "" || len(r.Words) != 4 {
		t.Fatalf("done row %+v", r)
	}
}
