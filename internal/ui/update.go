// The update loop: messages, keys in normal mode, chords, cursor moves
// and y/n questions.

package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fitInput()
		m.clampCursor()
		return m, nil
	case editorDoneMsg:
		m.editorDone(msg)
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
		typing := m.mode == modeAdd || m.mode == modeEdit || m.mode == modeFilter || m.mode == modePalette || m.mode == modeSaveName
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
		case modeAdd, modeEdit:
			if m.pick != nil {
				if m.alarmPickerKey(msg.String()) {
					return m, nil
				}
				// the due: picker closed on a key it doesn't use; type it
			}
			if m.choice != nil {
				m.choiceKey(msg.String())
				return m, nil
			}
			return m.updateInput(msg)
		case modeFilter, modeSaveName:
			return m.updateInput(msg)
		case modePalette:
			return m.updatePalette(msg)
		case modePriority:
			m.updatePriority(msg)
			return m, nil
		case modeTheme:
			m.updateThemePicker(msg)
			return m, nil
		case modeConfirm:
			m.updateConfirm(msg)
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
		if m.scrollHelp(k) {
			return m, nil
		}
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
	if m.calendar && m.focus == focusList && m.calendarKey(k) {
		return m, nil
	}
	if m.agenda && m.focus == focusList && m.agendaKey(k) {
		return m, nil
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
		for c >= 0 && c < len(m.side) && m.side[c].Kind == entryHeader {
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
	m.mode, m.confirm, m.onConfirm, m.confirmNo = modeConfirm, prompt, fn, false
}

// updateConfirm answers the y/n question: y or n, or enter on the button
// that's highlighted (yes, until ←→ or tab moves it). Any other key is no.
func (m *Model) updateConfirm(msg tea.KeyMsg) {
	k := strings.ToLower(msg.String())
	switch k {
	case "left", "right", "h", "l", "tab", "shift+tab":
		m.confirmNo = !m.confirmNo
		return
	case "enter":
		if !m.confirmNo {
			k = "y"
		}
	}
	m.mode = modeNormal
	if k == "y" {
		m.onConfirm(m)
	} else {
		m.setStatus("cancelled")
	}
}
