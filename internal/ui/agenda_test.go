package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/listview"
)

func TestAgendaOrdersWeekByDayAndTime(t *testing.T) {
	ws := weekStart(time.Now())
	mon, wed := ws.AddDate(0, 0, 1), ws.AddDate(0, 0, 3)
	m, _ := calendarModel(t,
		"Wed lunch at:12:30 due:"+day(wed),
		"(A) Wed all day due:"+day(wed),
		"Wed standup 9:15 due:"+day(wed),
		"Mon call 3pm due:"+day(mon),
		"Next week due:"+day(ws.AddDate(0, 0, 9)),
		"No date",
	)
	m = keyPress(m, "w")
	if !m.agenda {
		t.Fatal("w didn't open the agenda")
	}
	var got []string
	for _, tk := range m.visible {
		got = append(got, listview.Body(tk))
	}
	want := []string{"Mon call 3pm", "Wed all day", "Wed standup 9:15", "Wed lunch"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("agenda order\n got %q\nwant %q", got, want)
	}

	view := m.View()
	for _, s := range []string{"15:00", "9:15", "12:30", "—"} {
		if !strings.Contains(view, s) {
			t.Errorf("view lacks %q:\n%s", s, view)
		}
	}

	s := m.Snapshot()
	a := s.Agenda
	if a == nil || len(a.Days) != 7 {
		t.Fatalf("snapshot agenda %+v", a)
	}
	w := a.Days[3]
	if len(w.AllDay) != 1 || len(w.Timed) != 2 || w.Timed[0].Time != "9:15" || w.Timed[1].Minutes != 12*60+30 {
		t.Fatalf("wednesday %+v", w)
	}
	if a.FromHour > 9 || a.ToHour < 16 {
		t.Fatalf("hours %d–%d", a.FromHour, a.ToHour)
	}

	// It's the list: x completes the task under the cursor.
	m.cursor = 1
	m = keyPress(m, "x")
	data, _ := os.ReadFile(m.cfg.Files.Todo)
	if !strings.Contains(string(data), "x ") || !strings.Contains(string(data), "Wed all day") {
		t.Fatalf("x in agenda didn't complete:\n%s", data)
	}

	m = keyPress(m, "l")
	if len(m.visible) != 1 || listview.Body(m.visible[0]) != "Next week" {
		t.Fatalf("next week shows %d tasks", len(m.visible))
	}
	m = keyPress(m, "esc")
	if m.agenda || len(m.visible) < 4 {
		t.Fatalf("esc: agenda %v, %d tasks", m.agenda, len(m.visible))
	}
}

func TestAgendaAddAndCalendarSwitch(t *testing.T) {
	ws := weekStart(time.Now())
	thu := ws.AddDate(0, 0, 4)
	m, _ := calendarModel(t, "Thursday thing due:"+day(thu))
	m = keyPress(m, "w", "a")
	if m.mode != modeAdd || strings.TrimSpace(m.input.Value()) != "due:"+day(thu) {
		t.Fatalf("add: mode %v value %q", m.mode, m.input.Value())
	}
	m = keyPress(m, "esc", "c")
	if m.agenda || !m.calendar || !sameDay(m.calDay, thu) {
		t.Fatalf("c: agenda %v calendar %v day %v", m.agenda, m.calendar, m.calDay)
	}
	m = keyPress(m, "w")
	if !m.agenda || m.calendar || len(m.visible) != 1 {
		t.Fatalf("w from calendar: agenda %v calendar %v visible %d", m.agenda, m.calendar, len(m.visible))
	}
}

func TestWeekTitle(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-15": "Nov 15 – 21, 2026",
		"2026-11-29": "Nov 29 – Dec 5, 2026",
		"2026-12-27": "Dec 27, 2026 – Jan 2, 2027",
	} {
		d, _ := time.ParseInLocation("2006-01-02", in, time.Local)
		if got := weekTitle(d); got != want {
			t.Errorf("weekTitle(%s) = %q, want %q", in, got, want)
		}
	}
}
