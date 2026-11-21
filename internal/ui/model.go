// Package ui implements the katTasks terminal interface.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/script"
	"github.com/devinthemtn/katTasks/internal/theme"
	"github.com/devinthemtn/katTasks/internal/todo"
)

type focus int

const (
	focusList focus = iota
	focusSidebar
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeEdit
	modeFilter
	modePriority
	modeConfirm
	modeTheme
	modePalette
	modeSaveName
)

var sortModes = config.SortModes

// sideEntry is one sidebar row; see listview.
type sideEntry = listview.Entry

const (
	entryAll     = listview.All
	entryHeader  = listview.Header
	entryProject = listview.Project
	entryContext = listview.Context
	entrySaved   = listview.Saved
)

const maxUndo = 50

// Model is the Bubble Tea model.
type Model struct {
	opts    Options
	cfg     config.Config
	tasks   []*todo.Task
	modTime time.Time
	undo    [][]*todo.Task

	visible []*todo.Task
	cursor  int
	offset  int

	side       []sideEntry
	sideCursor int
	sideOffset int
	known      []string // +projects and @contexts, most used first

	focus    focus
	mode     mode
	visual   bool                // visual (multi-select) mode
	sel      map[*todo.Task]bool // tasks selected in visual mode
	lineNums bool
	sortIdx  int
	hideDone bool
	filter   string
	q        query.Query // parsed filter

	input     textinput.Model
	editing   *todo.Task
	sugg      []string
	pick      *alarmPick    // the alarm: date and time picker, over the task popup
	choice    *scriptChoice // a script's kat.choose list, over the task popup
	suggIdx   int
	confirm   string
	confirmNo bool // the question's "no" button is highlighted
	onConfirm func(*Model)

	status     string
	statusErr  bool
	configWarn string // settings problems, shown until fixed and reloaded

	st         styles
	themes     []theme.Theme
	theme      string          // name of the theme in use
	pickIdx    int             // theme picker cursor
	pickPrev   string          // theme to restore if the picker is cancelled
	scripts    []script.Script // Lua scripts in the scripts folder
	palItems   []paletteItem
	palShown   []int // indexes into palItems that match the query, best first
	palIdx     int
	km         keymap
	pending    string // first key of a chord, waiting for the second
	chordSeq   int
	showHelp   bool
	helpScroll int       // the help screen's first line, when it doesn't all fit
	calendar   bool      // the calendar is shown instead of the task list
	agenda     bool      // the list shows one week, by day (see agenda.go)
	calDay     time.Time // the day chosen in the calendar

	width, height int

	copyFn func(string) error // the desktop app's clipboard; nil in a terminal
	// windowed: run by katTasks-gui, with no terminal to hand an editor
	windowed bool
}

// Options configure New.
type Options struct {
	ConfigPath string  // settings file, re-read by Ctrl-r
	TodoFile   string  // -file override; empty uses files.todo
	Errors     []error // problems found while loading the settings
	Version    string  // shown on the help screen
}

// New builds the model and loads the todo file.
func New(cfg config.Config, opts Options) (Model, error) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 0

	m := Model{
		opts:  opts,
		input: ti,
	}
	errs := append(opts.Errors, m.applyConfig(cfg)...)
	m.hideDone = cfg.View.HideCompleted
	m.lineNums = cfg.View.LineNumbers
	m.showConfigErrors(errs)
	if err := m.load(); err != nil {
		return m, err
	}
	return m, nil
}

// applyConfig installs settings, from startup or a Ctrl-r reload, and
// returns problems with the key bindings.
func (m *Model) applyConfig(cfg config.Config) []error {
	if m.opts.TodoFile != "" {
		cfg.SetTodo(m.opts.TodoFile)
	}
	m.cfg = cfg
	for i, s := range sortModes {
		if s == cfg.View.Sort {
			m.sortIdx = i
		}
	}
	km, errs := buildKeymap(cfg.Keys)
	m.km = km

	themes, warns := theme.Load(cfg.Dir())
	errs = append(errs, warns...)
	m.themes = themes
	if !m.useTheme(cfg.Theme) {
		errs = append(errs, fmt.Errorf("theme = %q: no such theme (built-in: default, terminal, nord, gruvbox, dracula; or a file in %s)", cfg.Theme, theme.Dir(cfg.Dir())))
		m.useTheme("default")
	}
	m.loadScripts()
	return errs
}

