package listview

import (
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

func TestLinkedNote(t *testing.T) {
	cases := map[string]struct {
		note string
		ok   bool
	}{
		"call kn:abc123 note:page/house": {"page/house", true},
		"call kn:abc123":                 {"", true},
		"call kn:xyz note:page/house":    {"", false}, // not an ID katnotes writes
		"call note:page/house":           {"", false},
	}
	for line, want := range cases {
		note, ok := LinkedNote(todo.Parse(line))
		if note != want.note || ok != want.ok {
			t.Errorf("%q: %q %v", line, note, ok)
		}
	}
	if !IsLinkTag("kn:abc123") || !IsLinkTag("note:page/x") || IsLinkTag("due:2026-01-01") || IsLinkTag("kn:") {
		t.Error("IsLinkTag")
	}
}

func TestSidebarLinked(t *testing.T) {
	now := time.Date(2026, 11, 23, 9, 0, 0, 0, time.Local)
	tasks := []*todo.Task{todo.Parse("a kn:abc123 note:page/x"), todo.Parse("x 2026-11-01 b kn:def456"), todo.Parse("c")}
	side, _ := Sidebar(tasks, nil, now)
	i := linkedAt(side)
	if i < 0 || side[i].Count != 1 || side[i].Label() != "Linked to notes" {
		t.Fatalf("side = %+v", side)
	}
	if got := Visible(tasks, side[i], query.Parse(""), false, "file", now); len(got) != 2 {
		t.Errorf("visible = %v", got)
	}
	if side, _ := Sidebar(tasks[2:], nil, now); linkedAt(side) >= 0 {
		t.Error("entry shown with nothing linked")
	}
}

func linkedAt(side []Entry) int {
	for i, e := range side {
		if e.Kind == Linked {
			return i
		}
	}
	return -1
}
