package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// The agenda is the task list narrowed to one week of due dates and
// grouped by day: tasks without a time first, then timed ones in time
// order. It is still the list, so the cursor moves over its tasks and every
// action works on them. It shares its date with the calendar, so c and w
// switch between the two on the same day.

// agendaHelp is the footer while the agenda is open.
const agendaHelp = "j/k task · h/l week · t this week · a add on the day · c calendar · esc close"

func weekStart(d time.Time) time.Time {
	d = listview.Today(d)
	return d.AddDate(0, 0, -int(d.Weekday()))
}

func (m *Model) toggleAgenda() {
	if m.agenda {
		m.agenda = false
		m.refresh(nil)
		return
	}
	m.endVisual()
	m.agenda, m.calendar, m.focus = true, false, focusList
	if m.calDay.IsZero() {
		m.calDay = listview.Today(time.Now())
	}
	m.agendaGoTo(m.calDay)
}

// agendaGoTo shows the week holding d, with the cursor on the first task
// due on or after d.
func (m *Model) agendaGoTo(d time.Time) {
	m.calDay = listview.Today(d)
	m.cursor, m.offset = 0, 0
	m.refresh(nil)
	for i, t := range m.visible {
		if due, _ := t.Due(); !due.Before(m.calDay) {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
}

// agendaFilter narrows the sorted list to the week, ordered by day, then
// all-day before timed, then time; ties keep the list's sort.
func (m *Model) agendaFilter() {
	start := weekStart(m.calDay)
	end := start.AddDate(0, 0, 7)
	var out []*todo.Task
	for _, t := range m.visible {
		if d, ok := t.Due(); ok && !d.Before(start) && d.Before(end) {
			out = append(out, t)
		}
	}
	key := func(t *todo.Task) (time.Time, int) {
		d, _ := t.Due()
		if c, ok := t.Clock(); ok {
			return d, c
		}
		return d, -1
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, ci := key(out[i])
		dj, cj := key(out[j])
		if !di.Equal(dj) {
			return di.Before(dj)
		}
		return ci < cj
	})
	m.visible = out
}

// agendaDay is the day a new task gets: the cursor task's, else the
// chosen day.
func (m *Model) agendaDay() time.Time {
	if t := m.selected(); t != nil {
		if d, ok := t.Due(); ok {
			return d
		}
	}
	return m.calDay
}

// agendaKey handles a key in the agenda, reporting whether it did. As in
// the calendar, keys are matched by their action, so [keys] apply.
func (m *Model) agendaKey(k string) bool {
	name := ""
	if a := m.km.single[k]; a != nil {
		name = a.name
	}
	switch {
	case name == "focus_sidebar", name == "" && k == "[":
		m.agendaGoTo(m.calDay.AddDate(0, 0, -7))
	case name == "focus_tasks", name == "" && k == "]":
		m.agendaGoTo(m.calDay.AddDate(0, 0, 7))
	case name == "" && k == "t":
		m.agendaGoTo(time.Now())
	case name == "add":
		m.calDay = m.agendaDay()
		m.calendarAdd()
	case name == "move_up", name == "move_down":
		m.setStatus("the agenda orders tasks by day and time — close it (esc) to reorder")
	case name == "agenda", name == "quit":
		m.toggleAgenda()
	case name == "clear_filter" && !m.visual:
		m.toggleAgenda()
	default:
		return false
	}
	return true
}

// ---- terminal view -------------------------------------------------------------

// agendaLine is one line of the agenda: a day heading or a task.
type agendaLine struct {
	day  time.Time
	task int // index into m.visible, or -1 for a heading or "nothing due"
	head bool
}

func (m Model) agendaLines() []agendaLine {
	start := weekStart(m.calDay)
	var out []agendaLine
	i := 0
	for d := range 7 {
		day := start.AddDate(0, 0, d)
		out = append(out, agendaLine{day: day, task: -1, head: true})
		n := 0
		for ; i < len(m.visible); i++ {
			if due, _ := m.visible[i].Due(); !sameDay(due, day) {
				break
			}
			out = append(out, agendaLine{day: day, task: i})
			n++
		}
		if n == 0 {
			out = append(out, agendaLine{day: day, task: -1})
		}
	}
	return out
}

func weekTitle(start time.Time) string {
	end := start.AddDate(0, 0, 6)
	switch {
	case start.Year() != end.Year():
		return start.Format("Jan 2, 2006") + " – " + end.Format("Jan 2, 2006")
	case start.Month() != end.Month():
		return start.Format("Jan 2") + " – " + end.Format("Jan 2, 2006")
	}
	return start.Format("Jan 2") + " – " + end.Format("2, 2006")
}

func (m Model) viewAgenda(w, h int) string {
	start := weekStart(m.calDay)
	today := listview.Today(time.Now())
	title := m.st.prompt.Render("‹ " + weekTitle(start) + " ›")
	right := m.st.faint.Render(plural(len(m.visible), "task"))
	head := title + strings.Repeat(" ", max(w-widthOf(title)-widthOf(right), 1)) + right

	const gutter = 7 // " 14:30 "
	lines := m.agendaLines()
	cur := 0
	for i, l := range lines {
		if l.task == m.cursor && l.task >= 0 {
			cur = i
		}
	}
	rows := h - 1
	first := 0
	if len(lines) > rows {
		first = clamp(cur-rows/3, 0, len(lines)-rows)
	}

	hide := map[string]bool{}
	for _, k := range m.cfg.View.HideKeys {
		hide[k] = true
	}
	now := time.Now()
	out := []string{head}
	for _, l := range lines[first:min(len(lines), first+rows)] {
		switch {
		case l.head:
			st := m.st.header
			label := l.day.Format("Mon Jan 2")
			switch {
			case l.day.Equal(today):
				st, label = m.st.today, label+" · today"
			case l.day.Before(today):
				st = m.st.faint
			}
			rule := m.st.faint.Render(" " + strings.Repeat("─", max(w-widthOf(label)-1, 0)))
			out = append(out, st.Render(label)+rule)
		case l.task < 0:
			out = append(out, m.st.faint.Render(strings.Repeat(" ", gutter)+"—"))
		default:
			t := m.visible[l.task]
			clock := strings.Repeat(" ", gutter)
			if c, ok := t.Clock(); ok {
				clock = fmt.Sprintf("%6s ", todo.FormatClock(c))
			}
			r := row{cursor: l.task == m.cursor, focused: m.focus == focusList, marked: m.sel[t], hide: hide}
			out = append(out, m.st.faint.Render(clock)+m.renderTask(t, r, w-gutter, now))
		}
	}
	return strings.Join(out, "\n")
}

// ---- desktop app ---------------------------------------------------------------

// Agenda is the agenda as data, for katTasks-gui.
type Agenda struct {
	Title string      `json:"title"` // "Nov 15 – 21, 2026"
	Days  []AgendaDay `json:"days"`  // Sunday to Saturday
	Count int         `json:"count"`
	// FromHour and ToHour bound the hour grid: 7 to 21, widened to fit the
	// week's timed tasks.
	FromHour int `json:"fromHour"`
	ToHour   int `json:"toHour"`
}

// AgendaDay is one day of the agenda.
type AgendaDay struct {
	Date   string       `json:"date"`
	Label  string       `json:"label"` // "Wed 18"
	Today  bool         `json:"today"`
	Past   bool         `json:"past"`
	AllDay []AgendaItem `json:"allDay"`
	Timed  []AgendaItem `json:"timed"` // in time order
}

// AgendaItem is a task in the agenda. Row.Index is its place in the list,
// for clicks.
type AgendaItem struct {
	Row     TaskRow `json:"row"`
	Minutes int     `json:"minutes"` // after midnight; timed items only
	Time    string  `json:"time"`
}

func (m Model) agendaSnapshot() *Agenda {
	start := weekStart(m.calDay)
	today := listview.Today(time.Now())
	a := &Agenda{Title: weekTitle(start), Count: len(m.visible), FromHour: 7, ToHour: 21}
	hide, now := m.hiddenKeys(), time.Now()
	byDay := map[string]*AgendaDay{}
	for d := range 7 {
		day := start.AddDate(0, 0, d)
		a.Days = append(a.Days, AgendaDay{
			Date: day.Format(todo.DateLayout), Label: day.Format("Mon 2"),
			Today: day.Equal(today), Past: day.Before(today),
		})
	}
	for i := range a.Days {
		byDay[a.Days[i].Date] = &a.Days[i]
	}
	for i, t := range m.visible {
		due, _ := t.Due()
		ad := byDay[due.Format(todo.DateLayout)]
		if ad == nil {
			continue
		}
		r := m.taskRow(t, hide, now)
		r.Index, r.Cursor = i, i == m.cursor
		if c, ok := t.Clock(); ok {
			ad.Timed = append(ad.Timed, AgendaItem{Row: r, Minutes: c, Time: todo.FormatClock(c)})
			a.FromHour = min(a.FromHour, c/60)
			a.ToHour = max(a.ToHour, c/60+1)
		} else {
			ad.AllDay = append(ad.AllDay, AgendaItem{Row: r})
		}
	}
	a.ToHour = min(a.ToHour, 24)
	return a
}

// agendaWeek moves the agenda n weeks.
func (m *Model) agendaWeek(n int) tea.Cmd {
	if m.agenda {
		m.agendaGoTo(m.calDay.AddDate(0, 0, 7*n))
	}
	return nil
}
