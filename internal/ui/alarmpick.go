package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// The alarm picker opens over the add/edit popup when "alarm:" is typed
// (or tab is pressed on an alarm: word), to choose the alarm's date on a
// month grid and its time, then writes alarm:YYYY-MM-DDTHH:MM into the
// line.

type alarmPick struct {
	day   time.Time // the chosen date, at midnight
	min   int       // the chosen time, minutes after midnight
	field int       // pickDate or pickTime
	typed string    // a time being typed in the time field
	start int       // the alarm: word, in runes of the input
	end   int
	fresh bool // the word was a bare "alarm:" typed just now
}

const (
	pickDate = iota
	pickTime
)

// openAlarmPicker opens the picker on the alarm: word that ends at the
// cursor. A valid value starts it there; otherwise it starts at
// alarms.default_time, today if that's still ahead and tomorrow if not.
func (m *Model) openAlarmPicker() {
	start, w := m.wordAtCursor()
	p := &alarmPick{start: start, end: start + len([]rune(w)), fresh: strings.EqualFold(w, "alarm:")}
	if a, ok := todo.ParseAlarm(w[len("alarm:"):]); ok {
		p.day, p.min = todo.Midnight(a), a.Hour()*60+a.Minute()
	} else {
		now := time.Now()
		p.day, p.min = todo.Midnight(now), m.cfg.Alarms.DefaultMinutes
		if now.Hour()*60+now.Minute() >= p.min {
			p.day = p.day.AddDate(0, 0, 1)
		}
	}
	m.pick, m.sugg = p, nil
}

func (p *alarmPick) when() time.Time {
	return p.day.Add(time.Duration(p.min) * time.Minute)
}

// isAlarmWord reports whether w is an alarm: word the picker can open on.
func isAlarmWord(w string) bool { return strings.HasPrefix(strings.ToLower(w), "alarm:") }

// alarmPickerKey handles a key while the picker is open.
func (m *Model) alarmPickerKey(k string) {
	p := m.pick
	switch k {
	case "esc":
		m.closeAlarmPicker(false)
		return
	case "enter":
		m.closeAlarmPicker(true)
		return
	case "tab", "shift+tab":
		p.field, p.typed = 1-p.field, ""
		return
	}
	if p.field == pickDate {
		switch k {
		case "left", "h":
			p.day = p.day.AddDate(0, 0, -1)
		case "right", "l":
			p.day = p.day.AddDate(0, 0, 1)
		case "up", "k":
			p.day = p.day.AddDate(0, 0, -7)
		case "down", "j":
			p.day = p.day.AddDate(0, 0, 7)
		case "[", "pgup":
			p.day = addMonths(p.day, -1)
		case "]", "pgdown":
			p.day = addMonths(p.day, 1)
		case "t":
			p.day = todo.Midnight(time.Now())
		case "T":
			p.day = todo.Midnight(time.Now()).AddDate(0, 0, 1)
		}
		return
	}
	step := func(n int) {
		p.min = ((p.min+n)%(24*60) + 24*60) % (24 * 60)
		p.typed = ""
	}
	switch k {
	case "up", "k":
		step(15)
	case "down", "j":
		step(-15)
	case "right", "l":
		step(60)
	case "left", "h":
		step(-60)
	case "backspace":
		if p.typed != "" {
			p.typed = p.typed[:len(p.typed)-1]
		}
	default:
		// Typing a time: "6:45", "645", "7pm". It's used as soon as it reads
		// as one, so 7, 3, 0 goes 7:00, then (73 isn't a time) 7:30.
		if len(k) == 1 && strings.Contains("0123456789:apm", k) && len(p.typed) < 7 {
			p.typed += k
			if min, ok := todo.ParseClock(p.typed, true); ok {
				p.min = min
			}
		}
	}
}

// pickDay chooses a day of the grid (a click in katTasks-gui).
func (m *Model) pickDay(i int) {
	if m.pick == nil {
		return
	}
	days := pickGrid(m.pick.day)
	if i >= 0 && i < len(days) {
		m.pick.day = days[i]
		m.pick.field = pickDate
	}
}

// closeAlarmPicker writes the chosen alarm into the line, or on cancel
// takes back a bare "alarm:" just typed.
func (m *Model) closeAlarmPicker(set bool) {
	p := m.pick
	m.pick = nil
	val := []rune(m.input.Value())
	end := min(p.end, len(val))
	start := min(p.start, end)
	var ins []rune
	switch {
	case set:
		ins = []rune("alarm:" + todo.FormatAlarm(p.when()))
		if end >= len(val) || val[end] != ' ' {
			ins = append(ins, ' ')
		}
		if p.when().Before(time.Now()) {
			m.setStatus("that alarm is in the past, so it won't ring")
		}
	case p.fresh:
		// nothing: drop the bare alarm:
	default:
		return // keep what was there
	}
	m.input.SetValue(string(val[:start]) + string(ins) + string(val[end:]))
	m.input.SetCursor(start + len(ins))
	m.updateSuggestions()
}

// pickGrid is the six weeks, Sunday first, around d's month.
func pickGrid(d time.Time) []time.Time {
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	days := make([]time.Time, 42)
	for i := range days {
		days[i] = start.AddDate(0, 0, i)
	}
	return days
}

