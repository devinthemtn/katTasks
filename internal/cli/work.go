package cli

// Commands for the tags katTasks adds beyond todo.sh: in progress
// (status:, progress:), times (at:), alarms (alarm:), and the alerts and
// alarms coming up.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/alert"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// openTasks resolves task numbers like several, refusing done tasks.
func openTasks(c *ctx, args []string) ([]numbered, error) {
	ts, err := several(c, args)
	if err != nil {
		return nil, err
	}
	for _, x := range ts {
		if x.t.Done {
			return nil, fmt.Errorf("task %d is done", x.n)
		}
	}
	return ts, nil
}

func (c *ctx) saveChanged(msg string, ts []numbered) error {
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(msg, ts, nil)
}

func cmdStart(c *ctx, args []string) error {
	ts, err := openTasks(c, args)
	if err != nil {
		return err
	}
	for _, x := range ts {
		x.t.SetTag("status", "doing")
	}
	return c.saveChanged(fmt.Sprintf("%d started.", len(ts)), ts)
}

func cmdStop(c *ctx, args []string) error {
	ts, err := several(c, args)
	if err != nil {
		return err
	}
	for _, x := range ts {
		x.t.RemoveTag("status")
	}
	return c.saveChanged(fmt.Sprintf("%d stopped.", len(ts)), ts)
}

// cmdStatus sets any status: status 3 blocked; status 3 none clears it.
func cmdStatus(c *ctx, args []string) error {
	if len(args) != 2 {
		return c.bad("need a task number and a status")
	}
	ts, err := openTasks(c, args[:1])
	if err != nil {
		return err
	}
	t, s := ts[0].t, strings.ToLower(args[1])
	if strings.ContainsAny(s, ": \t") {
		return c.bad("a status is one word without a colon, e.g. doing or blocked")
	}
	if s == "none" {
		t.RemoveTag("status")
		return c.saveChanged(fmt.Sprintf("%d status cleared.", ts[0].n), ts)
	}
	t.SetTag("status", s)
	return c.saveChanged(fmt.Sprintf("%d status %s.", ts[0].n, s), ts)
}

// cmdProgress sets progress: progress 3 40, progress 3 +10, progress 3 none.
func cmdProgress(c *ctx, args []string) error {
	if len(args) != 2 {
		return c.bad("need a task number and a percent")
	}
	ts, err := openTasks(c, args[:1])
	if err != nil {
		return err
	}
	t, v := ts[0].t, strings.TrimSuffix(args[1], "%")
	cur, _ := t.Progress()
	var p int
	switch {
	case v == "none":
		p = 0
	case strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-"):
		d, err := strconv.Atoi(v)
		if err != nil {
			return c.bad("%q isn't a percent", args[1])
		}
		p = min(max(cur+d, 0), 100)
	default:
		n, ok := todo.ParsePercent(v)
		if !ok {
			return c.bad("progress is 0 to 100, or +N / -N to change it")
		}
		p = n
	}
	t.SetProgress(p)
	if p == 0 {
		return c.saveChanged(fmt.Sprintf("%d progress cleared.", ts[0].n), ts)
	}
	return c.saveChanged(fmt.Sprintf("%d is %d%% done.", ts[0].n, p), ts)
}

// relative reads "+5m", "+1h30m", "+90s" or "in 5m" from the start of
// args: how long from now, and how many arguments it took.
func relative(args []string) (time.Duration, int, bool) {
	parse := func(s string) (time.Duration, bool) {
		d, err := time.ParseDuration(strings.TrimPrefix(s, "+"))
		return d, err == nil && d > 0
	}
	switch {
	case len(args) >= 1 && strings.HasPrefix(args[0], "+"):
		if d, ok := parse(args[0]); ok {
			return d, 1, true
		}
	case len(args) >= 2 && strings.EqualFold(args[0], "in"):
		if d, ok := parse(args[1]); ok {
			return d, 2, true
		}
	}
	return 0, 0, false
}

// shortDuration writes 20m rather than Go's 20m0s, and 2h rather than 2h0m0s.
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// target is the task an at or alarm command works on: task N when the
// first argument is a number, else a new task made of the words after
// WHEN, which read(args) recognizes and says the length of.
func (c *ctx) target(args []string, read func([]string) (int, bool)) (t *todo.Task, n int, when []string, added bool, err error) {
	if _, numErr := strconv.Atoi(args[0]); numErr == nil {
		ts, err := openTasks(c, args[:1])
		if err != nil {
			return nil, 0, nil, false, err
		}
		return ts[0].t, ts[0].n, args[1:], false, nil
	}
	used, ok := read(args)
	if !ok || used >= len(args) {
		return nil, 0, nil, false, c.bad("give a task number, or when and then the new task's text")
	}
	t = todo.NewTask(strings.Join(args[used:], " "), c.Now(), c.Cfg.Tasks.AddCreationDate)
	c.tasks = append(c.tasks, t)
	return t, len(c.tasks), args[:used], true, nil
}

