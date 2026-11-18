package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// ---- visual mode -------------------------------------------------------------

func (m *Model) toggleVisual() {
	if m.visual {
		m.endVisual()
		return
	}
	// Nothing is marked yet; until something is, actions still apply to
	// the row under the cursor.
	m.visual, m.sel = true, map[*todo.Task]bool{}
}

func (m *Model) selectAll() {
	m.visual, m.sel = true, map[*todo.Task]bool{}
	for _, t := range m.visible {
		m.sel[t] = true
	}
}

func (m *Model) endVisual() { m.visual, m.sel = false, nil }

// toggleSelected flips the row under the cursor and moves down, so space
// can be held to sweep a run of rows.
func (m *Model) toggleSelected() {
	t := m.selected()
	if t == nil {
		return
	}
	if m.sel[t] {
		delete(m.sel, t)
	} else {
		m.sel[t] = true
	}
	m.move(moveDown)
}

// targets are the tasks an action applies to: the visual selection (in
// list order) if there is one, otherwise the task under the cursor.
func (m *Model) targets() []*todo.Task {
	if m.visual && len(m.sel) > 0 {
		var ts []*todo.Task
		for _, t := range m.visible {
			if m.sel[t] {
				ts = append(ts, t)
			}
		}
		if len(ts) > 0 {
			return ts
		}
	}
	if t := m.selected(); t != nil {
		return []*todo.Task{t}
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// ---- bulk edits --------------------------------------------------------------

// toggleTargets completes the targets, or reopens them if all are done.
// Recurring tasks queue their next occurrence. It is one undo step.
func (m *Model) toggleTargets() {
	ts := m.targets()
	if len(ts) == 0 {
		return
	}
	defer m.endVisual()
	now := time.Now()

	var open []*todo.Task
	for _, t := range ts {
		if !t.Done {
			open = append(open, t)
		}
	}
	if len(open) == 0 {
		if m.mutate(func() {
			for _, t := range ts {
				t.Uncomplete()
			}
		}) {
			m.setStatus("reopened %s", plural(len(ts), "task"))
		}
		m.refresh(ts[0])
		return
	}

	nexts := map[*todo.Task]*todo.Task{}
	for _, t := range open {
		if n, ok := t.Next(now); ok {
			nexts[t] = n
		}
	}
	complete := func() {
		for _, t := range open {
			t.Complete(now)
			if n := nexts[t]; n != nil {
				m.insertAfter(t, n)
			}
		}
	}
	msg := "completed " + plural(len(open), "task") + " — u to undo"
	if len(open) == 1 {
		if n := nexts[open[0]]; n != nil {
			if d, ok := n.Due(); ok {
				msg = "completed — next due " + d.Format(todo.DateLayout)
			} else {
				msg = "completed — next occurrence added"
			}
		}
	} else if len(nexts) > 0 {
		msg = fmt.Sprintf("completed %s, %d repeat — u to undo", plural(len(open), "task"), len(nexts))
	}

	if m.cfg.Tasks.AutoArchive {
		if m.stale() {
			m.setError("todo.txt changed on disk — press r to reload first")
			return
		}
		complete()
		m.archive(open)
		if !m.statusErr {
			m.setStatus("%s (archived)", strings.TrimSuffix(msg, " — u to undo"))
		}
		return
	}
	if m.mutate(complete) {
		m.setStatus("%s", msg)
	}
	m.refresh(open[0])
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

func (m *Model) deleteTargets() {
	ts := m.targets()
	if len(ts) == 0 {
		return
	}
	prompt := fmt.Sprintf("Delete %q? (y/n)", truncate(ts[0].Text, 50))
	if len(ts) > 1 {
		prompt = fmt.Sprintf("Delete %s? (y/n)", plural(len(ts), "task"))
	}
	m.ask(prompt, func(m *Model) {
		rm := map[*todo.Task]bool{}
		for _, t := range ts {
			rm[t] = true
		}
		if m.mutate(func() { m.removeTasks(rm) }) {
			m.setStatus("deleted %s — u to undo", plural(len(ts), "task"))
		}
		m.endVisual()
		m.refresh(nil)
	})
}

func (m *Model) bumpPriority(up bool) {
	var ts []*todo.Task
	for _, t := range m.targets() {
		if !t.Done {
			ts = append(ts, t)
		}
	}
	if len(ts) == 0 {
		return
	}
	bump := func(p byte) byte {
		switch {
		case up && p == 0:
			return 'A'
		case up && p > 'A':
			return p - 1
		case !up && p == 'Z':
			return 0
		case !up && p != 0:
			return p + 1
		}
		return p
	}
	changed := false
	for _, t := range ts {
		changed = changed || bump(t.Priority) != t.Priority
	}
	if !changed {
		return
	}
	m.mutate(func() {
		for _, t := range ts {
			t.Priority = bump(t.Priority)
		}
	})
	m.refresh(ts[0])
}

func (m *Model) updatePriority(msg tea.KeyMsg) {
	m.mode = modeNormal
	ts := m.targets()
	if len(ts) == 0 {
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
	var open []*todo.Task
	for _, t := range ts {
		if !t.Done {
			open = append(open, t)
		}
	}
	if p != 0 && len(open) == 0 {
		m.setError("completed tasks can't have a priority")
		return
	}
	apply := ts
	if p != 0 {
		apply = open
	}
	m.mutate(func() {
		for _, t := range apply {
			t.Priority = p
		}
	})
	m.endVisual()
	m.refresh(ts[0])
}

// ---- reordering ----------------------------------------------------------------

// compareTasks orders two tasks under a sort mode, ignoring file order; 0
// means they tie and file order decides.
func compareTasks(mode string, a, b *todo.Task) int {
	if mode == "file" {
		return 0
	}
	if a.Done != b.Done {
		if !a.Done {
			return -1
		}
		return 1
	}
	pri := func(t *todo.Task) int {
		if t.Priority == 0 {
			return 27
		}
		return int(t.Priority - 'A')
	}
	cmpInt := func(x, y int) int {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	cmpDue := func() int {
		da, oka := a.Due()
		db, okb := b.Due()
		switch {
		case oka != okb:
			if oka {
				return -1
			}
			return 1
		case oka && da.Before(db):
			return -1
		case oka && db.Before(da):
			return 1
		}
		return 0
	}
	switch mode {
	case "priority":
		if c := cmpInt(pri(a), pri(b)); c != 0 {
			return c
		}
		return cmpDue()
	case "due":
		if c := cmpDue(); c != 0 {
			return c
		}
		return cmpInt(pri(a), pri(b))
	case "created":
		switch {
		case a.Created.Equal(b.Created):
			return 0
		case a.Created.IsZero():
			return 1
		case b.Created.IsZero():
			return -1
		case a.Created.After(b.Created):
			return -1
		}
		return 1
	}
	return 0
}

func (m *Model) sortVisible() {
	order := make(map[*todo.Task]int, len(m.tasks))
	for i, t := range m.tasks {
		order[t] = i
	}
	mode := sortModes[m.sortIdx]
	sort.SliceStable(m.visible, func(i, j int) bool {
		a, b := m.visible[i], m.visible[j]
		if c := compareTasks(mode, a, b); c != 0 {
			return c < 0
		}
		return order[a] < order[b]
	})
}

// moveTask swaps the task under the cursor with its neighbor in the file,
// which moves it within the list. In any sort but "file" only tasks that
// tie under the sort can swap, or the list order would not change.
func (m *Model) moveTask(dir int) {
	t := m.selected()
	j := m.cursor + dir
	if t == nil || j < 0 || j >= len(m.visible) {
		return
	}
	n := m.visible[j]
	mode := sortModes[m.sortIdx]
	if compareTasks(mode, t, n) != 0 {
		m.setError("can only move within the same %s here — press s for file order", map[string]string{
			"priority": "priority and due date", "due": "due date and priority", "created": "creation date",
		}[mode])
		return
	}
	ti, ni := -1, -1
	for i, x := range m.tasks {
		switch x {
		case t:
			ti = i
		case n:
			ni = i
		}
	}
	if ti < 0 || ni < 0 {
		return
	}
	m.mutate(func() { m.tasks[ti], m.tasks[ni] = m.tasks[ni], m.tasks[ti] })
	m.refresh(t)
}
