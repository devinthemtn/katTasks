package todo

import (
	"math"
	"strings"
	"time"
)

// An alarm: tag rings an alarm at a date and time: alarm:2026-12-01T0730,
// without a colon in the time as todo.txt asks of tag values. due: stays a
// plain date.

// ParseAlarm reads an alarm: value: a date, T, and a time as an at: tag
// takes it (0730, 730, 7pm; and 07:30 as written before).
func ParseAlarm(v string) (time.Time, bool) {
	date, clock, ok := strings.Cut(v, "T")
	if !ok {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(DateLayout, date, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	min, ok := ParseClock(clock, true)
	if !ok {
		return time.Time{}, false
	}
	return d.Add(time.Duration(min) * time.Minute), true
}

// FormatAlarm writes an alarm: value: 2026-12-01T0730.
func FormatAlarm(t time.Time) string { return t.Format("2006-01-02T1504") }

// IsAlarmTag reports whether word is an alarm: tag that Alarm reads, so a
// row can show it as a badge instead.
func IsAlarmTag(word string) bool {
	k, v, ok := splitTag(word)
	if !ok || k != "alarm" {
		return false
	}
	_, ok = ParseAlarm(v)
	return ok
}

// Alarm returns the task's alarm: date and time, if it has a valid one.
func (t *Task) Alarm() (time.Time, bool) {
	v, ok := t.Tag("alarm")
	if !ok {
		return time.Time{}, false
	}
	return ParseAlarm(v)
}

// nextAlarm moves the next occurrence's alarm along with it: the same
// number of days from the new due date as it was from the old one, at the
// same time of day. Without a due date it moves by the recurrence, from the
// alarm's own day when strict and from today otherwise.
func (t *Task) nextAlarm(n *Task, r Recurrence, today time.Time) {
	a, ok := t.Alarm()
	if !ok {
		return
	}
	day := Midnight(a)
	if due, hasDue := t.Due(); hasDue {
		if nd, ok := n.Due(); ok {
			gap := int(math.Round(day.Sub(due).Hours() / 24))
			day = nd.AddDate(0, 0, gap)
		}
	} else if r.Strict {
		day = r.Add(day)
	} else {
		day = r.Add(today)
	}
	na := time.Date(day.Year(), day.Month(), day.Day(), a.Hour(), a.Minute(), 0, 0, time.Local)
	n.SetTag("alarm", FormatAlarm(na))
}