// cmdAt sets the time of an alert: at 3 9:30, at 3 3pm, at 3 none, or
// at 3 +5m, which alerts in 5 minutes: it sets due: to that day and the
// at: time alerts.lead later, as an alert goes off lead before it.
// alert +5m Check the oven adds a task to do it.
func cmdAt(c *ctx, args []string) error {
	if len(args) < 2 {
		return c.bad("need a task number (or text) and a time")
	}
	now, lead := c.Now(), c.Cfg.Alerts.LeadTime
	t, n, when, added, err := c.target(args, func(a []string) (int, bool) {
		if _, used, ok := relative(a); ok {
			return used, true
		}
		_, ok := todo.ParseClock(a[0], true)
		return 1, ok
	})
	if err != nil {
		return err
	}
	if len(when) == 1 && strings.EqualFold(when[0], "none") {
		t.RemoveTag("at")
		return c.saveChanged(fmt.Sprintf("%d time cleared.", n), []numbered{{n, t}})
	}
	var msg string
	if d, used, ok := relative(when); ok && used == len(when) {
		fire := now.Add(d).Round(time.Minute)
		at := fire.Add(lead)
		t.SetTag("due", at.Format(todo.DateLayout))
		t.SetTag("at", todo.FormatClockTag(at.Hour()*60+at.Minute()))
		msg = fmt.Sprintf("%d alerts at %s (in %s).", n, todo.ClockOf(fire), shortDuration(d))
	} else {
		if len(when) != 1 {
			return c.bad("%q isn't a time; try 9:30, 1430, 3pm or +5m", strings.Join(when, " "))
		}
		min, ok := todo.ParseClock(when[0], true)
		if !ok {
			return c.bad("%q isn't a time; try 9:30, 1430, 3pm or +5m", when[0])
		}
		t.SetTag("at", strings.ToLower(when[0]))
		t.Normalize(now) // writes it without a colon
		msg = fmt.Sprintf("%d at %s.", n, todo.FormatClock(min))
	}
	if !c.Cfg.Alerts.Enabled {
		msg += " Alerts are off in the settings ([alerts] enabled)."
	}
	return c.saveAdded(msg, t, n, added)
}

// cmdAlarm sets an alarm: alarm 3 +5m, alarm 3 in 2h, alarm 3 tomorrow
// 6:30, alarm 3 fri 7am, alarm 3 2026-12-01 0730, alarm 3 18:00 (the next
// 18:00), alarm 3 none. alarm +5m Tea is ready adds a task to ring.
func cmdAlarm(c *ctx, args []string) error {
	if len(args) < 2 {
		return c.bad("need a task number (or when) and when (or text)")
	}
	t, n, when, added, err := c.target(args, func(a []string) (int, bool) {
		// The longest WHEN that reads, leaving words for the text.
		for used := min(2, len(a)-1); used >= 1; used-- {
			if _, ok := c.alarmTime(a[:used]); ok {
				return used, true
			}
		}
		return 0, false
	})
	if err != nil {
		return err
	}
	if len(when) == 1 && strings.EqualFold(when[0], "none") {
		t.RemoveTag("alarm")
		return c.saveChanged(fmt.Sprintf("%d alarm cleared.", n), []numbered{{n, t}})
	}
	at, ok := c.alarmTime(when)
	if !ok {
		return c.bad("%q isn't a date and time; try +5m, tomorrow 6:30, fri 7am or 2026-12-01 0730", strings.Join(when, " "))
	}
	t.SetTag("alarm", todo.FormatAlarm(at))
	msg := fmt.Sprintf("%d alarm %s.", n, at.Format("Mon Jan 2 ")+todo.ClockOf(at))
	if at.Before(c.Now().Add(-time.Minute)) {
		msg += " That's in the past, so it won't ring."
	}
	if !c.Cfg.Alarms.Enabled {
		msg += " Alarms are off in the settings ([alarms] enabled)."
	}
	return c.saveAdded(msg, t, n, added)
}

