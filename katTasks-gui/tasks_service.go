package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/ui"
)

// StateEvent carries a new Snapshot when the screen changes by itself, e.g.
// when a chord times out or a copy finishes.
const StateEvent = "kattasks:state"

// TasksService hands the frontend's keys and clicks to the task list and
// returns what to draw. Every exported method becomes a TS binding.
type TasksService struct {
	h       *ui.Headless
	err     string
	cfgPath string
}

// NewTasksService loads the settings and the todo file. A failure is kept
// and shown in the window rather than ending the app, as a desktop app has
// nowhere else to report it.
func NewTasksService(cfgPath, todoFile, version string, app func() *application.App) *TasksService {
	s := &TasksService{cfgPath: cfgPath}
	cfg, created, errs := config.Load(cfgPath)
	if todoFile != "" {
		cfg.SetTodo(todoFile)
	}
	m, err := ui.New(cfg, ui.Options{ConfigPath: cfgPath, TodoFile: todoFile, Errors: errs, Version: version})
	if err != nil {
		s.err = fmt.Sprintf("katTasks couldn't open %s: %v", cfg.Files.Todo, err)
		return s
	}
	s.h = ui.NewHeadless(m, ui.HeadlessOptions{
		Copy: func(text string) error {
			if a := app(); a == nil || !a.Clipboard.SetText(text) {
				return errors.New("the clipboard isn't available")
			}
			return nil
		},
		Changed: func(snap ui.Snapshot) {
			if a := app(); a != nil {
				a.Event.Emit(StateEvent, snap)
			}
		},
		Quit: func() {
			if a := app(); a != nil {
				a.Quit()
			}
		},
	})
	if created {
		s.h.SetStatus("wrote default settings to " + cfgPath)
	}
	return s
}

// StartupError is why the task list couldn't be opened, or "".
func (s *TasksService) StartupError() string { return s.err }

func (s *TasksService) ok() bool { return s.h != nil }

// State returns the screen.
func (s *TasksService) State() ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.State()
}

// Resize tells the list how many characters and task rows fit.
func (s *TasksService) Resize(cols, rows int) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Resize(cols, rows)
}

// Key presses a key, in Bubble Tea's naming ("j", "ctrl+d", "enter", " ").
func (s *TasksService) Key(name string) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Key(name)
}

// Paste types text into the input being edited.
func (s *TasksService) Paste(text string) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Paste(text)
}

// ClickTask selects a task; open edits it, toggle marks it done or open.
func (s *TasksService) ClickTask(index int, open, toggle bool) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.ClickTask(index, open, toggle)
}

// ClickCalendarTask lists the calendar's day with the cursor on its task;
// open also edits it.
func (s *TasksService) ClickCalendarTask(index int, open bool) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.ClickCalendarTask(index, open)
}

// AgendaWeek moves the agenda by n weeks.
func (s *TasksService) AgendaWeek(n int) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.AgendaWeek(n)
}

// CalendarMonth moves the calendar by n months.
func (s *TasksService) CalendarMonth(n int) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.CalendarMonth(n)
}

// ClickSidebar shows a sidebar entry.
func (s *TasksService) ClickSidebar(index int) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.ClickSidebar(index)
}

// Scroll scrolls the list, or the sidebar, by n rows.
func (s *TasksService) Scroll(n int, sidebar bool) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Scroll(n, sidebar)
}

// Run runs a named action, as from the command palette.
func (s *TasksService) Run(action string) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Run(action)
}

// Pick chooses a palette entry, theme or completion; commit runs it.
func (s *TasksService) Pick(index int, commit bool) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.Pick(index, commit)
}

// SetInputCursor moves the text cursor of the input being edited.
func (s *TasksService) SetInputCursor(pos int) ui.Snapshot {
	if !s.ok() {
		return ui.Snapshot{}
	}
	return s.h.SetInputCursor(pos)
}

// ConfigPath is the settings file.
func (s *TasksService) ConfigPath() string { return strings.TrimSpace(s.cfgPath) }
