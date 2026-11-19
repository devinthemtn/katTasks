package ui

// This file runs the terminal UI's model without a terminal, for the
// katTasks-gui desktop app. The app sends it key presses and clicks, and it
// returns a Snapshot of the screen as data for the app to draw. Every key,
// chord, mode and message therefore works exactly as in the terminal UI.

import (
	"strings"
	"sync"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// HeadlessOptions are the desktop app's hooks.
type HeadlessOptions struct {
	// Copy puts text on the clipboard (yy, yb).
	Copy func(text string) error
	// Changed is called with a new snapshot when the screen changes on its
	// own, e.g. when a chord times out.
	Changed func(Snapshot)
	// Quit is called when the user quits (q).
	Quit func()
}

// Headless is a Model driven by the desktop app. It is safe for concurrent
// use.
type Headless struct {
	mu  sync.Mutex
	m   Model
	hop HeadlessOptions
	seq int
}

// NewHeadless loads settings and the todo file as New does.
func NewHeadless(m Model, hop HeadlessOptions) *Headless {
	m.copyFn = hop.Copy
	if m.width == 0 {
		m.width, m.height = 100, 30
	}
	return &Headless{m: m, hop: hop}
}

// do runs fn on the model and returns the new screen. A command fn returns
// (a chord timer, a copy) runs in the background.
func (h *Headless) do(fn func(m *Model) tea.Cmd) Snapshot {
	h.mu.Lock()
	cmd := fn(&h.m)
	h.seq++
	s := h.m.Snapshot()
	s.Seq = h.seq
	h.mu.Unlock()
	h.run(cmd)
	return s
}

// update feeds msg to the model as Bubble Tea would.
func (h *Headless) update(msg tea.Msg) func(m *Model) tea.Cmd {
	return func(m *Model) tea.Cmd {
		nm, cmd := m.Update(msg)
		*m = nm.(Model)
		return cmd
	}
}

// run executes cmd in the background and feeds its message back in.
func (h *Headless) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		switch msg := cmd().(type) {
		case nil:
		case tea.QuitMsg:
			if h.hop.Quit != nil {
				h.hop.Quit()
			}
		case tea.BatchMsg:
			for _, c := range msg {
				h.run(c)
			}
		default:
			s := h.do(h.update(msg))
			if h.hop.Changed != nil {
				h.hop.Changed(s)
			}
		}
	}()
}

// State returns the current screen.
func (h *Headless) State() Snapshot {
	return h.do(func(*Model) tea.Cmd { return nil })
}

// Resize sets how many task rows fit in the list and how many characters
// fit across the window; half-page moves and scrolling depend on them.
func (h *Headless) Resize(cols, rows int) Snapshot {
	return h.do(h.update(tea.WindowSizeMsg{Width: max(cols, 40), Height: max(rows, 1) + headerHeight + footerHeight + 2}))
}

// Key presses one key, named as Bubble Tea names it: "j", "G", "ctrl+d",
// "shift+tab", "alt+b", "enter", " " for space.
func (h *Headless) Key(name string) Snapshot {
	k, ok := ParseKeyMsg(name)
	if !ok || k.String() == "ctrl+c" { // the window's close button quits
		return h.State()
	}
	return h.do(h.update(k))
}

// Paste types text into the input being edited, if any.
func (h *Headless) Paste(text string) Snapshot {
	text = strings.Join(strings.Fields(text), " ") // tasks are one line
	if text == "" {
		return h.State()
	}
	return h.do(h.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true}))
}

// keyTypes maps Bubble Tea's key names to key types.
var keyTypes = func() map[string]tea.KeyType {
	out := map[string]tea.KeyType{}
	for k := tea.KeyType(-200); k <= 200; k++ {
		if s := k.String(); s != "" && k != tea.KeyRunes {
			if _, dup := out[s]; !dup {
				out[s] = k
			}
		}
	}
	return out
}()

// ParseKeyMsg is the inverse of tea.KeyMsg.String.
func ParseKeyMsg(name string) (tea.KeyMsg, bool) {
	if name == "" {
		return tea.KeyMsg{}, false
	}
	var k tea.KeyMsg
	if rest, ok := strings.CutPrefix(name, "alt+"); ok && rest != "" {
		k.Alt, name = true, rest
	}
	if t, ok := keyTypes[name]; ok {
		k.Type = t
		if t == tea.KeySpace {
			k.Runes = []rune{' '} // as a terminal sends it; text inputs need the rune
		}
		return k, true
	}
	if utf8.RuneCountInString(name) != 1 {
		return tea.KeyMsg{}, false
	}
	k.Type, k.Runes = tea.KeyRunes, []rune(name)
	return k, true
}

