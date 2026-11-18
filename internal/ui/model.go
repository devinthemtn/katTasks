// Package ui implements the katTasks terminal interface.
package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
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
)

var sortModes = []string{"priority", "due", "created", "file"}

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
	sortIdx  int
	hideDone bool
	filter   string

	input     textinput.Model
	editing   *todo.Task
	sugg      []string
	suggIdx   int
	confirm   string
	onConfirm func(*Model)

	status    string
	statusErr bool

	keys     keyMap
	help     help.Model
	showHelp bool

	width, height int
}

// New builds the model and loads the todo file.
func New(cfg config.Config) (Model, error) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 0

	m := Model{
		cfg:      cfg,
		hideDone: cfg.HideCompleted,
		input:    ti,
		keys:     newKeyMap(),
		help:     help.New(),
	}
	for i, s := range sortModes {
		if s == cfg.DefaultSort {
			m.sortIdx = i
		}
	}
	if err := m.load(); err != nil {
		return m, err
	}
	return m, nil
}

func (m *Model) load() error {
	tasks, mt, err := todo.Load(m.cfg.TodoFile)
	if err != nil {
		return err
	}
	m.tasks, m.modTime = tasks, mt
	m.refresh(nil)
	return nil
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

func (m *Model) sortVisible() {
	order := make(map[*todo.Task]int, len(m.tasks))
	for i, t := range m.tasks {
		order[t] = i
	}
	mode := sortModes[m.sortIdx]
	pri := func(t *todo.Task) int {
		if t.Priority == 0 {
			return 27
		}
		return int(t.Priority - 'A')
	}
	dueKey := func(t *todo.Task) (time.Time, bool) { return t.Due() }

	sort.SliceStable(m.visible, func(i, j int) bool {
		a, b := m.visible[i], m.visible[j]
		if mode == "file" {
			return order[a] < order[b]
		}
		if a.Done != b.Done {
			return !a.Done
		}
		switch mode {
		case "priority":
			if pri(a) != pri(b) {
				return pri(a) < pri(b)
			}
			da, oka := dueKey(a)
			db, okb := dueKey(b)
			if oka != okb {
				return oka
			}
			if oka && !da.Equal(db) {
				return da.Before(db)
			}
		case "due":
			da, oka := dueKey(a)
			db, okb := dueKey(b)
			if oka != okb {
				return oka
			}
			if oka && !da.Equal(db) {
				return da.Before(db)
			}
			if pri(a) != pri(b) {
				return pri(a) < pri(b)
			}
		case "created":
			if !a.Created.Equal(b.Created) {
				if a.Created.IsZero() || b.Created.IsZero() {
					return !a.Created.IsZero()
				}
				return a.Created.After(b.Created)
			}
		}
		return order[a] < order[b]
	})
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
	return !todo.ModTime(m.cfg.TodoFile).Equal(m.modTime)
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
	mt, err := todo.Save(m.cfg.TodoFile, m.tasks)
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
	if err := todo.Append(m.cfg.DoneFile, ts); err != nil {
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
		m.setStatus("archived %d task(s) to %s", len(ts), m.cfg.DoneFile)
	}
	m.refresh(nil)
}

// ---- update ----------------------------------------------------------------

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		m.input.Width = msg.Width - 12
		m.clampCursor()
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeAdd, modeEdit, modeFilter:
			return m.updateInput(msg)
		case modePriority:
			m.updatePriority(msg)
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
		// Keys typed quickly can arrive as one multi-rune message; handle
		// them one at a time so e.g. "pB" sets priority B.
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Paste {
			var cmds []tea.Cmd
			var mdl tea.Model = m
			for _, r := range msg.Runes {
				var cmd tea.Cmd
				mdl, cmd = mdl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				cmds = append(cmds, cmd)
			}
			return mdl, tea.Batch(cmds...)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	m.status = ""

	if m.showHelp {
		if key.Matches(msg, k.Quit) {
			return m, tea.Quit
		}
		m.showHelp = false
		return m, nil
	}

	switch {
	case key.Matches(msg, k.Quit):
		return m, tea.Quit
	case key.Matches(msg, k.Help):
		m.showHelp = true
	case key.Matches(msg, k.SwitchPane):
		if m.focus == focusList {
			m.focus = focusSidebar
		} else {
			m.focus = focusList
		}
	case key.Matches(msg, k.Left):
		m.focus = focusSidebar
	case key.Matches(msg, k.Right):
		m.focus = focusList

	case key.Matches(msg, k.Up), key.Matches(msg, k.Down), key.Matches(msg, k.Top),
		key.Matches(msg, k.Bottom), key.Matches(msg, k.HalfUp), key.Matches(msg, k.HalfDown):
		m.move(msg)

	case m.focus == focusSidebar && msg.String() == "enter":
		m.focus = focusList

	case key.Matches(msg, k.Add):
		m.startAdd()
	case key.Matches(msg, k.Edit):
		if t := m.selected(); t != nil {
			m.mode, m.editing = modeEdit, t
			m.startInput(t.String(), -1)
		}
	case key.Matches(msg, k.Toggle):
		m.toggle()
	case key.Matches(msg, k.Delete):
		if t := m.selected(); t != nil {
			m.ask(fmt.Sprintf("Delete %q? (y/n)", truncate(t.Text, 50)), func(m *Model) {
				if m.mutate(func() { m.removeTasks(map[*todo.Task]bool{t: true}) }) {
					m.setStatus("deleted — u to undo")
				}
				m.refresh(nil)
			})
		}
	case key.Matches(msg, k.Priority):
		if m.selected() != nil {
			m.mode = modePriority
		}
	case key.Matches(msg, k.PriUp), key.Matches(msg, k.PriDown):
		m.bumpPriority(key.Matches(msg, k.PriUp))

	case key.Matches(msg, k.Filter):
		m.mode = modeFilter
		m.startInput(m.filter, -1)
	case key.Matches(msg, k.ClearFilter):
		if m.filter != "" {
			m.filter = ""
			m.refresh(nil)
		} else if m.sideCursor != 0 {
			m.sideCursor = 0
			m.refresh(nil)
		}
	case key.Matches(msg, k.Sort):
		m.sortIdx = (m.sortIdx + 1) % len(sortModes)
		m.refresh(nil)
		m.setStatus("sorted by %s", sortModes[m.sortIdx])
	case key.Matches(msg, k.HideDone):
		m.hideDone = !m.hideDone
		m.refresh(nil)
		if m.hideDone {
			m.setStatus("hiding completed tasks")
		} else {
			m.setStatus("showing completed tasks")
		}
	case key.Matches(msg, k.Archive):
		var done []*todo.Task
		for _, t := range m.tasks {
			if t.Done {
				done = append(done, t)
			}
		}
		if len(done) == 0 {
			m.setStatus("no completed tasks to archive")
			break
		}
		m.ask(fmt.Sprintf("Move %d completed task(s) to %s? This can't be undone. (y/n)", len(done), m.cfg.DoneFile),
			func(m *Model) { m.archive(done) })
	case key.Matches(msg, k.Undo):
		m.undoLast()
	case key.Matches(msg, k.Reload):
		if err := m.load(); err != nil {
			m.setError("reload failed: %v", err)
		} else {
			m.undo = nil
			m.setStatus("reloaded %s (%d tasks)", m.cfg.TodoFile, len(m.tasks))
		}
	}
	return m, nil
}

func (m *Model) move(msg tea.KeyMsg) {
	k := m.keys
	half := m.listRows() / 2
	if m.focus == focusSidebar {
		c := m.sideCursor
		dir := 1
		switch {
		case key.Matches(msg, k.Up):
			c, dir = c-1, -1
		case key.Matches(msg, k.Down):
			c++
		case key.Matches(msg, k.Top):
			c = 0
		case key.Matches(msg, k.Bottom):
			c, dir = len(m.side)-1, -1
		case key.Matches(msg, k.HalfUp):
			c, dir = c-half, -1
		case key.Matches(msg, k.HalfDown):
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
	switch {
	case key.Matches(msg, k.Up):
		m.cursor--
	case key.Matches(msg, k.Down):
		m.cursor++
	case key.Matches(msg, k.Top):
		m.cursor = 0
	case key.Matches(msg, k.Bottom):
		m.cursor = len(m.visible) - 1
	case key.Matches(msg, k.HalfUp):
		m.cursor -= half
	case key.Matches(msg, k.HalfDown):
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

func (m *Model) toggle() {
	t := m.selected()
	if t == nil {
		return
	}
	now := time.Now()
	if t.Done {
		m.mutate(t.Uncomplete)
		m.refresh(t)
		return
	}

	// Recurring task: queue the next occurrence right after this one.
	next, recurs := t.Next(now)
	complete := func() {
		t.Complete(now)
		if recurs {
			m.insertAfter(t, next)
		}
	}
	msg := "completed — u to undo"
	if recurs {
		if d, ok := next.Due(); ok {
			msg = "completed — next due " + d.Format(todo.DateLayout)
		} else {
			msg = "completed — next occurrence added"
		}
	}

	if m.cfg.AutoArchive {
		if m.stale() {
			m.setError("todo.txt changed on disk — press r to reload first")
			return
		}
		complete()
		m.archive([]*todo.Task{t})
		if !m.statusErr && recurs {
			m.setStatus("%s (archived)", msg)
		}
		if recurs {
			m.refresh(next)
		}
		return
	}
	if m.mutate(complete) {
		m.setStatus("%s", msg)
	}
	m.refresh(t)
}

// insertAfter puts n directly after t in file order.
func (m *Model) insertAfter(t, n *todo.Task) {
	for i, x := range m.tasks {
		if x == t {
			m.tasks = append(m.tasks[:i+1], append([]*todo.Task{n}, m.tasks[i+1:]...)...)
			return
		}
	}
	m.tasks = append(m.tasks, n)
}

func (m *Model) bumpPriority(up bool) {
	t := m.selected()
	if t == nil || t.Done {
		return
	}
	p := t.Priority
	switch {
	case up && p == 0:
		p = 'A'
	case up && p > 'A':
		p--
	case !up && p == 'Z':
		p = 0
	case !up && p != 0:
		p++
	}
	if p != t.Priority {
		m.mutate(func() { t.Priority = p })
		m.refresh(t)
	}
}

func (m *Model) updatePriority(msg tea.KeyMsg) {
	m.mode = modeNormal
	t := m.selected()
	if t == nil {
		return
	}
	s := msg.String()
	var p byte
	switch {
	case len(s) == 1 && strings.ToUpper(s)[0] >= 'A' && strings.ToUpper(s)[0] <= 'Z':
		p = strings.ToUpper(s)[0]
	case s == " " || s == "backspace" || s == "delete" || s == "0":
		p = 0
	default:
		m.setStatus("cancelled")
		return
	}
	if t.Done && p != 0 {
		m.setError("completed tasks can't have a priority")
		return
	}
	m.mutate(func() { t.Priority = p })
	m.refresh(t)
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
		if m.cfg.AddCreationDate && t.Created.IsZero() && !t.Done {
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
