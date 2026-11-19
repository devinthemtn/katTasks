package todo

import (
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	for in, want := range map[string]int{"progress:40": 40, "progress:0": 0, "progress:100": 100, "progress:75%": 75} {
		tk := Parse("write it " + in)
		if got, ok := tk.Progress(); !ok || got != want {
			t.Errorf("%q: got %d %v, want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"progress:101", "progress:-5", "progress:half", "nothing"} {
		if _, ok := Parse("write it " + in).Progress(); ok {
			t.Errorf("%q should have no progress", in)
		}
	}
}

func TestSetProgress(t *testing.T) {
	tk := Parse("write it +Work")
	tk.SetProgress(30)
	if tk.Text != "write it +Work progress:30" {
		t.Errorf("got %q", tk.Text)
	}
	tk.SetProgress(250)
	if got, _ := tk.Progress(); got != 100 {
		t.Errorf("capped to %d", got)
	}
	tk.SetProgress(0)
	if tk.Text != "write it +Work" {
		t.Errorf("not removed: %q", tk.Text)
	}
}

func TestStatus(t *testing.T) {
	if s, ok := Parse("a status:Doing").Status(); !ok || s != "doing" {
		t.Errorf("got %q %v", s, ok)
	}
	if _, ok := Parse("a").Status(); ok {
		t.Error("no status expected")
	}
}

func TestIsWorkTag(t *testing.T) {
	for w, want := range map[string]bool{"status:doing": true, "progress:40": true, "progress:x": false, "due:2026-01-01": false, "status": false} {
		if got := IsWorkTag(w); got != want {
			t.Errorf("%q: %v", w, got)
		}
	}
}

func TestNextDropsWork(t *testing.T) {
	tk := Parse("water plants rec:1w status:doing progress:50")
	n, ok := tk.Next(time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local))
	if !ok {
		t.Fatal("no next")
	}
	if _, ok := n.Status(); ok {
		t.Errorf("status kept: %q", n.Text)
	}
	if _, ok := n.Progress(); ok {
		t.Errorf("progress kept: %q", n.Text)
	}
}
