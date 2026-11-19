package todo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A task can carry a time of day for the agenda: an at: tag (at:1430,
// at:9am, at:315pm; at:14:30 as written before) or a time written in its
// text ("Standup 9:30", "Call
// mom 3pm"). due: stays a plain date, so the file stays todo.txt.

var clockRe = regexp.MustCompile(`^(\d{1,2})(?:([:.]?)(\d{2}))?(am|pm|a|p)?$`)

// ParseClock reads a time of day as minutes after midnight: "14:30",
// "9.30", "3pm", "3:15pm", and in tags also "1430", "930", "315pm" and a
// bare hour ("9"), as bare is set for tag values. In text a time needs a
// colon, a dot or am/pm, so ordinary numbers aren't taken for times.
func ParseClock(s string, bare bool) (int, bool) {
	m := clockRe.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return 0, false
	}
	hour, sep, mins, ampm := m[1], m[2], m[3], m[4]
	if !bare && (mins == "" || sep == "") && ampm == "" {
		return 0, false
	}
	h, _ := strconv.Atoi(hour)
	min := 0
	if mins != "" {
		min, _ = strconv.Atoi(mins)
	}
	if min > 59 {
		return 0, false
	}
	switch ampm {
	case "":
		if h > 23 {
			return 0, false
		}
	default:
		if h < 1 || h > 12 {
			return 0, false
		}
		h %= 12
		if ampm[0] == 'p' {
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

// ClockOf shows t's time of day as FormatClock does.
func ClockOf(t time.Time) string { return FormatClock(t.Hour()*60 + t.Minute()) }

// FormatClockTag writes minutes after midnight for an at: tag, without the
// colon todo.txt doesn't allow in a tag's value: "905", "1430", and "0030"
// in the first hour.
func FormatClockTag(min int) string {
	if min < 60 {
		return fmt.Sprintf("00%02d", min)
	}
	return fmt.Sprintf("%d%02d", min/60, min%60)
}

// clockTag rewrites the time v (which reads as min) without its colon,
// keeping 12-hour times 12-hour: "14:30" → "1430", "3:15pm" → "315pm".
func clockTag(v string, min int) string {
	if s := strings.ReplaceAll(v, ":", ""); s != v {
		if n, ok := ParseClock(s, true); ok && n == min {
			return s
		}
	}
	return FormatClockTag(min)
}