// untilText says how far off t is: "in 2h 15m", "in 3 days", "in the past".
func untilText(t, now time.Time) string {
	d := t.Sub(now)
	switch {
	case d < 0:
		return "in the past"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes())+1)
	case d < 24*time.Hour:
		return fmt.Sprintf("in %dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(todo.Midnight(t).Sub(todo.Midnight(now)).Hours()/24 + 0.5)
	if days == 1 {
		return "tomorrow"
	}
	return fmt.Sprintf("in %d days", days)
}

func (p *alarmPick) clock() string {
	return todo.FormatClock(p.min)
}

// viewAlarmPicker draws the picker for the task popup, w cells wide.
func (m Model) viewAlarmPicker(w int) []string {
	p := m.pick
	now := time.Now()
	key := func(k string) string { return m.st.helpKey.Render(k) }
	faint := m.st.faint.Render
	// The chip without its side padding, to keep the grid lined up; sel is
	// the choice in the field that doesn't have focus.
	on, sel := m.st.suggestOn.Padding(0), m.st.text.Background(m.st.selectedBg)

	head := m.st.header.Render("⏰ ALARM") + "  " + m.st.text.Render(p.when().Format("Mon Jan 2, 2006")+"  "+p.clock()) +
		faint("  "+untilText(p.when(), now))
	lines := []string{head, ""}

	// Month grid, left; time, right.
	var grid []string
	title := p.day.Format("January 2006")
	grid = append(grid, m.st.title.Render(fmt.Sprintf("%-20s", " "+title)))
	grid = append(grid, faint(" Su Mo Tu We Th Fr Sa"))
	days := pickGrid(p.day)
	for wk := 0; wk < 6; wk++ {
		var row strings.Builder
		row.WriteString(" ")
		for i := wk * 7; i < wk*7+7; i++ {
			d := days[i]
			cell := fmt.Sprintf("%2d", d.Day())
			st := m.st.text
			switch {
			case sameDay(d, p.day) && p.field == pickDate:
				st = on
			case sameDay(d, p.day):
				st = sel
			case sameDay(d, now):
				st = m.st.today
			case d.Month() != p.day.Month() || d.Before(todo.Midnight(now)):
				st = m.st.faint
			}
			row.WriteString(st.Render(cell))
			if i < wk*7+6 {
				row.WriteString(" ")
			}
		}
		grid = append(grid, row.String())
	}

	timeBox := " " + p.clock() + " "
	if p.field == pickTime {
		timeBox = on.Render(timeBox)
	} else {
		timeBox = sel.Render(timeBox)
	}
	side := []string{"", "", faint("Time"), timeBox}
	if p.field == pickTime && p.typed != "" {
		side = append(side, faint("typed ")+m.st.text.Render(p.typed))
	}
	for i, g := range grid {
		s := ""
		if i < len(side) {
			s = side[i]
		}
		lines = append(lines, g+"    "+s)
	}

	lines = append(lines, "")
	if p.field == pickDate {
		lines = append(lines, key("←→↑↓")+faint(" day  ")+key("[ ]")+faint(" month  ")+key("t")+faint(" today  ")+key("T")+faint(" tomorrow"))
	} else {
		lines = append(lines, key("↑↓")+faint(" 15 min  ")+key("←→")+faint(" hour  ")+faint("or type ")+key("6:45")+faint(" / ")+key("7pm"))
	}
	lines = append(lines, key("tab")+faint(" date / time  ")+key("enter")+faint(" set  ")+key("esc")+faint(" cancel"))
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "…")
	}
	return lines
}

// AlarmPicker is the picker as data, for katTasks-gui.
type AlarmPicker struct {
	When     string    `json:"when"`  // "Tue Dec 1, 2026  7:30"
	Until    string    `json:"until"` // "in 2h 15m"
	Month    string    `json:"month"` // "December 2026"
	Weekdays []string  `json:"weekdays"`
	Days     []PickDay `json:"days"` // six weeks, Sunday first
	Time     string    `json:"time"` // "7:30"
	Typed    string    `json:"typed"`
	Field    string    `json:"field"` // "date" or "time"
}

// PickDay is one day of the picker's grid.
type PickDay struct {
	Day      int  `json:"day"`
	InMonth  bool `json:"inMonth"`
	Today    bool `json:"today"`
	Past     bool `json:"past"`
	Selected bool `json:"selected"`
}

func (m Model) alarmPickerState() *AlarmPicker {
	p := m.pick
	if p == nil {
		return nil
	}
	now := time.Now()
	ap := &AlarmPicker{
		When: p.when().Format("Mon Jan 2, 2006") + "  " + p.clock(), Until: untilText(p.when(), now),
		Month: p.day.Format("January 2006"), Weekdays: []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
		Time: p.clock(), Typed: p.typed, Field: "date",
	}
	if p.field == pickTime {
		ap.Field = "time"
	}
	for _, d := range pickGrid(p.day) {
		ap.Days = append(ap.Days, PickDay{Day: d.Day(), InMonth: d.Month() == p.day.Month(), Today: sameDay(d, now),
			Past: d.Before(todo.Midnight(now)), Selected: sameDay(d, p.day)})
	}
	return ap
}
