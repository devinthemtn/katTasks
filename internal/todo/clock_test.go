package todo

import "testing"

func TestParseClock(t *testing.T) {
	for _, c := range []struct {
		in   string
		bare bool
		want int
		ok   bool
	}{
		{"14:30", false, 14*60 + 30, true},
		{"9:05", false, 9*60 + 5, true},
		{"9.30", false, 9*60 + 30, true},
		{"3pm", false, 15 * 60, true},
		{"3:15PM", false, 15*60 + 15, true},
		{"12am", false, 0, true},
		{"12pm", false, 12 * 60, true},
		{"9a", false, 9 * 60, true},
		{"9", true, 9 * 60, true},
		{"9", false, 0, false},
		{"24:00", false, 0, false},
		{"13pm", false, 0, false},
		{"9:75", false, 0, false},
		{"2026", true, 0, false},
		{"ten", true, 0, false},
	} {
		got, ok := ParseClock(c.in, c.bare)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseClock(%q, %v) = %d, %v; want %d, %v", c.in, c.bare, got, ok, c.want, c.ok)
		}
	}
}

func TestTaskClock(t *testing.T) {
	for in, want := range map[string]int{
		"Standup 9:30 @work":            9*60 + 30,
		"Call mom (3pm) due:2026-11-20": -1, // "(3pm)" isn't a time word
		"Call mom 3pm, then dinner":     15 * 60,
		"Dentist at:14:15 9:00":         14*60 + 15, // at: wins
		"Read chapter 4":                -1,
		"Meet at:9":                     9 * 60,
	} {
		got, ok := Parse(in).Clock()
		if want < 0 {
			if ok {
				t.Errorf("%q: got time %d", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("%q: Clock() = %d, %v; want %d", in, got, ok, want)
		}
	}
	if s := FormatClock(9*60 + 5); s != "9:05" {
		t.Errorf("FormatClock = %q", s)
	}
}