// useTheme switches to the named theme, reporting whether it exists.
func (m *Model) useTheme(name string) bool {
	t, ok := theme.Find(m.themes, name)
	if !ok {
		return false
	}
	m.theme, m.st = name, newStyles(t)
	return true
}

func (m *Model) showConfigErrors(errs []error) {
	m.configWarn = ""
	if len(errs) == 0 {
		return
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	m.configWarn = strings.Join(msgs, "; ")
}

func (m *Model) load() error {
	tasks, mt, err := todo.Load(m.cfg.Files.Todo)
	if err != nil {
		return err
	}
	m.tasks, m.modTime = tasks, mt
	m.endVisual()
	m.refresh(nil)
	return nil
}

// reloadConfig re-reads the settings file, and the todo file if its path
// changed.
func (m *Model) reloadConfig() {
	cfg, _, errs := config.Load(m.opts.ConfigPath)
	oldTodo := m.cfg.Files.Todo
	errs = append(errs, m.applyConfig(cfg)...)
	if m.cfg.Files.Todo != oldTodo {
		m.undo = nil
		if err := m.load(); err != nil {
			errs = append(errs, err)
		}
	} else {
		m.refresh(nil)
	}
	if len(errs) > 0 {
		m.showConfigErrors(errs)
		return
	}
	m.setStatus("settings reloaded from %s", m.opts.ConfigPath)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// ---- state helpers ---------------------------------------------------------

func (m *Model) setStatus(format string, a ...any) {
	m.status, m.statusErr = fmt.Sprintf(format, a...), false
}

func (m *Model) setError(format string, a ...any) {
	m.status, m.statusErr = fmt.Sprintf(format, a...), true
}

func (m *Model) selected() *todo.Task {
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		return m.visible[m.cursor]
	}
	return nil
}

func (m *Model) currentEntry() sideEntry {
	if m.sideCursor >= 0 && m.sideCursor < len(m.side) {
		return m.side[m.sideCursor]
	}
	return sideEntry{Kind: entryAll}
}

// refresh rebuilds the sidebar and visible list. If keep is non-nil the
// cursor follows that task.
func (m *Model) refresh(keep *todo.Task) {
	if keep == nil {
		keep = m.selected()
	}
	m.buildSidebar()

	m.q = query.Parse(m.filter)
	m.visible = listview.Visible(m.tasks, m.currentEntry(), m.q, m.hideDone, sortModes[m.sortIdx], time.Now())
	if m.agenda {
		m.agendaFilter()
	}

	if keep != nil {
		for i, t := range m.visible {
			if t == keep {
				m.cursor = i
				break
			}
		}
	}
	m.clampCursor()
}

func (m *Model) buildSidebar() {
	prev := m.currentEntry()
	m.side, m.known = listview.Sidebar(m.tasks, m.cfg.Filters, time.Now())
	// Keep the same sidebar selection if it still exists.
	m.sideCursor = listview.Find(m.side, prev.Kind, prev.Value)
}

func (m *Model) listRows() int {
	r := m.bodyHeight() - 2 // pane border
	if r < 1 {
		r = 1
	}
	return r
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	rows := m.listRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if max := len(m.visible) - rows; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}

	if m.sideCursor < m.sideOffset {
		m.sideOffset = m.sideCursor
	}
	if m.sideCursor >= m.sideOffset+rows {
		m.sideOffset = m.sideCursor - rows + 1
	}
	if m.sideOffset < 0 {
		m.sideOffset = 0
	}
}
