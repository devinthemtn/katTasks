package listview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

func TestEntryMatch(t *testing.T) {
	dated := todo.Parse("(A) call +Work @phone due:2026-11-20")
	plain := todo.Parse("read +Home")
	for _, c := range []struct {
		e            Entry
		dated, plain bool
	}{
		{Entry{Kind: All}, true, true},
		{Entry{Kind: Due}, true, false},
		{Entry{Kind: Project, Value: "+Home"}, false, true},
		{Entry{Kind: Context, Value: "@phone"}, true, false},
		{Entry{Kind: Saved, Value: "a", Q: query.Parse("pri:A")}, true, false},
	} {
		if got := c.e.Match(dated, now); got != c.dated {
			t.Errorf("%s %q matches the dated task: %v", c.e.Kind, c.e.Value, got)
		}
		if got := c.e.Match(plain, now); got != c.plain {
			t.Errorf("%s %q matches the plain task: %v", c.e.Kind, c.e.Value, got)
		}
	}
}

func TestFindMissingIsAll(t *testing.T) {
	side := []Entry{{Kind: All}, {Kind: Project, Value: "+Work"}}
	if i := Find(side, Project, "+Gone"); i != 0 {
		t.Errorf("Find of a missing entry = %d, want 0 (All)", i)
	}
	if i := Find(side, Context, "+Work"); i != 0 {
		t.Errorf("Find matches the value only with its kind, got %d", i)
	}
}

func TestCurrentQuery(t *testing.T) {
	filters := map[string]string{"urgent": "pri:A"}
	for _, c := range []struct {
		e      Entry
		filter string
		want   string
	}{
		{Entry{Kind: All}, "  milk ", "milk"},
		{Entry{Kind: Context, Value: "@phone"}, "", "@phone"},
		{Entry{Kind: Due}, "+Work", "-due:none +Work"},
		{Entry{Kind: Due}, "", "-due:none"},
		{Entry{Kind: Saved, Value: "urgent"}, "@home", "pri:A @home"},
	} {
		if got := CurrentQuery(c.e, c.filter, filters); got != c.want {
			t.Errorf("CurrentQuery(%s %q, %q) = %q, want %q", c.e.Kind, c.e.Value, c.filter, got, c.want)
		}
	}
}

func TestVisibleHidesDone(t *testing.T) {
	tasks := []*todo.Task{todo.Parse("x 2026-11-17 done"), todo.Parse("open")}
	if vis := Visible(tasks, Entry{Kind: All}, query.Query{}, true, "file", now); len(vis) != 1 || vis[0] != tasks[1] {
		t.Errorf("hideDone: %v", vis)
	}
	if vis := Visible(tasks, Entry{Kind: All}, query.Query{}, false, "file", now); len(vis) != 2 || vis[0] != tasks[0] {
		t.Errorf("file order keeps done tasks where they are: %v", vis)
	}
	if vis := Visible(tasks, Entry{Kind: All}, query.Parse("open"), false, "file", now); len(vis) != 1 {
		t.Errorf("search: %v", vis)
	}
}

func TestSortModes(t *testing.T) {
	a := todo.Parse("(B) 2026-11-01 old due:2026-11-25")
	b := todo.Parse("(A) 2026-11-10 new")
	c := todo.Parse("no dates")
	d := todo.Parse("x 2026-11-17 done due:2026-11-19")
	e := todo.Parse("(B) 2026-11-01 tie with old due:2026-11-25")
	all := []*todo.Task{c, d, a, b, e}
	texts := func(ts []*todo.Task) []string {
		var out []string
		for _, t := range ts {
			out = append(out, t.Text)
		}
		return out
	}
	for mode, want := range map[string][]string{
		"file":     {"no dates", "done due:2026-11-19", "old due:2026-11-25", "new", "tie with old due:2026-11-25"},
		"priority": {"new", "old due:2026-11-25", "tie with old due:2026-11-25", "no dates", "done due:2026-11-19"},
		"due":      {"old due:2026-11-25", "tie with old due:2026-11-25", "new", "no dates", "done due:2026-11-19"},
		"created":  {"new", "old due:2026-11-25", "tie with old due:2026-11-25", "no dates", "done due:2026-11-19"},
	} {
		vis := append([]*todo.Task(nil), all...)
		Sort(mode, vis, all, now)
		got := texts(vis)
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: %q\nwant %q", mode, got, want)
				break
			}
		}
	}
	if Compare("nonsense", a, b, now) != 0 {
		t.Error("an unknown mode should tie, leaving file order")
	}
}

