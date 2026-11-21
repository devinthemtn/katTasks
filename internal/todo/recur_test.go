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

// AddBusinessDays must agree with counting weekdays one at a time, from
// every day of the week, forwards and backwards.
func TestAddBusinessDays(t *testing.T) {
	slow := func(d time.Time, n int) time.Time {
		step := 1
		if n < 0 {
			step, n = -1, -n
		}
		for n > 0 {
			d = d.AddDate(0, 0, step)
			if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
				n--
			}
		}
		return d
	}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local) // a Sunday
	for day := 0; day < 7; day++ {
		d := start.AddDate(0, 0, day)
		for n := -40; n <= 40; n++ {
			if got, want := AddBusinessDays(d, n), slow(d, n); !got.Equal(want) {
				t.Errorf("%s %+db = %s, want %s", d.Format("Mon 2006-01-02"), n, got.Format("Mon 2006-01-02"), want.Format("Mon 2006-01-02"))
			}
		}
	}
}

// A huge rec:Nb must not hang completing the task.
func TestHugeBusinessRecurrence(t *testing.T) {
	r, ok := ParseRecurrence("999999999b")
	if !ok {
		t.Fatal("not parsed")
	}
	done := make(chan time.Time, 1)
	go func() { done <- r.Add(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rec:999999999b took over 2s")
	}
}
