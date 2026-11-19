package listview

import (
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

var now = time.Date(2026, 11, 18, 10, 0, 0, 0, time.Local)

func TestPrioritySortPutsOverdueFirst(t *testing.T) {
	hi := todo.Parse("(A) important due:2026-11-21")
	late := todo.Parse("(C) late due:2026-11-16")
	none := todo.Parse("(B) no date")
	if Compare("priority", late, hi, now) >= 0 {
		t.Error("overdue (C) should sort before (A) due later")
	}
	if Compare("priority", late, none, now) >= 0 {
		t.Error("overdue (C) should sort before undated (B)")
	}
	if Compare("priority", hi, none, now) >= 0 {
		t.Error("(A) should still sort before (B) when neither is overdue")
	}
	if Compare("priority", todo.Parse("today due:2026-11-18"), hi, now) <= 0 {
		t.Error("a task due today isn't overdue")
	}
}

func TestSidebarAndVisible(t *testing.T) {
	tasks := []*todo.Task{
		todo.Parse("(B) one +Work @phone"),
		todo.Parse("x done +Work"),
		todo.Parse("(A) two +Home due:2026-11-20"),
	}
	side, known := Sidebar(tasks, map[string]string{"urgent": "pri:A"}, now)
	var got []string
	for _, e := range side {
		got = append(got, e.Kind.String()+":"+e.Label())
	}
	want := []string{"all:All tasks", "due:Has due date", "header:SAVED", "saved:urgent",
		"header:PROJECTS", "project:+Home", "project:+Work", "header:CONTEXTS", "context:@phone"}
	if len(got) != len(want) {
		t.Fatalf("sidebar = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sidebar = %v, want %v", got, want)
		}
	}
	if side[0].Count != 2 || side[6].Count != 1 {
		t.Errorf("counts: all %d, +Work %d", side[0].Count, side[6].Count)
	}
	if known[0] != "+Work" {
		t.Errorf("known = %v, most used first", known)
	}
	if i := Find(side, Project, "+Work"); i != 6 {
		t.Errorf("Find = %d", i)
	}

	vis := Visible(tasks, side[6], query.Query{}, false, "priority", now)
	if len(vis) != 2 || vis[0] != tasks[0] {
		t.Errorf("visible +Work = %v", vis)
	}
	if q := CurrentQuery(side[6], "milk", nil); q != "+Work milk" {
		t.Errorf("CurrentQuery = %q", q)
	}
}

func TestDueLabel(t *testing.T) {
	for _, c := range []struct {
		date, label string
		kind        DueKind
	}{
		{"2026-11-15", "overdue 3d", DueOverdue},
		{"2026-11-18", "today", DueToday},
		{"2026-11-19", "tomorrow", DueSoon},
		{"2026-11-21", "Sat", DueSoon},
		{"2026-12-24", "Dec 24", DueLater},
		{"2027-01-02", "2027-01-02", DueLater},
	} {
		d, _ := time.ParseInLocation(todo.DateLayout, c.date, time.Local)
		if l, k := DueLabel(d, now); l != c.label || k != c.kind {
			t.Errorf("DueLabel(%s) = %q %s, want %q %s", c.date, l, k, c.label, c.kind)
		}
	}
}

func TestIsTagSkipsTimes(t *testing.T) {
	for w, want := range map[string]bool{"due:2026-11-18": true, "at:9:30": true, "14:30": false, "9:05pm": false, "http://x": false} {
		if got := IsTag(w); got != want {
			t.Errorf("IsTag(%q) = %v", w, got)
		}
	}
}
