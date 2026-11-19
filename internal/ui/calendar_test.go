package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/todo"
)

func calendarModel(t *testing.T, lines ...string) (Model, time.Time) {
	t.Helper()
	m := scriptModel(t, nil)
	os.WriteFile(m.cfg.Files.Todo, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	if err := m.load(); err != nil {
		t.Fatal(err)
	}
	return m, listview.Today(time.Now())
}

func keyPress(m Model, ks ...string) Model {
	for _, k := range ks {
		msg, _ := ParseKeyMsg(k)
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

func day(d time.Time) string { return d.Format(todo.DateLayout) }

func TestCalendarMovesAndListsDay(t *testing.T) {
	today := listview.Today(time.Now())
	tomorrow := today.AddDate(0, 0, 1)
	m, _ := calendarModel(t, "Pay rent due:"+day(tomorrow), "Gym due:"+day(tomorrow), "No date")

	m = keyPress(m, "c")
	if !m.calendar || !sameDay(m.calDay, today) {
		t.Fatalf("c: calendar %v day %v", m.calendar, m.calDay)
	}
	m = keyPress(m, "l")
	if !sameDay(m.calDay, tomorrow) {
		t.Fatalf("l: day %v", m.calDay)
	}
	s := m.Snapshot()
	if s.Calendar == nil || len(s.Calendar.Tasks) != 2 || s.Calendar.Days[s.Calendar.Selected].Open != 2 {
		t.Fatalf("snapshot calendar %+v", s.Calendar)
	}
	if !strings.Contains(m.View(), "•2") {
		t.Errorf("view doesn't show the count:\n%s", m.View())
	}

	m = keyPress(m, "j", "k", "]", "[")
	if !sameDay(m.calDay, tomorrow) {
		t.Fatalf("week and month round trip: day %v", m.calDay)
	}
	m = keyPress(m, "x") // needs a task: refused, not applied to the hidden list
	if m.statusErr || !strings.Contains(m.status, "enter") {
		t.Fatalf("x in calendar: %q", m.status)
	}

	m = keyPress(m, "enter")
	if m.calendar || m.filter != "due:"+day(tomorrow) || len(m.visible) != 2 {
		t.Fatalf("enter: calendar %v filter %q visible %d", m.calendar, m.filter, len(m.visible))
	}
}

func TestCalendarAddPrefillsDue(t *testing.T) {
	m, today := calendarModel(t, "Old task")
	m = keyPress(m, "c", "l", "a")
	want := "due:" + day(today.AddDate(0, 0, 1))
	if m.mode != modeAdd || strings.TrimSpace(m.input.Value()) != want || m.input.Position() != 0 {
		t.Fatalf("add: mode %v value %q pos %d", m.mode, m.input.Value(), m.input.Position())
	}
	m = typeText(m, "Dentist")
	m = press(m, tea.KeyEnter)
	if !m.calendar || m.mode != modeNormal {
		t.Fatalf("after adding: calendar %v mode %v", m.calendar, m.mode)
	}
	data, _ := os.ReadFile(m.cfg.Files.Todo)
	if !strings.Contains(string(data), "Dentist "+want) {
		t.Fatalf("saved:\n%s", data)
	}
}

func TestCalendarFollowsKeyOverrides(t *testing.T) {
	m, today := calendarModel(t, "Old task")
	km, errs := buildKeymap(map[string][]string{"down": {"ctrl+n"}})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	m.km = km
	m = keyPress(m, "c", "ctrl+n")
	if !sameDay(m.calDay, today.AddDate(0, 0, 7)) {
		t.Fatalf("rebound down: day %v", m.calDay)
	}
}

func TestAddMonthsClampsDay(t *testing.T) {
	jan31 := time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local)
	if got := addMonths(jan31, 1); got.Month() != time.February || got.Day() != 28 {
		t.Errorf("Jan 31 + 1 month = %v", got)
	}
	if got := addMonths(jan31, -2); got.Month() != time.November || got.Day() != 30 || got.Year() != 2025 {
		t.Errorf("Jan 31 - 2 months = %v", got)
	}
}

func TestHeadlessCalendarClicks(t *testing.T) {
	h, _ := headless(t)
	today := listview.Today(time.Now())
	keys(h, "a")
	for _, r := range "Dentist due:" + day(today) {
		h.Key(string(r))
	}
	h.Key("enter")
	s := h.Run("calendar")
	if s.Calendar == nil || !s.Calendar.Days[s.Calendar.Selected].Today {
		t.Fatalf("calendar %+v", s.Calendar)
	}
	s = h.ClickCalendarTask(0, false)
	if s.Calendar != nil || s.Total != 1 || !s.Rows[0].Cursor {
		t.Fatalf("task click: calendar %v total %d", s.Calendar != nil, s.Total)
	}
	h.Key("esc")
	s = h.Run("calendar")
	s = h.Pick(s.Calendar.Selected+1, false)
	if s.Calendar.Days[s.Calendar.Selected].Today {
		t.Fatal("day click didn't move the selection")
	}
}
