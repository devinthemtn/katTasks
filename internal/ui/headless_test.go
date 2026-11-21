package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/theme"
	"github.com/devinthemtn/katTasks/internal/todo"
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
	data, _ := os.ReadFile(h.model().cfg.Files.Todo)
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
	data, _ := os.ReadFile(h.model().cfg.Files.Todo)
	if strings.TrimSpace(string(data)) != "Old task status:doing progress:40" {
		t.Fatalf("saved %q", data)
	}
	var line string
	h.with(func(m *Model) { line = m.renderTask(m.tasks[0], row{}, 100, time.Now()) })
	if !strings.Contains(line, "▶ doing ▰▰▱▱▱ 40%") {
		t.Error("terminal row has no badge")
	}

	s = keys(h, "<", "<", "<", "<", "i")
	if r := s.Rows[0]; r.Status != "" || r.Progress != "" {
		t.Fatalf("after stopping: %+v", r)
	}
	data, _ = os.ReadFile(h.model().cfg.Files.Todo)
	if strings.TrimSpace(string(data)) != "Old task" {
		t.Fatalf("saved %q", data)
	}

	// Done tasks keep the tags but show them as words, not badges.
	s = keys(h, "i", ">", "x", "H") // H: completed tasks are hidden by default
	if r := s.Rows[0]; !r.Done || r.Status != "" || r.Progress != "" || len(r.Words) != 4 {
		t.Fatalf("done row %+v", r)
	}
}

func TestAtSuggestions(t *testing.T) {
	now := time.Date(2026, 11, 19, 14, 7, 0, 0, time.Local)
	firsts := func(w string, now time.Time, n int) []string { return atSuggestions(w, now)[:n] }
	cases := []struct {
		w    string
		now  time.Time
		want []string
	}{
		{"at:", now, []string{"at:1430", "at:1500", "at:1530"}},
		{"at:", time.Date(2026, 11, 19, 23, 50, 0, 0, time.Local), []string{"at:0000", "at:0030"}},
		{"at:9", now, []string{"at:1415", "at:1430"}}, // unfiltered; updateSuggestions filters
		{"at:3p", now, []string{"at:215pm", "at:230pm"}},
	}
	for _, c := range cases {
		if got := firsts(c.w, c.now, len(c.want)); strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%q: got %q, want %q", c.w, got, c.want)
		}
	}
}

func TestHeadlessAtCompletion(t *testing.T) {
	h, _ := headless(t)
	s := keys(h, "a")
	for _, r := range "Lunch at:9" {
		s = h.Key(string(r))
	}
	want := map[string]bool{"at:900": true, "at:915": true, "at:930": true, "at:945": true}
	if len(s.Input.Sugg) != 4 {
		t.Fatalf("suggestions %q", s.Input.Sugg)
	}
	for _, g := range s.Input.Sugg {
		if !want[g] {
			t.Errorf("suggestion %q doesn't fit at:9", g)
		}
	}
	s = h.Key("tab")
	got := strings.TrimSpace(strings.TrimPrefix(s.Input.Value, "Lunch at:"))
	if strings.Contains(got, ":") {
		t.Errorf("inserted a colon: %q", s.Input.Value)
	}
	if min, ok := todo.ParseClock(got, true); !ok || min/60 != 9 {
		t.Errorf("inserted time doesn't parse as 9:xx: %q", s.Input.Value)
	}

	// The old colon form still finds them.
	for _, r := range " at:14:3" {
		s = h.Key(string(r))
	}
	if len(s.Input.Sugg) == 0 || s.Input.Sugg[0] != "at:1430" {
		t.Fatalf("14:3 suggestions %q", s.Input.Sugg)
	}

	// The ones for 3p are 12-hour times that parse too.
	for _, r := range " at:3p" {
		s = h.Key(string(r))
	}
	if len(s.Input.Sugg) == 0 || s.Input.Sugg[0] != "at:3pm" {
		t.Fatalf("3p suggestions %q", s.Input.Sugg)
	}
}

