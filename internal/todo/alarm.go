package todo

import (
	"math"
	"strings"
	"time"
)

// AlarmLayout is an alarm: tag's value: alarm:2026-12-01T07:30. The tag
// rings an alarm at that date and time; due: stays a plain date.
const AlarmLayout = "2006-01-02T15:04"

// ParseAlarm reads an alarm: value. The hour may be one digit
// (2026-12-01T7:30).
func ParseAlarm(v string) (time.Time, bool) {
	if d, rest, ok := strings.Cut(v, "T"); ok && len(rest) == 4 {
		v = d + "T0" + rest
	}
	a, err := time.ParseInLocation(AlarmLayout, v, time.Local)
	return a, err == nil
}

// FormatAlarm writes an alarm: value.
func FormatAlarm(t time.Time) string { return t.Format(AlarmLayout) }

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
	day := truncateDay(a)
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
