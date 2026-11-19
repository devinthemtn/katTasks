package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// The calendar shows a month of due dates in place of the task list. It
// shows the tasks of the sidebar entry and search, so +work's calendar has
// only work. Enter shows the chosen day's tasks in the list, where every
// action works on them.

const calCells = 42 // six weeks, so the grid never changes height

// calDay is one day of the calendar grid.
type calDay struct {
	date    time.Time
	inMonth bool
	tasks   []*todo.Task // due that day, in list order
	open    int          // how many aren't done
}

// calMonth is the grid for the month holding m.calDay.
type calMonth struct {
	days     [calCells]calDay
	selected int // index into days
	today    time.Time
}

func (m *Model) toggleCalendar() {
	if m.calendar {
		m.calendar = false
		return
	}
	m.endVisual()
	m.calendar, m.focus = true, focusList
	if m.agenda {
		m.calDay = m.agendaDay()
		m.agenda = false
		m.refresh(nil)
	}
	if m.calDay.IsZero() {
		m.calDay = listview.Today(time.Now())
	}
}

// calendarMonth builds the grid. Weeks start on Sunday.
func (m *Model) calendarMonth() calMonth {
	now := time.Now()
	cm := calMonth{today: listview.Today(now)}
	y, mo, _ := m.calDay.Date()
	first := time.Date(y, mo, 1, 0, 0, 0, 0, time.Local)
	start := first.AddDate(0, 0, -int(first.Weekday()))

	byDate := map[string][]*todo.Task{}
	for _, t := range listview.Visible(m.tasks, m.currentEntry(), m.q, m.hideDone, sortModes[m.sortIdx], now) {
		if d, ok := t.Due(); ok {
			k := d.Format(todo.DateLayout)
			byDate[k] = append(byDate[k], t)
		}
	}
	for i := range cm.days {
		d := start.AddDate(0, 0, i)
		cd := calDay{date: d, inMonth: d.Month() == mo, tasks: byDate[d.Format(todo.DateLayout)]}
		for _, t := range cd.tasks {
			if !t.Done {
				cd.open++
			}
		}
		cm.days[i] = cd
		if sameDay(d, m.calDay) {
			cm.selected = i
		}
	}
	return cm
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// calendarKey handles a key in the calendar, reporting whether it did.
// Keys are matched by the action they're bound to, so [keys] overrides of
// the movement keys apply here too; other keys work as in the list.
func (m *Model) calendarKey(k string) bool {
	name := ""
	if a := m.km.single[k]; a != nil {
		name = a.name
	}
	switch {
	case name == "focus_sidebar":
		m.calDay = m.calDay.AddDate(0, 0, -1)
	case name == "focus_tasks":
		m.calDay = m.calDay.AddDate(0, 0, 1)
	case name == "down":
		m.calDay = m.calDay.AddDate(0, 0, 7)
	case name == "up":
		m.calDay = m.calDay.AddDate(0, 0, -7)
	case name == "half_page_down", name == "" && k == "]":
		m.calDay = addMonths(m.calDay, 1)
	case name == "half_page_up", name == "" && k == "[":
		m.calDay = addMonths(m.calDay, -1)
	case name == "top", name == "" && k == "t":
		m.calDay = listview.Today(time.Now())
	case name == "bottom":
		y, mo, _ := m.calDay.Date()
		m.calDay = time.Date(y, mo+1, 0, 0, 0, 0, 0, time.Local)
	case name == "edit":
		m.showDay(nil)
	case name == "add":
		m.calendarAdd()
	case name == "calendar", name == "clear_filter", name == "quit":
		m.calendar = false
	case name != "" && actionByName[name].needsTask, name == "visual", name == "select_all":
		m.setStatus("press enter to list the day's tasks, then act on them")
	default:
		return false
	}
	return true
}

// addMonths moves d by n months, keeping the day where the month has it
// (Jan 31 + 1 month is Feb 28, not Mar 3).
func addMonths(d time.Time, n int) time.Time {
	y, mo, day := d.Date()
	last := time.Date(y, mo+time.Month(n)+1, 0, 0, 0, 0, 0, time.Local).Day()
	return time.Date(y, mo+time.Month(n), min(day, last), 0, 0, 0, 0, time.Local)
}

// showDay leaves the calendar and lists the chosen day's tasks, with the
// cursor on sel if given.
func (m *Model) showDay(sel *todo.Task) {
	m.calendar = false
	date := "due:" + m.calDay.Format(todo.DateLayout)
	m.filter = strings.TrimSpace(date + " " + m.filter)
	m.cursor, m.offset = 0, 0
	m.refresh(sel)
	m.setStatus("tasks due %s — esc to clear", m.calDay.Format("Mon Jan 2"))
}

// calendarAdd adds a task due on the chosen day.
func (m *Model) calendarAdd() {
	m.startAdd()
	due := "due:" + m.calDay.Format(todo.DateLayout)
	val := strings.TrimSpace(m.input.Value())
	if val == "" {
		m.startInput(" "+due, 0)
	} else {
		m.startInput(" "+val+" "+due, 0)
	}
}

// ---- terminal view -------------------------------------------------------------

func (m Model) viewCalendar(w, h int) string {
	cm := m.calendarMonth()
	cw := clamp(w/7, 4, 12)
	month := 0
	for _, d := range cm.days {
		if d.inMonth {
			month += d.open
		}
	}
	title := m.st.prompt.Render("‹ " + m.calDay.Format("January 2006") + " ›")
	right := m.st.faint.Render(fmt.Sprintf("%s due", plural(month, "task")))
	lines := []string{title + strings.Repeat(" ", max(cw*7-widthOf(title)-widthOf(right), 1)) + right}

	var wd []string
	for i := range 7 {
		wd = append(wd, " "+padRight(time.Weekday(i).String()[:2], cw-1)) // over the day numbers
	}
	lines = append(lines, m.st.header.Render(strings.Join(wd, "")))

	for wk := range 6 {
		var row string
		for i := wk * 7; i < wk*7+7; i++ {
			row += m.calCell(cm, i, cw)
		}
		lines = append(lines, row)
	}

	sel := cm.days[cm.selected]
	lines = append(lines, "", m.st.header.Render(sel.date.Format("Monday, January 2"))+
		m.st.faint.Render(" · "+plural(len(sel.tasks), "task")))
	room := h - len(lines)
	now := time.Now()
	for i, t := range sel.tasks {
		if i == room-1 && len(sel.tasks) > room {
			lines = append(lines, m.st.faint.Render(fmt.Sprintf("  … %d more — enter lists them", len(sel.tasks)-i)))
			break
		}
		lines = append(lines, m.renderTask(t, row{}, w, now))
	}
	if len(sel.tasks) == 0 {
		lines = append(lines, m.st.faint.Render("  nothing due — a adds a task for this day"))
	}
	return strings.Join(lines, "\n")
}

// calCell draws one day: its number, then a dot and the count of open
// tasks due, in the overdue color for past days.
func (m Model) calCell(cm calMonth, i, cw int) string {
	d := cm.days[i]
	num := m.st.text
	switch {
	case !d.inMonth:
		num = m.st.faint
	case sameDay(d.date, cm.today):
		num = m.st.today
	}
	mark, ms := "", m.st.due
	if d.open > 0 {
		mark = "•"
		if cw >= 6 {
			mark += fmt.Sprint(d.open)
		}
		if d.date.Before(cm.today) {
			ms = m.st.overdue
		}
	} else if len(d.tasks) > 0 {
		mark, ms = "✓", m.st.faint
	}
	bg := lipgloss.NewStyle()
	if i == cm.selected {
		bg = bg.Background(m.st.cursorBg)
		num, ms = num.Background(m.st.cursorBg).Bold(true), ms.Background(m.st.cursorBg)
	}
	n := fmt.Sprintf("%2d", d.date.Day())
	pad := max(cw-1-len(n)-len([]rune(mark)), 0)
	return bg.Render(" ") + num.Render(n) + ms.Render(mark) + bg.Render(strings.Repeat(" ", pad))
}

// calendarHelp is the footer while the calendar is open.
const calendarHelp = "h/l day · j/k week · [ ] month · t today · enter list the day · a add · esc close"

// ---- desktop app ---------------------------------------------------------------

// Calendar is the calendar as data, for katTasks-gui.
type Calendar struct {
	Title    string        `json:"title"`    // "November 2026"
	Weekdays []string      `json:"weekdays"` // "Sun" … "Sat"
	Days     []CalendarDay `json:"days"`     // six weeks, Sunday first
	Selected int           `json:"selected"` // index into Days
	DayTitle string        `json:"dayTitle"` // "Wednesday, November 18"
	Tasks    []TaskRow     `json:"tasks"`    // due on the selected day; Index is the position here
	Due      int           `json:"due"`      // open tasks due this month
}

// CalendarDay is one day of the grid.
type CalendarDay struct {
	Date    string         `json:"date"` // 2026-11-18
	Day     int            `json:"day"`
	InMonth bool           `json:"inMonth"`
	Today   bool           `json:"today"`
	Past    bool           `json:"past"`
	Open    int            `json:"open"`  // open tasks due
	Total   int            `json:"total"` // all tasks due, done ones included
	Items   []CalendarItem `json:"items"`
}

// CalendarItem is a task as a day cell lists it.
type CalendarItem struct {
	Text     string `json:"text"`
	PriStyle string `json:"priStyle"`
	Done     bool   `json:"done"`
}

func (m Model) calendarSnapshot() *Calendar {
	cm := m.calendarMonth()
	c := &Calendar{Title: m.calDay.Format("January 2006"), Selected: cm.selected}
	for i := range 7 {
		c.Weekdays = append(c.Weekdays, time.Weekday(i).String()[:3])
	}
	for _, d := range cm.days {
		cd := CalendarDay{
			Date: d.date.Format(todo.DateLayout), Day: d.date.Day(), InMonth: d.inMonth,
			Today: sameDay(d.date, cm.today), Past: d.date.Before(cm.today), Open: d.open, Total: len(d.tasks),
		}
		for _, t := range d.tasks {
			it := CalendarItem{Text: listview.Body(t), Done: t.Done}
			if t.Priority != 0 {
				it.PriStyle = priKey(t.Priority)
			}
			cd.Items = append(cd.Items, it)
		}
		if d.inMonth {
			c.Due += d.open
		}
		c.Days = append(c.Days, cd)
	}
	sel := cm.days[cm.selected]
	c.DayTitle = sel.date.Format("Monday, January 2")
	hide, now := m.hiddenKeys(), time.Now()
	for i, t := range sel.tasks {
		r := m.taskRow(t, hide, now)
		r.Index = i
		c.Tasks = append(c.Tasks, r)
	}
	return c
}

// calendarPick chooses grid cell i; with open it also lists that day.
func (m *Model) calendarPick(i int, open bool) {
	cm := m.calendarMonth()
	if i < 0 || i >= calCells {
		return
	}
	m.focus, m.calDay = focusList, cm.days[i].date
	if open {
		m.showDay(nil)
	}
}

// calendarTask lists the chosen day with the cursor on its task i; with
// edit it also opens the task for editing.
func (m *Model) calendarTask(i int, edit bool) tea.Cmd {
	cm := m.calendarMonth()
	ts := cm.days[cm.selected].tasks
	if i < 0 || i >= len(ts) {
		return nil
	}
	m.showDay(ts[i])
	if edit {
		return actionByName["edit"].run(m)
	}
	return nil
}