// ---- mouse -------------------------------------------------------------------

// ClickTask puts the cursor on the visible task at index; with open it
// also edits it, and with toggle it marks it done or open.
func (h *Headless) ClickTask(index int, open, toggle bool) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.mode != modeNormal || m.showHelp || index < 0 || index >= len(m.visible) {
			return nil
		}
		m.status = ""
		m.focus, m.cursor = focusList, index
		m.clampCursor()
		switch {
		case toggle:
			return actionByName["toggle_done"].run(m)
		case open:
			return actionByName["edit"].run(m)
		case m.visual:
			if t := m.visible[index]; m.sel[t] {
				delete(m.sel, t)
			} else {
				m.sel[t] = true
			}
		}
		return nil
	})
}

// ClickCalendarTask lists the calendar's chosen day with the cursor on its
// task at index; with open it also edits it.
func (h *Headless) ClickCalendarTask(index int, open bool) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.mode != modeNormal || !m.calendar || m.showHelp {
			return nil
		}
		m.status = ""
		return m.calendarTask(index, open)
	})
}

// ClickSidebar shows the sidebar entry at index.
func (h *Headless) ClickSidebar(index int) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.mode != modeNormal || m.showHelp || index < 0 || index >= len(m.side) || m.side[index].Kind == entryHeader {
			return nil
		}
		m.status = ""
		m.focus, m.sideCursor = focusSidebar, index
		m.cursor, m.offset = 0, 0
		m.refresh(nil)
		m.cursor = 0
		m.clampCursor()
		return nil
	})
}

// Scroll moves the task list (or, with sidebar, the sidebar) by n rows,
// keeping the cursor on screen as a terminal list would.
func (h *Headless) Scroll(n int, sidebar bool) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		rows := m.listRows()
		if sidebar {
			m.sideOffset = clamp(m.sideOffset+n, 0, len(m.side)-rows)
			return nil
		}
		m.offset = clamp(m.offset+n, 0, len(m.visible)-rows)
		m.cursor = clamp(m.cursor, m.offset, m.offset+rows-1)
		m.clampCursor()
		return nil
	})
}

// Run runs the named action (see the command palette), as its key would.
func (h *Headless) Run(name string) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		a := actionByName[name]
		if a == nil || m.mode != modeNormal {
			return nil
		}
		m.showHelp = false
		m.status, m.pending = "", ""
		return a.run(m)
	})
}

// Pick chooses row index of the open list: a command palette entry, a
// theme, a completion, a calendar day or an alarm picker day. With commit it also runs, saves,
// accepts or opens it.
func (h *Headless) Pick(index int, commit bool) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		switch {
		case m.mode == modePalette && index >= 0 && index < len(m.palShown):
			m.palIdx = index
		case m.mode == modeTheme && index >= 0 && index < len(m.themes):
			m.pickIdx = index
			m.useTheme(m.themes[index].Name)
		case (m.mode == modeAdd || m.mode == modeEdit) && m.pick != nil:
			m.pickDay(index) // a day of the alarm picker; commit sets it
			if commit {
				m.closeAlarmPicker(true)
			}
			return nil
		case m.mode == modeNormal && m.calendar && !m.showHelp:
			m.status = ""
			m.calendarPick(index, commit)
			return nil
		case (m.mode == modeAdd || m.mode == modeEdit || m.mode == modeFilter || m.mode == modeSaveName) &&
			index >= 0 && index < len(m.sugg):
			m.suggIdx = index
			if commit {
				m.acceptSuggestion()
			}
			return nil
		default:
			return nil
		}
		if !commit {
			return nil
		}
		nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		*m = nm.(Model)
		return cmd
	})
}

// SetInputCursor moves the text cursor of the input being edited.
func (h *Headless) SetInputCursor(pos int) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.input.Focused() {
			m.input.SetCursor(pos)
			m.updateSuggestions()
		}
		return nil
	})
}

// SetStatus shows a message in the footer, as the app's own actions do.
func (h *Headless) SetStatus(msg string) {
	h.mu.Lock()
	h.m.setStatus("%s", msg)
	h.mu.Unlock()
}

// AgendaWeek moves the agenda n weeks.
func (h *Headless) AgendaWeek(n int) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.mode != modeNormal {
			return nil
		}
		return m.agendaWeek(n)
	})
}

// CalendarMonth moves the calendar n months.
func (h *Headless) CalendarMonth(n int) Snapshot {
	return h.do(func(m *Model) tea.Cmd {
		if m.calendar && m.mode == modeNormal {
			m.calDay = addMonths(m.calDay, n)
		}
		return nil
	})
}