func TestMoveScope(t *testing.T) {
	for mode, want := range map[string]string{"priority": "priority and due date", "due": "due date and priority", "created": "creation date", "file": ""} {
		if got := MoveScope(mode); got != want {
			t.Errorf("MoveScope(%q) = %q, want %q", mode, got, want)
		}
	}
}

func TestBody(t *testing.T) {
	task := todo.Parse("(A) 2026-11-01 Call Mom +Family @phone at 14:30 due:2026-11-20 see https://x.org")
	if got, want := Body(task), "Call Mom at 14:30 see https://x.org"; got != want {
		t.Errorf("Body = %q, want %q", got, want)
	}
	if got := Body(todo.Parse("+Only @tags k:v")); got != "" {
		t.Errorf("Body of only tags = %q", got)
	}
	if got := Body(todo.Parse("a + b @ c")); got != "a + b @ c" {
		t.Errorf("a lone + or @ is a word, got %q", got)
	}
}

func TestWorkLabels(t *testing.T) {
	for _, c := range []struct {
		line, status string
		doing        bool
		progress     string
	}{
		{"write status:doing progress:40", "▶ doing", true, "▰▰▱▱▱ 40%"},
		{"write status:blocked", "blocked", false, ""},
		{"write progress:0", "", false, "▱▱▱▱▱ 0%"},
		{"write progress:99", "", false, "▰▰▰▰▱ 99%"},
		{"write progress:100", "", false, "▰▰▰▰▰ 100%"},
		{"write", "", false, ""},
		{"x 2026-11-17 write status:doing progress:40", "", false, ""},
	} {
		s, d, p := WorkLabels(todo.Parse(c.line))
		if s != c.status || d != c.doing || p != c.progress {
			t.Errorf("WorkLabels(%q) = %q %v %q, want %q %v %q", c.line, s, d, p, c.status, c.doing, c.progress)
		}
	}
}

func TestAlarmLabel(t *testing.T) { // now is Wednesday 2026-11-18 10:00
	for _, c := range []struct {
		line, label string
		past        bool
	}{
		{"wake alarm:2026-11-18T0730", "⏰ 7:30", true},
		{"wake alarm:2026-11-18T2200", "⏰ 22:00", false},
		{"wake alarm:2026-11-19T0730", "⏰ tomorrow 7:30", false},
		{"wake alarm:2026-11-21T0730", "⏰ Sat 7:30", false},
		{"wake alarm:2026-11-17T0730", "⏰ Nov 17 7:30", true},
		{"wake alarm:2026-12-01T0730", "⏰ Dec 1 7:30", false},
		{"wake alarm:2027-01-02T0730", "⏰ 2027-01-02 7:30", false},
		{"no alarm", "", false},
		{"x 2026-11-17 wake alarm:2026-11-19T0730", "", false},
	} {
		if l, p := AlarmLabel(todo.Parse(c.line), now); l != c.label || p != c.past {
			t.Errorf("AlarmLabel(%q) = %q %v, want %q %v", c.line, l, p, c.label, c.past)
		}
	}
}

func TestBraced(t *testing.T) {
	for _, c := range []struct {
		text string
		want []bool
	}{
		{"call {{Ann}} today", []bool{false, true, false}},
		{"ask {{Ann and Bo}} +Work", []bool{false, true, true, true, false}},
		{"x{{y}}z and {{w}}", []bool{true, false, true}},
		{"{{a}} {{b", []bool{true, false}},
		{"open {{ never closed", []bool{false, false, false, false}},
		{"{{}} }} {{a}}{{b}}", []bool{true, false, true}},
		{"no braces", []bool{false, false}},
	} {
		got := Braced(strings.Fields(c.text))
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("Braced(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}
