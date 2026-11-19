package todo

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	lines := []string{
		"(A) Call Mom +Family @phone",
		"(B) 2026-01-02 Write report +Work due:2026-02-01",
		"x 2026-01-05 2026-01-02 Pay rent",
		"x 2026-01-05 Done without creation date",
		"Plain task with url http://example.com",
		"2026-03-04 created only",
		"(a) lowercase is not a priority",
	}
	for _, l := range lines {
		if got := Parse(l).String(); got != l {
			t.Errorf("round trip:\n in  %q\n out %q", l, got)
		}
	}
}

func TestParseFields(t *testing.T) {
	tk := Parse("(B) 2026-01-02 Write report +Work @desk due:2026-02-01 http://x.y")
	if tk.Priority != 'B' || tk.Created.Format(DateLayout) != "2026-01-02" {
		t.Fatalf("bad header: %+v", tk)
	}
	if p := tk.Projects(); len(p) != 1 || p[0] != "+Work" {
		t.Errorf("projects = %v", p)
	}
	if c := tk.Contexts(); len(c) != 1 || c[0] != "@desk" {
		t.Errorf("contexts = %v", c)
	}
	if d, ok := tk.Due(); !ok || d.Format(DateLayout) != "2026-02-01" {
		t.Errorf("due = %v %v", d, ok)
	}
	if _, ok := tk.Tag("http"); ok {
		t.Errorf("url parsed as tag")
	}
}

func TestCompleteRestoresPriority(t *testing.T) {
	tk := Parse("(A) 2026-01-02 thing")
	now := time.Date(2026, 1, 10, 15, 0, 0, 0, time.Local)
	tk.Complete(now)
	if got := tk.String(); got != "x 2026-01-10 2026-01-02 thing pri:A" {
		t.Fatalf("complete: %q", got)
	}
	tk.Uncomplete()
	if got := tk.String(); got != "(A) 2026-01-02 thing" {
		t.Fatalf("uncomplete: %q", got)
	}
}

func TestNormalizeDue(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local) // Wednesday
	cases := map[string]string{
		"today": "2026-11-18", "tomorrow": "2026-11-19", "fri": "2026-11-20",
		"wed": "2026-11-25", "+3d": "2026-11-21", "+2w": "2026-12-02",
	}
	for in, want := range cases {
		tk := Parse("task due:" + in)
		tk.Normalize(now)
		if got, _ := tk.Tag("due"); got != want {
			t.Errorf("due:%s -> %s, want %s", in, got, want)
		}
	}
}

func TestSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "todo.txt")
	in := []*Task{Parse("(A) one"), Parse("x 2026-01-01 two")}
	if _, err := Save(p, in); err != nil {
		t.Fatal(err)
	}
	out, _, err := Load(p)
	if err != nil || len(out) != 2 || out[1].String() != "x 2026-01-01 two" {
		t.Fatalf("load: %v %v", out, err)
	}
}
