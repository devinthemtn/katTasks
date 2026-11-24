// files.watch: rechecking todo.txt now and then for changes made by other
// programs, and the sync action that does the same by hand.

package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// watchMsg is the files.watch timer going off. seq is the timer it came
// from; a settings reload starts a new one and the old one's are ignored.
type watchMsg struct{ seq int }

// watchStartMsg starts the files.watch timer (see Model.Init).
type watchStartMsg struct{}

// watchTick starts the files.watch timer, or returns nil if it's off.
func (m *Model) watchTick() tea.Cmd {
	m.watchSeq++
	every, seq := m.cfg.Files.WatchEvery, m.watchSeq
	if every <= 0 {
		return nil
	}
	return tea.Tick(every, func(time.Time) tea.Msg { return watchMsg{seq} })
}

// watched handles the timer: it reloads todo.txt if it changed, then waits
// again. A change found while a task is being typed, a question is asked or
// tasks are selected waits for the next check, so nothing in progress is
// lost; saving still refuses to overwrite the changed file meanwhile.
func (m *Model) watched(msg watchMsg) tea.Cmd {
	if msg.seq != m.watchSeq {
		return nil
	}
	if m.stale() && m.mode == modeNormal && !m.visual {
		m.syncFile(false)
	}
	return m.watchTick()
}

// syncFile re-reads todo.txt if another program changed it, keeping the
// cursor on the same task. With always it reports that nothing changed too.
func (m *Model) syncFile(always bool) {
	if !m.stale() {
		if always {
			m.setStatus("%s is up to date (%d tasks)", m.cfg.Files.Todo, len(m.tasks))
		}
		return
	}
	var keep string
	if t := m.selected(); t != nil {
		keep = t.String()
	}
	cur := m.cursor
	if err := m.load(); err != nil {
		m.setError("todo.txt changed on disk, but reloading it failed: %v", err)
		return
	}
	m.undo = nil // the undo history is of the old file
	m.cursor = cur
	for i, t := range m.visible {
		if keep != "" && t.String() == keep {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	m.setStatus("todo.txt changed on disk; reloaded %s (%d tasks)", m.cfg.Files.Todo, len(m.tasks))
}