func TestAlarmPicker(t *testing.T) {
	h, _ := headless(t)
	s := keys(h, "a")
	for _, r := range "Wake up alarm:" {
		s = h.Key(string(r))
	}
	a := s.Input.Alarm
	if a == nil || a.Field != "date" || len(a.Days) != 42 {
		t.Fatalf("picker not open: %+v", s.Input)
	}
	start := time.Now()
	if start.Hour()*60+start.Minute() >= 7*60 {
		start = start.AddDate(0, 0, 1)
	}
	if a.Time != "7:00" || !strings.Contains(a.When, start.Format("Jan 2")) {
		t.Fatalf("starts at %q %q", a.When, a.Time)
	}
	if !strings.Contains(h.view(), "⏰ ALARM") {
		t.Error("terminal popup doesn't show the picker")
	}

	// Next day, then the time field: type 6:45.
	s = keys(h, "right", "tab", "6", "4", "5")
	if s.Input.Alarm.Field != "time" || s.Input.Alarm.Time != "6:45" {
		t.Fatalf("time %+v", s.Input.Alarm)
	}
	s = keys(h, "up") // +15 min
	s = h.Key("enter")
	want := "Wake up alarm:" + start.AddDate(0, 0, 1).Format(todo.DateLayout) + "T0700 "
	if s.Input.Alarm != nil || s.Input.Value != want {
		t.Fatalf("value %q, want %q", s.Input.Value, want)
	}

	// tab on the alarm: word opens it again where it was.
	s = keys(h, "backspace", "tab")
	if s.Input.Alarm == nil || s.Input.Alarm.Time != "7:00" {
		t.Fatalf("reopen %+v", s.Input.Alarm)
	}
	s = h.Key("esc") // keeps the value
	if s.Input.Alarm != nil || strings.TrimSpace(s.Input.Value) != strings.TrimSpace(want) {
		t.Fatalf("after esc %q", s.Input.Value)
	}

	// esc on a fresh alarm: takes it back.
	for _, r := range " alarm:" {
		s = h.Key(string(r))
	}
	s = h.Key("esc")
	if strings.Contains(strings.TrimPrefix(s.Input.Value, strings.TrimSpace(want)), "alarm:") {
		t.Fatalf("bare alarm: left: %q", s.Input.Value)
	}

	// Saving shows the badge instead of the tag.
	s = h.Key("enter")
	var row *TaskRow
	for i := range s.Rows {
		if strings.HasPrefix(s.Rows[i].Words[0].Text, "Wake") {
			row = &s.Rows[i]
		}
	}
	if row == nil || !strings.HasPrefix(row.Alarm, "⏰ ") || !strings.HasSuffix(row.Alarm, "7:00") {
		t.Fatalf("row %+v", row)
	}
	for _, w := range row.Words {
		if strings.HasPrefix(w.Text, "alarm:") {
			t.Error("alarm: tag shown as text too")
		}
	}
}

func TestAlarmPickerClick(t *testing.T) {
	h, _ := headless(t)
	keys(h, "a")
	for _, r := range "x alarm:" {
		h.Key(string(r))
	}
	s := h.Pick(20, true) // a day in the third week of the grid
	if s.Input.Alarm != nil || !strings.Contains(s.Input.Value, "alarm:") || !strings.Contains(s.Input.Value, "T0700") {
		t.Fatalf("after click %q", s.Input.Value)
	}
}

