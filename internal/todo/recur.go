package todo

import (
	"math"
	"strconv"
	"time"
)

// Recurrence is a parsed rec: tag, e.g. rec:1w or rec:+2m.
//
// Units: d (days), b (business days, Mon–Fri), w (weeks), m (months),
// y (years). A leading "+" makes the recurrence strict: the next date is
// computed from the task's current due date instead of the completion date.
type Recurrence struct {
	N      int
	Unit   byte
	Strict bool
}

// ParseRecurrence parses the value of a rec: tag.
func ParseRecurrence(v string) (Recurrence, bool) {
	var r Recurrence
	if len(v) > 0 && v[0] == '+' {
		r.Strict, v = true, v[1:]
	}
	if len(v) < 1 {
		return r, false
	}
	r.Unit = v[len(v)-1]
	switch r.Unit {
	case 'd', 'b', 'w', 'm', 'y':
	default:
		return r, false
	}
	r.N = 1
	if num := v[:len(v)-1]; num != "" {
		n, err := strconv.Atoi(num)
		if err != nil || n < 1 {
			return r, false
		}
		r.N = n
	}
	return r, true
}

// Add returns d advanced by the recurrence interval.
func (r Recurrence) Add(d time.Time) time.Time {
	switch r.Unit {
	case 'd':
		return d.AddDate(0, 0, r.N)
	case 'w':
		return d.AddDate(0, 0, 7*r.N)
	case 'm':
		return addMonths(d, r.N)
	case 'y':
		return addMonths(d, 12*r.N)
	case 'b':
		for n := r.N; n > 0; {
			d = d.AddDate(0, 0, 1)
			if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
				n--
			}
		}
	}
	return d
}

// addMonths adds months, clamping to the end of the month so that
// Jan 31 + 1m is Feb 28/29 rather than early March.
func addMonths(d time.Time, n int) time.Time {
	y, m, day := d.Date()
	first := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, d.Location())
	last := first.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, d.Location())
}

// Recurrence returns the task's rec: tag, if it has a valid one.
func (t *Task) Recurrence() (Recurrence, bool) {
	v, ok := t.Tag("rec")
	if !ok {
		return Recurrence{}, false
	}
	return ParseRecurrence(v)
}

// threshold returns the t:YYYY-MM-DD date, if any.
func (t *Task) threshold() (time.Time, bool) {
	v, ok := t.Tag("t")
	if !ok {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(DateLayout, v, time.Local)
	return d, err == nil
}

// Next builds the follow-up task for a recurring task completed on now.
// It should be called on the task before it is completed. It returns false
// if the task has no valid rec: tag.
//
//   - With a due date, the new due date is the old one + interval (strict)
//     or the completion date + interval (normal).
//   - Without a due date, a due date of completion + interval is added.
//   - A t: threshold date keeps the same distance from the due date; if there
//     is no due date it is advanced the same way a due date would be.
//   - The new task gets today as its creation date if the original had one.
func (t *Task) Next(now time.Time) (*Task, bool) {
	r, ok := t.Recurrence()
	if !ok {
		return nil, false
	}
	today := Midnight(now)
	n := t.Clone()
	n.Done, n.Completed = false, time.Time{}
	n.RemoveTag("status") // a fresh occurrence isn't started
	n.RemoveTag("progress")
	if !t.Created.IsZero() {
		n.Created = today
	}

	due, hasDue := t.Due()
	thr, hasThr := t.threshold()

	base := today
	switch {
	case r.Strict && hasDue:
		base = due
	case r.Strict && hasThr:
		base = thr
	}

	switch {
	case hasDue:
		nd := r.Add(base)
		n.SetTag("due", nd.Format(DateLayout))
		if hasThr {
			gap := int(math.Round(thr.Sub(due).Hours() / 24)) // whole days, DST-safe
			n.SetTag("t", nd.AddDate(0, 0, gap).Format(DateLayout))
		}
	case hasThr:
		n.SetTag("t", r.Add(base).Format(DateLayout))
	default:
		n.SetTag("due", r.Add(base).Format(DateLayout))
	}
	t.nextAlarm(n, r, today)
	return n, true
}
