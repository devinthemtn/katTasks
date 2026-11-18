package todo

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	d, _ := time.ParseInLocation(DateLayout, s, time.Local)
	return d
}

func TestParseRecurrence(t *testing.T) {
	good := map[string]Recurrence{
		"1d": {1, 'd', false}, "+2w": {2, 'w', true}, "m": {1, 'm', false},
		"3b": {3, 'b', false}, "+1y": {1, 'y', true},
	}
	for in, want := range good {
		if got, ok := ParseRecurrence(in); !ok || got != want {
			t.Errorf("%q -> %+v %v, want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "+", "0d", "1x", "-1d", "abc"} {
		if _, ok := ParseRecurrence(in); ok {
			t.Errorf("%q should be invalid", in)
		}
	}
}

func TestRecurrenceAdd(t *testing.T) {
	cases := []struct{ from, rec, want string }{
		{"2026-01-31", "1m", "2026-02-28"},
		{"2028-01-31", "1m", "2028-02-29"},
		{"2026-11-20", "1b", "2026-11-23"}, // Fri -> Mon
		{"2026-11-18", "5b", "2026-11-25"},
		{"2026-11-18", "2w", "2026-12-02"},
		{"2024-02-29", "1y", "2025-02-28"},
	}
	for _, c := range cases {
		r, _ := ParseRecurrence(c.rec)
		if got := r.Add(day(c.from)).Format(DateLayout); got != c.want {
			t.Errorf("%s + %s = %s, want %s", c.from, c.rec, got, c.want)
		}
	}
}

func TestNext(t *testing.T) {
	now := time.Date(2026, 11, 18, 14, 0, 0, 0, time.Local)
	cases := []struct{ in, want string }{
		// normal: from completion date
		{"(A) 2026-11-01 Water plants due:2026-11-10 rec:1w", "(A) 2026-11-18 Water plants due:2026-11-25 rec:1w"},
		// strict: from old due date
		{"Pay rent due:2026-11-01 rec:+1m", "Pay rent due:2026-12-01 rec:+1m"},
		// no due date: one is added
		{"Stretch rec:1d", "Stretch rec:1d due:2026-11-19"},
		// threshold keeps its distance from due
		{"Report t:2026-11-13 due:2026-11-15 rec:+1w", "Report t:2026-11-20 due:2026-11-22 rec:+1w"},
		// threshold only
		{"Review t:2026-11-10 rec:2d", "Review t:2026-11-20 rec:2d"},
	}
	for _, c := range cases {
		n, ok := Parse(c.in).Next(now)
		if !ok || n.String() != c.want {
			t.Errorf("next of %q:\n got  %v\n want %q", c.in, n, c.want)
		}
	}
	if _, ok := Parse("No rec here").Next(now); ok {
		t.Error("task without rec: recurred")
	}
}
