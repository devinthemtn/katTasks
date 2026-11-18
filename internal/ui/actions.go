package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// action is one thing a key, chord or the command palette can do. Its name
// is what [keys] in config.toml uses.
type action struct {
	name, desc string
	group      string   // help screen column
	defaults   []string // default keys; "yy"-style entries are chords
	needsTask  bool     // does nothing without a task under the cursor
	run        func(m *Model) tea.Cmd
}

const (
	groupMove = "MOVE"
	groupEdit = "EDIT"
	groupView = "VIEW"
)

// actions lists every action in help-screen order. It is filled in init
// because some actions reach buildKeymap, which reads it.
var (
	actions      []*action
	actionByName map[string]*action
)

func init() {
	actions = defineActions()
	actionByName = map[string]*action{}
	for _, a := range actions {
		actionByName[a.name] = a
	}
}

func defineActions() []*action {
	return []*action{
		{name: "down", desc: "down", group: groupMove, defaults: []string{"j", "down"}, run: func(m *Model) tea.Cmd { m.move(moveDown); return nil }},
		{name: "up", desc: "up", group: groupMove, defaults: []string{"k", "up"}, run: func(m *Model) tea.Cmd { m.move(moveUp); return nil }},
		{name: "top", desc: "top", group: groupMove, defaults: []string{"g", "home"}, run: func(m *Model) tea.Cmd { m.move(moveTop); return nil }},
		{name: "bottom", desc: "bottom", group: groupMove, defaults: []string{"G", "end"}, run: func(m *Model) tea.Cmd { m.move(moveBottom); return nil }},
		{name: "half_page_down", desc: "half page down", group: groupMove, defaults: []string{"ctrl+d", "pgdown"}, run: func(m *Model) tea.Cmd { m.move(moveHalfDown); return nil }},
		{name: "half_page_up", desc: "half page up", group: groupMove, defaults: []string{"ctrl+u", "pgup"}, run: func(m *Model) tea.Cmd { m.move(moveHalfUp); return nil }},
		{name: "switch_pane", desc: "switch pane", group: groupMove, defaults: []string{"tab", "shift+tab"}, run: func(m *Model) tea.Cmd {
			if m.focus == focusList {
				m.focus = focusSidebar
			} else {
				m.focus = focusList
			}
			return nil
		}},
		{name: "focus_sidebar", desc: "go to sidebar", group: groupMove, defaults: []string{"h", "left"}, run: func(m *Model) tea.Cmd { m.focus = focusSidebar; return nil }},
		{name: "focus_tasks", desc: "go to tasks", group: groupMove, defaults: []string{"l", "right"}, run: func(m *Model) tea.Cmd { m.focus = focusList; return nil }},

		{name: "add", desc: "add task", group: groupEdit, defaults: []string{"a", "n"}, run: func(m *Model) tea.Cmd { m.startAdd(); return nil }},
		{name: "edit", desc: "edit task", group: groupEdit, defaults: []string{"e", "enter"}, run: func(m *Model) tea.Cmd {
			if m.focus == focusSidebar {
				m.focus = focusList
				return nil
			}
			if t := m.selected(); t != nil {
				m.mode, m.editing = modeEdit, t
				m.startInput(t.String(), -1)
			}
			return nil
		}},
		{name: "toggle_done", desc: "toggle done", group: groupEdit, defaults: []string{"x", "space"}, needsTask: true, run: func(m *Model) tea.Cmd { m.toggleTargets(); return nil }},
		{name: "delete", desc: "delete task", group: groupEdit, defaults: []string{"d"}, needsTask: true, run: func(m *Model) tea.Cmd { m.deleteTargets(); return nil }},
		{name: "set_priority", desc: "set priority (then A–Z)", group: groupEdit, defaults: []string{"p"}, needsTask: true, run: func(m *Model) tea.Cmd {
			if m.selected() != nil {
				m.mode = modePriority
			}
			return nil
		}},
		{name: "priority_up", desc: "raise priority", group: groupEdit, defaults: []string{"+", "="}, needsTask: true, run: func(m *Model) tea.Cmd { m.bumpPriority(true); return nil }},
		{name: "priority_down", desc: "lower priority", group: groupEdit, defaults: []string{"-"}, needsTask: true, run: func(m *Model) tea.Cmd { m.bumpPriority(false); return nil }},
		{name: "move_down", desc: "move task down", group: groupEdit, defaults: []string{"J"}, needsTask: true, run: func(m *Model) tea.Cmd { m.moveTask(1); return nil }},
		{name: "move_up", desc: "move task up", group: groupEdit, defaults: []string{"K"}, needsTask: true, run: func(m *Model) tea.Cmd { m.moveTask(-1); return nil }},
		{name: "visual", desc: "select several (space marks)", group: groupEdit, defaults: []string{"v"}, run: func(m *Model) tea.Cmd { m.toggleVisual(); return nil }},
		{name: "select_all", desc: "select all shown", group: groupEdit, defaults: []string{"V"}, run: func(m *Model) tea.Cmd { m.selectAll(); return nil }},
		{name: "copy_line", desc: "copy line", group: groupEdit, defaults: []string{"yy"}, needsTask: true, run: func(m *Model) tea.Cmd { return m.copyTargets(false) }},
		{name: "copy_body", desc: "copy text only", group: groupEdit, defaults: []string{"yb"}, needsTask: true, run: func(m *Model) tea.Cmd { return m.copyTargets(true) }},
		{name: "undo", desc: "undo", group: groupEdit, defaults: []string{"u"}, run: func(m *Model) tea.Cmd { m.undoLast(); return nil }},
		{name: "archive_done", desc: "archive done to done.txt", group: groupEdit, defaults: []string{"A"}, run: func(m *Model) tea.Cmd { m.askArchive(); return nil }},

		{name: "filter", desc: "filter / search", group: groupView, defaults: []string{"/"}, run: func(m *Model) tea.Cmd {
			m.mode = modeFilter
			m.startInput(m.filter, -1)
			return nil
		}},
		{name: "clear_filter", desc: "clear filter / selection", group: groupView, defaults: []string{"esc"}, run: func(m *Model) tea.Cmd {
			if m.visual {
				m.endVisual()
			} else if m.filter != "" {
				m.filter = ""
				m.refresh(nil)
			} else if m.sideCursor != 0 {
				m.sideCursor = 0
				m.refresh(nil)
			}
			return nil
		}},
		{name: "cycle_sort", desc: "cycle sort", group: groupView, defaults: []string{"s"}, run: func(m *Model) tea.Cmd {
			m.sortIdx = (m.sortIdx + 1) % len(sortModes)
			m.refresh(nil)
			m.setStatus("sorted by %s", sortModes[m.sortIdx])
			return nil
		}},
		{name: "toggle_show_done", desc: "hide / show done", group: groupView, defaults: []string{"H"}, run: func(m *Model) tea.Cmd {
			m.hideDone = !m.hideDone
			m.refresh(nil)
			if m.hideDone {
				m.setStatus("hiding completed tasks")
			} else {
				m.setStatus("showing completed tasks")
			}
			return nil
		}},
		{name: "toggle_line_numbers", desc: "line numbers", group: groupView, defaults: []string{"L"}, run: func(m *Model) tea.Cmd { m.lineNums = !m.lineNums; return nil }},
		{name: "theme_picker", desc: "pick theme", group: groupView, defaults: []string{"T"}, run: func(m *Model) tea.Cmd { m.openThemePicker(); return nil }},
		{name: "cycle_theme", desc: "next theme", group: groupView, run: func(m *Model) tea.Cmd { m.cycleTheme(); return nil }},
		{name: "reload", desc: "reload todo.txt", group: groupView, defaults: []string{"r"}, run: func(m *Model) tea.Cmd {
			if err := m.load(); err != nil {
				m.setError("reload failed: %v", err)
			} else {
				m.undo = nil
				m.setStatus("reloaded %s (%d tasks)", m.cfg.Files.Todo, len(m.tasks))
			}
			return nil
		}},
		{name: "reload_config", desc: "reload settings", group: groupView, defaults: []string{"ctrl+r"}, run: func(m *Model) tea.Cmd { m.reloadConfig(); return nil }},
		{name: "help", desc: "help", group: groupView, defaults: []string{"?"}, run: func(m *Model) tea.Cmd { m.showHelp = true; return nil }},
		{name: "quit", desc: "quit", group: groupView, defaults: []string{"q"}, run: func(m *Model) tea.Cmd { return tea.Quit }},
	}
}

// shortHelpActions are shown in the footer.
var shortHelpActions = []string{"add", "edit", "toggle_done", "set_priority", "filter", "cycle_sort", "switch_pane", "help", "quit"}

func (m *Model) askArchive() {
	var done []*todo.Task
	for _, t := range m.tasks {
		if t.Done {
			done = append(done, t)
		}
	}
	if len(done) == 0 {
		m.setStatus("no completed tasks to archive")
		return
	}
	m.ask(fmt.Sprintf("Move %d completed task(s) to %s? This can't be undone. (y/n)", len(done), m.cfg.Files.Done),
		func(m *Model) { m.archive(done) })
}
