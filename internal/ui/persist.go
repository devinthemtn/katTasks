// Saving todo.txt, undo and archiving.

package ui

import (
	"github.com/devinthemtn/katTasks/internal/todo"
)

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

// archive moves the given tasks to done.txt and reports whether it did.
// Archiving touches two files, so it clears the undo history. If it fails,
// both files and the list are left as they were.
func (m *Model) archive(ts []*todo.Task) bool {
	if len(ts) == 0 {
		m.setStatus("nothing to archive")
		return false
	}
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return false
	}
	rm := map[*todo.Task]bool{}
	for _, t := range ts {
		rm[t] = true
	}
	var keep []*todo.Task
	for _, t := range m.tasks {
		if !rm[t] {
			keep = append(keep, t)
		}
	}
	mt, err := todo.Archive(m.cfg.Files.Todo, keep, m.cfg.Files.Done, ts)
	if err != nil {
		m.setError("archive failed: %v", err)
		return false
	}
	m.tasks, m.modTime, m.undo = keep, mt, nil
	m.setStatus("archived %d task(s) to %s", len(ts), m.cfg.Files.Done)
	m.refresh(nil)
	return true
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