func TestHelpScrolls(t *testing.T) {
	h, _ := headless(t)
	h.Resize(100, 14) // too short for the help screen
	keys(h, "?")
	first := h.view()
	if !strings.Contains(first, "↓ more") || strings.Contains(first, "TAGS & SEARCH") {
		t.Fatalf("top of a short help screen:\n%s", first)
	}
	keys(h, "G")
	if v := h.view(); !strings.Contains(v, "TAGS & SEARCH") || !strings.Contains(v, "the end") {
		t.Fatalf("bottom:\n%s", v)
	}
	if !h.model().showHelp {
		t.Fatal("scrolling closed the help")
	}
	s := keys(h, "g", "j", "k", "x") // x isn't a scroll key: it closes
	if h.model().showHelp || s.Help != nil {
		t.Error("x didn't close the help")
	}

	// Tall enough: everything shows and any key, j included, closes it.
	h.Resize(200, 80)
	keys(h, "?")
	if v := h.view(); !strings.Contains(v, "TAGS & SEARCH") || strings.Contains(v, "↓ more") {
		t.Fatalf("tall help:\n%s", v)
	}
	if s := keys(h, "?"); len(s.HelpTags) != 0 {
		t.Error("? didn't close it")
	}
	s = keys(h, "?")
	if len(s.HelpTyping) == 0 || len(s.HelpTags) == 0 {
		t.Error("snapshot lacks the help text for the GUI")
	}
	keys(h, "j")
	if h.model().showHelp {
		t.Error("j didn't close a help screen that fits")
	}
}

func TestSuggestionArrowsAndEnter(t *testing.T) {
	h, _ := headless(t)
	s := keys(h, "a")
	for _, r := range "Lunch at:9" {
		s = h.Key(string(r))
	}
	if s.Input.SuggIdx != 0 || len(s.Input.Sugg) < 2 {
		t.Fatalf("suggestions %q at %d", s.Input.Sugg, s.Input.SuggIdx)
	}
	if s = h.Key("right"); s.Input.SuggIdx != 1 {
		t.Fatalf("right: at %d", s.Input.SuggIdx)
	}
	if s = h.Key("left"); s.Input.SuggIdx != 0 {
		t.Fatalf("left: at %d", s.Input.SuggIdx)
	}
	if s = h.Key("left"); s.Input.SuggIdx != len(s.Input.Sugg)-1 {
		t.Fatalf("left wraps: at %d", s.Input.SuggIdx)
	}
	pick := s.Input.Sugg[s.Input.SuggIdx]
	s = h.Key("enter") // completes, doesn't save
	if s.Input == nil || s.Input.Value != "Lunch "+pick+" " {
		t.Fatalf("enter: input %+v, want %q completed", s.Input, pick)
	}
	if s = h.Key("enter"); s.Input != nil { // no suggestions now: saves
		t.Fatalf("second enter kept the input open: %+v", s.Input)
	}
}

func TestConfirmButtons(t *testing.T) {
	h, _ := headless(t)
	h.with(func(m *Model) { m.ask("delete it?", func(m *Model) { m.setStatus("deleted") }) })
	s := h.State()
	if s.Mode != "confirm" || s.ConfirmNo {
		t.Fatalf("yes should be highlighted first: %+v", s.Mode)
	}
	if s = h.Key("enter"); s.Mode != "normal" || h.model().status != "deleted" {
		t.Fatalf("enter on yes: mode %s, status %q", s.Mode, h.model().status)
	}

	h.with(func(m *Model) { m.ask("delete it?", func(m *Model) { m.setStatus("deleted") }) })
	if s = h.Key("right"); !s.ConfirmNo {
		t.Fatal("right should highlight no")
	}
	if s = h.Key("enter"); s.Mode != "normal" || h.model().status != "cancelled" {
		t.Fatalf("enter on no: mode %s, status %q", s.Mode, h.model().status)
	}

	h.with(func(m *Model) { m.ask("delete it?", func(m *Model) { m.setStatus("deleted") }) })
	h.Key("tab")
	if s = h.Key("y"); h.model().status != "deleted" {
		t.Fatalf("y answers yes whatever is highlighted: %q", h.model().status)
	}
}

