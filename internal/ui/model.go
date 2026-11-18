// Package ui implements the katTasks terminal interface.
package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
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
)

var sortModes = config.SortModes

type entryKind int

const (
	entryAll entryKind = iota
	entryDue
	entryHeader
	entryProject
	entryContext
)

type sideEntry struct {
	kind  entryKind
	value string
	count int
}

func (e sideEntry) label() string {
	switch e.kind {
	case entryAll:
		return "All tasks"
	case entryDue:
		return "Has due date"
	default:
		return e.value
	}
}

func (e sideEntry) match(t *todo.Task) bool {
	switch e.kind {
	case entryDue:
		_, ok := t.Due()
		return ok
	case entryProject, entryContext:
		return t.Has(e.value)
	}
	return true
}

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

	input     textinput.Model
	editing   *todo.Task
	sugg      []string
	suggIdx   int
	confirm   string
	onConfirm func(*Model)

	status     string
	statusErr  bool
	configWarn string // settings problems, shown until fixed and reloaded

	st       styles
	themes   []theme.Theme
	theme    string // name of the theme in use
	pickIdx  int    // theme picker cursor
	pickPrev string // theme to restore if the picker is cancelled
	km       keymap
	pending  string // first key of a chord, waiting for the second
	chordSeq int
	showHelp bool

	width, height int
}

// Options configure New.
type Options struct {
	ConfigPath string  // settings file, re-read by Ctrl-r
	TodoFile   string  // -file override; empty uses files.todo
	Errors     []error // problems found while loading the settings
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
		errs = append(errs, fmt.Errorf("theme = %q: no such theme (built-in: default, terminal, nord, gruvbox; or a file in %s)", cfg.Theme, theme.Dir(cfg.Dir())))
		m.useTheme("default")
	}
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
	return sideEntry{kind: entryAll}
}

// refresh rebuilds the sidebar and visible list. If keep is non-nil the
// cursor follows that task.
func (m *Model) refresh(keep *todo.Task) {
	if keep == nil {
		keep = m.selected()
	}
	m.buildSidebar()

	entry := m.currentEntry()
	needle := strings.ToLower(m.filter)
	m.visible = m.visible[:0]
	for _, t := range m.tasks {
		if m.hideDone && t.Done {
			continue
		}
		if !entry.match(t) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(t.String()), needle) {
			continue
		}
		m.visible = append(m.visible, t)
	}
	m.sortVisible()

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
	projects, contexts := map[string]int{}, map[string]int{}
	used := map[string]int{}
	due := 0
	pending := 0
	for _, t := range m.tasks {
		for _, p := range t.Projects() {
			used[p]++
			if _, ok := projects[p]; !ok {
				projects[p] = 0
			}
			if !t.Done {
				projects[p]++
			}
		}
		for _, c := range t.Contexts() {
			used[c]++
			if _, ok := contexts[c]; !ok {
				contexts[c] = 0
			}
			if !t.Done {
				contexts[c]++
			}
		}
		if t.Done {
			continue
		}
		pending++
		if _, ok := t.Due(); ok {
			due++
		}
	}

	m.side = []sideEntry{{kind: entryAll, count: pending}, {kind: entryDue, count: due}}
	add := func(title string, kind entryKind, set map[string]int) {
		if len(set) == 0 {
			return
		}
		names := make([]string, 0, len(set))
		for n := range set {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
		m.side = append(m.side, sideEntry{kind: entryHeader, value: title})
		for _, n := range names {
			m.side = append(m.side, sideEntry{kind: kind, value: n, count: set[n]})
		}
	}
	add("PROJECTS", entryProject, projects)
	add("CONTEXTS", entryContext, contexts)

	m.known = m.known[:0]
	for w := range used {
		m.known = append(m.known, w)
	}
	sort.Slice(m.known, func(i, j int) bool {
		if used[m.known[i]] != used[m.known[j]] {
			return used[m.known[i]] > used[m.known[j]]
		}
		return m.known[i] < m.known[j]
	})

	// Keep the same sidebar selection if it still exists.
	m.sideCursor = 0
	for i, e := range m.side {
		if e.kind == prev.kind && e.value == prev.value {
			m.sideCursor = i
			break
		}
	}
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

// ---- persistence -----------------------------------------------------------

func cloneAll(ts []*todo.Task) []*todo.Task {
	out := make([]*todo.Task, len(ts))
	for i, t := range ts {
		out[i] = t.Clone()
	}
	return out
}

// stale reports whether todo.txt was modified by another program since we
// last read or wrote it.
func (m *Model) stale() bool {
	return !todo.ModTime(m.cfg.Files.Todo).Equal(m.modTime)
}

// mutate applies fn to the task list and saves, refusing if the file has
// changed on disk so external edits are never clobbered.
func (m *Model) mutate(fn func()) bool {
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return false
	}
	m.undo = append(m.undo, cloneAll(m.tasks))
	if len(m.undo) > maxUndo {
		m.undo = m.undo[1:]
	}
	fn()
	return m.save()
}

