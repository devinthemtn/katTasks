package todo

import (
	"testing"
	"time"
)

func TestParseAlarm(t *testing.T) {
	want := time.Date(2026, 12, 1, 7, 30, 0, 0, time.Local)
	for _, v := range []string{"2026-12-01T07:30", "2026-12-01T7:30"} {
		if a, ok := ParseAlarm(v); !ok || !a.Equal(want) {
			t.Errorf("%q: %v %v", v, a, ok)
		}
	}
	for _, v := range []string{"7:30", "2026-12-01", "2026-12-01T25:00", "tomorrow"} {
		if _, ok := ParseAlarm(v); ok {
			t.Errorf("%q parsed", v)
		}
	}
	if FormatAlarm(want) != "2026-12-01T07:30" {
		t.Error(FormatAlarm(want))
	}
}

func TestNextMovesAlarm(t *testing.T) {
	now := time.Date(2026, 12, 3, 10, 0, 0, 0, time.Local)
	cases := map[string]string{
		// With due: it keeps its distance from the due date (a day before).
		"Rent due:2026-12-01 rec:+1m alarm:2026-11-30T09:00": "2026-12-31T09:00",
		// Without due: strict moves from the alarm, plain from today.
		"Stretch rec:+1w alarm:2026-12-01T07:00":     "2026-12-08T07:00",
		"Water plants rec:3d alarm:2026-12-01T18:15": "2026-12-06T18:15",
	}
	for line, want := range cases {
		n, ok := Parse(line).Next(now)
		if !ok {
			t.Fatalf("%q: no next", line)
		}
		if got, _ := n.Tag("alarm"); got != want {
			t.Errorf("%q: next alarm %s, want %s", line, got, want)
		}
	}
}