func TestDuePickerOnLongTask(t *testing.T) {
	long := strings.Repeat("a long task about many things ", 6) + "end"
	typeIt := func(h *Headless, text string) (s Snapshot) {
		for _, r := range text {
			s = h.Key(string(r))
		}
		return s
	}
	h, _ := headless(t)
	keys(h, "a")
	s := typeIt(h, long+" due:")
	if s.Input == nil || s.Input.Alarm == nil || s.Input.Alarm.Kind != "due" {
		t.Fatalf("add: due: at the end of a long task didn't open the calendar: %+v", s.Input)
	}
	s = keys(h, "enter", "enter") // set the date, then save
	if s.Mode != "normal" {
		t.Fatalf("mode %s after saving", s.Mode)
	}

	// Edit the long task: the cursor starts at the end.
	h, _ = headless(t)
	keys(h, "a")
	typeIt(h, long)
	keys(h, "enter", "e")
	s = typeIt(h, " due:")
	if s.Input == nil || s.Input.Alarm == nil || s.Input.Alarm.Kind != "due" {
		t.Fatalf("edit: due: at the end of a long task didn't open the calendar: %+v", s.Input)
	}
}

// A long line scrolls inside the terminal's add popup instead of wrapping
// onto more lines, while typing and when moving back to the start.
func TestPopupInputFitsLongText(t *testing.T) {
	m := scriptModel(t, nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = mm.(Model)
	m.startAdd()
	want := strings.Count(m.viewTaskPopup(), "\n")
	m = typeText(m, strings.Repeat("a long task about many things ", 6))
	if got := strings.Count(m.viewTaskPopup(), "\n"); got != want {
		t.Fatalf("typing: popup is %d lines, want %d:\n%s", got+1, want+1, m.viewTaskPopup())
	}
	m = press(m, tea.KeyHome)
	if got := strings.Count(m.viewTaskPopup(), "\n"); got != want {
		t.Fatalf("home: popup is %d lines, want %d:\n%s", got+1, want+1, m.viewTaskPopup())
	}
}

// A terminal can send fast typing as one message; due: and alarm: still
// open their pickers.
func TestPickerOpensOnBatchedTyping(t *testing.T) {
	for _, tc := range []struct{ typed, batch string }{
		{"some task ", "due:"}, {"some task du", "e:"}, {"some task ", "alarm:"},
	} {
		m := scriptModel(t, nil)
		m.startAdd()
		m = typeText(m, tc.typed)
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.batch)})
		m = mm.(Model)
		if wantDue := tc.batch != "alarm:"; m.pick == nil || m.pick.due != wantDue {
			t.Errorf("%q then %q as one message: value %q, no %s picker", tc.typed, tc.batch, m.input.Value(), map[bool]string{true: "due", false: "alarm"}[wantDue])
		}
	}
}

// katTasks-gui sees a /script's kat.choose list, and a click picks from it.
func TestHeadlessChooseList(t *testing.T) {
	src := `local p = kat.choose("Priority", {"A", "B", "C"})
if not p then return nil end
kat.replace("(" .. p .. ") " .. kat.input)`
	h := NewHeadless(scriptModel(t, map[string]string{"pri": src}), HeadlessOptions{})
	keys(h, "a")
	for _, r := range "Call mom /pri" {
		h.Key(string(r))
	}
	s := h.Key("tab")
	c := s.Input.Choice
	if c == nil || c.Prompt != "Priority" || strings.Join(c.Options, "") != "ABC" || c.Idx != 0 {
		t.Fatalf("choice list in the snapshot: %+v", c)
	}
	if s = h.Key("down"); s.Input.Choice.Idx != 1 {
		t.Fatalf("down: idx %d", s.Input.Choice.Idx)
	}
	s = h.Pick(2, true) // click C
	if s.Input.Choice != nil || !strings.HasPrefix(s.Input.Value, "(C) Call mom") {
		t.Fatalf("after clicking C: %q, list %+v", s.Input.Value, s.Input.Choice)
	}
}

// The helpers below reach into the model under h's lock, since commands
// such as chord and picker timers update it in the background.

// with runs fn on the model.
func (h *Headless) with(fn func(m *Model)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn(&h.m)
}

// model returns a copy of the model, for reading its fields.
func (h *Headless) model() Model {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.m
}

// view renders the terminal screen.
func (h *Headless) view() (v string) {
	h.with(func(m *Model) { v = m.View() })
	return v
}