// saveAdded saves and reports a task an at or alarm command changed, or
// added when it was given text instead of a task number.
func (c *ctx) saveAdded(msg string, t *todo.Task, n int, added bool) error {
	if !added {
		return c.saveChanged(msg, []numbered{{n, t}})
	}
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d added. ", n)+msg, []numbered{{n, t}}, nil)
}

// alarmTime reads when an alarm rings: from now (+5m, +1h30m, in 2h), an
// alarm: value, a date (YYYY-MM-DD, today, tomorrow, mon..sun, +3d) and a
// time, just a time (the next one), or just a date (at
// alarms.default_time). In +5m, m is minutes; +3d is days.
func (c *ctx) alarmTime(args []string) (time.Time, bool) {
	now := c.Now()
	if d, used, ok := relative(args); ok && used == len(args) {
		return now.Add(d).Round(time.Minute), true
	}
	if len(args) == 1 {
		if a, ok := todo.ParseAlarm(args[0]); ok {
			return a, true
		}
	}
	date := func(s string) (time.Time, bool) {
		if d, err := time.ParseInLocation(todo.DateLayout, s, time.Local); err == nil {
			return d, true
		}
		if _, err := time.ParseDuration(strings.TrimPrefix(s, "+")); err == nil {
			return time.Time{}, false // +5m is minutes here, never months as in due:+1m
		}
		return todo.ResolveDate(s, now)
	}
	at := func(d time.Time, min int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), min/60, min%60, 0, 0, time.Local)
	}
	switch len(args) {
	case 1:
		if min, ok := todo.ParseClock(args[0], true); ok {
			a := at(now, min)
			if !a.After(now) {
				a = at(now.AddDate(0, 0, 1), min)
			}
			return a, true
		}
		if d, ok := date(args[0]); ok {
			return at(d, c.Cfg.Alarms.DefaultMinutes), true
		}
	case 2:
		d, okD := date(args[0])
		min, okT := todo.ParseClock(args[1], true)
		if okD && okT {
			return at(d, min), true
		}
	}
	return time.Time{}, false
}

// Coming is the JSON form of an upcoming alert or alarm.
type Coming struct {
	Kind  string `json:"kind"`  // "alert" or "alarm"
	At    string `json:"at"`    // the task's time, RFC 3339
	Fires string `json:"fires"` // when it goes off: At minus alerts.lead for alerts
	Task  Task   `json:"task"`
}

// cmdAlerts lists the alerts and alarms of the next DAYS days (default 7).
func cmdAlerts(c *ctx, args []string) error {
	days := 7
	if len(args) > 1 {
		return c.bad("too many arguments")
	}
	if len(args) == 1 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return c.bad("DAYS is a number of days, 1 or more")
		}
		days = n
	}
	now := c.Now()
	to := now.AddDate(0, 0, days)
	lead := c.Cfg.Alerts.LeadTime
	var as []alert.Alert
	if c.Cfg.Alerts.Enabled {
		as = append(as, alert.Between(c.tasks, now, to, lead)...)
	}
	if c.Cfg.Alarms.Enabled {
		as = append(as, alert.AlarmsBetween(c.tasks, now, to)...)
	}
	fires := func(a alert.Alert) time.Time {
		if a.Alarm {
			return a.At
		}
		return a.At.Add(-lead)
	}
	sort.SliceStable(as, func(i, j int) bool { return fires(as[i]).Before(fires(as[j])) })

	if c.JSON {
		out := []Coming{}
		for _, a := range as {
			kind := "alert"
			if a.Alarm {
				kind = "alarm"
			}
			out = append(out, Coming{Kind: kind, At: a.At.Format(time.RFC3339), Fires: fires(a).Format(time.RFC3339), Task: toJSON(c.index(a.Task), a.Task)})
		}
		return c.printJSON(out)
	}
	for _, a := range as {
		kind := "alert"
		if a.Alarm {
			kind = "alarm"
		}
		fmt.Fprintf(c.Out, "%s %5s  %s  %d %s\n", fires(a).Format("Mon Jan 2"), todo.ClockOf(fires(a)), kind, c.index(a.Task), a.Task)
	}
	var off []string
	if !c.Cfg.Alerts.Enabled {
		off = append(off, "alerts")
	}
	if !c.Cfg.Alarms.Enabled {
		off = append(off, "alarms")
	}
	fmt.Fprintf(c.Out, "--\nTODO: %d coming up in the next %s", len(as), plural(days, "day"))
	if len(off) > 0 {
		fmt.Fprintf(c.Out, " (%s off in the settings)", strings.Join(off, " and "))
	}
	fmt.Fprintln(c.Out, ".")
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
