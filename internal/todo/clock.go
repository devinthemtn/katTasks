package todo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A task can carry a time of day for the agenda: an at: tag (at:14:30,
// at:9am, at:3:15pm) or a time written in its text ("Standup 9:30", "Call
// mom 3pm"). due: stays a plain date, so the file stays todo.txt.

var clockRe = regexp.MustCompile(`^(\d{1,2})(?:[:.](\d{2}))?(am|pm|a|p)?$`)

// ParseClock reads a time of day as minutes after midnight. A bare hour
// ("9") counts only when bare is set, as it is for at: tags; in text a time
// needs minutes or am/pm, so ordinary numbers aren't taken for times.
func ParseClock(s string, bare bool) (int, bool) {
	m := clockRe.FindStringSubmatch(strings.ToLower(s))
	if m == nil || (!bare && m[2] == "" && m[3] == "") {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	min := 0
	if m[2] != "" {
		min, _ = strconv.Atoi(m[2])
	}
	if min > 59 {
		return 0, false
	}
	switch m[3] {
	case "":
		if h > 23 {
			return 0, false
		}
	default:
		if h < 1 || h > 12 {
			return 0, false
		}
		h %= 12
		if m[3][0] == 'p' {
			h += 12
		}
	}
	return h*60 + min, true
}

// Clock returns the task's time of day in minutes after midnight: its at:
// tag, else the first time in its text.
func (t *Task) Clock() (int, bool) {
	if v, ok := t.Tag("at"); ok {
		return ParseClock(v, true)
	}
	for _, w := range strings.Fields(t.Text) {
		if n, ok := ParseClock(strings.TrimRight(w, ",;)"), false); ok {
			return n, true
		}
	}
	return 0, false
}

// FormatClock shows minutes after midnight as "9:05" or "14:30".
func FormatClock(min int) string { return fmt.Sprintf("%d:%02d", min/60, min%60) }