func (m *Model) save() bool {
	mt, err := todo.Save(m.cfg.Files.Todo, m.tasks)
	if err != nil {
		m.setError("save failed: %v", err)
		return false
	}
	m.modTime = mt
	return true
}

func (m *Model) removeTasks(rm map[*todo.Task]bool) {
	out := m.tasks[:0]
	for _, t := range m.tasks {
		if !rm[t] {
			out = append(out, t)
		}
	}
	m.tasks = out
}

// archive moves the given tasks to done.txt. Archiving touches two files,
// so it clears the undo history.
func (m *Model) archive(ts []*todo.Task) {
	if len(ts) == 0 {
		m.setStatus("nothing to archive")
		return
	}
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return
	}
	if err := todo.Append(m.cfg.Files.Done, ts); err != nil {
		m.setError("archive failed: %v", err)
		return
	}
	rm := map[*todo.Task]bool{}
	for _, t := range ts {
		rm[t] = true
	}
	m.removeTasks(rm)
	if m.save() {
		m.undo = nil
		m.setStatus("archived %d task(s) to %s", len(ts), m.cfg.Files.Done)
	}
	m.refresh(nil)
}

// ---- update ----------------------------------------------------------------

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = msg.Width - 12
		m.clampCursor()
		return m, nil
	case chordTimeoutMsg:
		return m, m.chordTimedOut(msg)
	case clipMsg:
		m.copied(msg)
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		typing := m.mode == modeAdd || m.mode == modeEdit || m.mode == modeFilter
		// Keys typed quickly can arrive as one multi-rune message; outside
		// text entry handle them one at a time, so e.g. "pB" sets priority B.
		if !typing && msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Paste {
			var cmds []tea.Cmd
			var mdl tea.Model = m
			for _, r := range msg.Runes {
				var cmd tea.Cmd
				mdl, cmd = mdl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				cmds = append(cmds, cmd)
			}
			return mdl, tea.Batch(cmds...)
		}
		switch m.mode {
		case modeAdd, modeEdit, modeFilter:
			return m.updateInput(msg)
		case modePriority:
			m.updatePriority(msg)
			return m, nil
		case modeTheme:
			m.updateThemePicker(msg)
			return m, nil
		case modeConfirm:
			if s := strings.ToLower(msg.String()); s == "y" {
				m.mode = modeNormal
				m.onConfirm(&m)
			} else {
				m.mode = modeNormal
				m.setStatus("cancelled")
			}
			return m, nil
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

// chordTimeoutMsg ends the wait for a chord's second key.
type chordTimeoutMsg struct{ seq int }

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	k := msg.String()

	if m.showHelp {
		m.showHelp = false
		if a := m.km.single[k]; a != nil && a.name == "quit" {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.pending != "" {
		combo := m.pending + k
		m.pending = ""
		if a := m.km.chords[combo]; a != nil {
			return m, a.run(&m)
		}
		// Not a chord: handle the key on its own.
	}
	if m.visual && k == " " {
		m.toggleSelected()
		return m, nil
	}
	if m.km.prefixes[k] {
		m.pending = k
		m.chordSeq++
		seq := m.chordSeq
		return m, tea.Tick(chordTimeoutMS*time.Millisecond, func(time.Time) tea.Msg { return chordTimeoutMsg{seq} })
	}
	if a := m.km.single[k]; a != nil {
		return m, a.run(&m)
	}
	return m, nil
}

// chordTimedOut runs the single-key action, if any, of a chord prefix that
// was not followed by a second key in time.
func (m *Model) chordTimedOut(msg chordTimeoutMsg) tea.Cmd {
	if msg.seq != m.chordSeq || m.pending == "" {
		return nil
	}
	k := m.pending
	m.pending = ""
	if a := m.km.single[k]; a != nil {
		return a.run(m)
	}
	return nil
}

type moveKind int

const (
	moveDown moveKind = iota
	moveUp
	moveTop
	moveBottom
	moveHalfDown
	moveHalfUp
)

func (m *Model) move(how moveKind) {
	half := m.listRows() / 2
	if m.focus == focusSidebar {
		c := m.sideCursor
		dir := 1
		switch how {
		case moveUp:
			c, dir = c-1, -1
		case moveDown:
			c++
		case moveTop:
			c = 0
		case moveBottom:
			c, dir = len(m.side)-1, -1
		case moveHalfUp:
			c, dir = c-half, -1
		case moveHalfDown:
			c += half
		}
		c = clamp(c, 0, len(m.side)-1)
		for c >= 0 && c < len(m.side) && m.side[c].kind == entryHeader {
			c += dir
		}
		if c >= 0 && c < len(m.side) {
			m.sideCursor = c
		}
		m.cursor, m.offset = 0, 0
		m.refresh(nil)
		m.cursor = 0
		m.clampCursor()
		return
	}
	switch how {
	case moveUp:
		m.cursor--
	case moveDown:
		m.cursor++
	case moveTop:
		m.cursor = 0
	case moveBottom:
		m.cursor = len(m.visible) - 1
	case moveHalfUp:
		m.cursor -= half
	case moveHalfDown:
		m.cursor += half
	}
	m.clampCursor()
}

func (m *Model) ask(prompt string, fn func(*Model)) {
	m.mode, m.confirm, m.onConfirm = modeConfirm, prompt, fn
}

func (m *Model) startAdd() {
	m.mode, m.editing = modeAdd, nil
	// Pre-fill the project/context selected in the sidebar.
	if e := m.currentEntry(); e.kind == entryProject || e.kind == entryContext {
		m.startInput(" "+e.value, 0)
		return
	}
	m.startInput("", -1)
}

func (m *Model) startInput(val string, cursor int) {
	m.input.SetValue(val)
	if cursor < 0 {
		m.input.CursorEnd()
	} else {
		m.input.SetCursor(cursor)
	}
	m.input.Focus()
	m.updateSuggestions()
}

func (m *Model) undoLast() {
	if len(m.undo) == 0 {
		m.setStatus("nothing to undo")
		return
	}
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return
	}
	idx := m.cursor
	m.endVisual()
	m.tasks = m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	if m.save() {
		m.setStatus("undone (%d more)", len(m.undo))
	}
	m.refresh(nil)
	m.cursor = idx
	m.clampCursor()
}

// ---- text input & autocomplete --------------------------------------------

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.mode == modeFilter {
			m.filter = ""
			m.refresh(nil)
		}
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		m.submitInput()
		return m, nil
	case "tab":
		if len(m.sugg) > 0 {
			m.acceptSuggestion()
			return m, nil
		}
	case "down", "ctrl+n":
		if len(m.sugg) > 0 {
			m.suggIdx = (m.suggIdx + 1) % len(m.sugg)
			return m, nil
		}
	case "up", "ctrl+p", "shift+tab":
		if len(m.sugg) > 0 {
			m.suggIdx = (m.suggIdx - 1 + len(m.sugg)) % len(m.sugg)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateSuggestions()
	if m.mode == modeFilter {
		m.filter = m.input.Value()
		m.refresh(nil)
	}
	return m, cmd
}

func (m *Model) submitInput() {
	val := strings.TrimSpace(m.input.Value())
	md := m.mode
	m.mode = modeNormal
	m.input.Blur()
	m.sugg = nil

	switch md {
	case modeFilter:
		m.filter = val
		m.refresh(nil)
	case modeAdd:
		if val == "" {
			m.setStatus("nothing added")
			return
		}
		t := todo.Parse(val)
		t.NormalizeDue(time.Now())
		if m.cfg.Tasks.AddCreationDate && t.Created.IsZero() && !t.Done {
			y, mo, d := time.Now().Date()
			t.Created = time.Date(y, mo, d, 0, 0, 0, 0, time.Local)
		}
		if m.mutate(func() { m.tasks = append(m.tasks, t) }) {
			m.setStatus("added")
		}
		m.refresh(t)
	case modeEdit:
		t := m.editing
		m.editing = nil
		if t == nil {
			return
		}
		if val == "" {
			m.setStatus("edit cancelled (empty) — use d to delete")
			return
		}
		nt := todo.Parse(val)
		nt.NormalizeDue(time.Now())
		if nt.String() == t.String() {
			return
		}
		if m.mutate(func() { *t = *nt }) {
			m.setStatus("saved")
		}
		m.refresh(t)
	}
}

var keywordWords = []string{
	"due:today", "due:tomorrow", "due:mon", "due:tue", "due:wed", "due:thu", "due:fri", "due:sat", "due:sun", "due:+1w", "due:+1m",
	"rec:1d", "rec:1b", "rec:1w", "rec:2w", "rec:1m", "rec:1y", "rec:+1w", "rec:+1m", "rec:+1y",
}

// wordAtCursor returns the start index (in runes) and text of the word that
// ends at the cursor.
func (m *Model) wordAtCursor() (int, string) {
	val := []rune(m.input.Value())
	pos := clamp(m.input.Position(), 0, len(val))
	start := pos
	for start > 0 && val[start-1] != ' ' {
		start--
	}
	return start, string(val[start:pos])
}

func (m *Model) updateSuggestions() {
	prev := ""
	if m.suggIdx < len(m.sugg) {
		prev = m.sugg[m.suggIdx]
	}
	m.sugg, m.suggIdx = nil, 0
	_, w := m.wordAtCursor()
	if w == "" {
		return
	}
	lw := strings.ToLower(w)
	var pool []string
	switch {
	case w[0] == '+' || w[0] == '@':
		pool = m.known
	case len(lw) >= 3 && (strings.HasPrefix("due:", lw) || strings.HasPrefix(lw, "due:") ||
		strings.HasPrefix("rec:", lw) || strings.HasPrefix(lw, "rec:")):
		if m.mode == modeFilter {
			return
		}
		pool = keywordWords
	default:
		return
	}
	for _, c := range pool {
		if c != w && strings.HasPrefix(strings.ToLower(c), lw) {
			m.sugg = append(m.sugg, c)
			if len(m.sugg) == 8 {
				break
			}
		}
	}
	for i, s := range m.sugg {
		if s == prev {
			m.suggIdx = i
		}
	}
}

func (m *Model) acceptSuggestion() {
	start, w := m.wordAtCursor()
	s := m.sugg[m.suggIdx]
	val := []rune(m.input.Value())
	end := start + len([]rune(w))
	rest := val[end:]
	ins := []rune(s)
	if len(rest) == 0 || rest[0] != ' ' {
		ins = append(ins, ' ')
	}
	nv := string(val[:start]) + string(ins) + string(rest)
	m.input.SetValue(nv)
	m.input.SetCursor(start + len(ins))
	m.updateSuggestions()
	if m.mode == modeFilter {
		m.filter = m.input.Value()
		m.refresh(nil)
	}
}

// ---- small utils -----------------------------------------------------------

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
