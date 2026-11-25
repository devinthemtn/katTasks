// Text input (adding, editing, searching, naming) and autocomplete.

package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/todo"
)

func (m *Model) startAdd() {
	m.mode, m.editing = modeAdd, nil
	// Pre-fill the project/context selected in the sidebar.
	if e := m.currentEntry(); e.Kind == entryProject || e.Kind == entryContext {
		m.startInput(" "+e.Value, 0)
		return
	}
	m.startInput("", -1)
}

// popupWidth is the width inside the add/edit popup's border and padding.
func (m Model) popupWidth() int { return clamp(m.width-4, 20, 64) }

// fitInput sizes the input to the box it's drawn in (the add/edit popup,
// or the footer), so it scrolls its text to keep the cursor in view there.
func (m *Model) fitInput() {
	if m.mode == modeAdd || m.mode == modeEdit {
		m.input.Width = m.popupWidth() - 3 // "› " prompt plus the cursor cell
	} else {
		m.input.Width = m.width - 12
	}
	m.input.SetCursor(m.input.Position()) // rescroll for the new width
}

func (m *Model) startInput(val string, cursor int) {
	m.fitInput()
	m.input.SetValue(val)
	if cursor < 0 {
		m.input.CursorEnd()
	} else {
		m.input.SetCursor(cursor)
	}
	m.input.Focus()
	if m.mode == modeAdd || m.mode == modeEdit {
		m.loadScripts()
	}
	m.updateSuggestions()
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	typingTask := m.mode == modeAdd || m.mode == modeEdit // not a search or a name
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
		// Like tab while the list is showing; a /script word still runs
		// and saves, below.
		if typingTask && len(m.sugg) > 0 && !m.isScriptSuggestion() {
			m.acceptSuggestion()
			return m, nil
		}
		if typingTask && !m.expandScripts() {
			if m.choice != nil {
				m.choice.submit = true // save once it's answered
			}
			return m, nil // the script's error is in the footer; keep typing
		}
		m.submitInput()
		return m, nil
	case "tab":
		if len(m.sugg) > 0 {
			m.acceptSuggestion()
			return m, nil
		}
		if typingTask && m.changeWord() {
			return m, nil
		}
	case "down", "ctrl+n":
		if m.moveSugg(1) {
			return m, nil
		}
	case "up", "ctrl+p", "shift+tab":
		if m.moveSugg(-1) {
			return m, nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	// A terminal sends fast typing as one message ("due:" or "e:"), so look
	// at the last rune typed, not the whole message. A paste doesn't count.
	typedColon := msg.Type == tea.KeyRunes && !msg.Paste && len(msg.Runes) > 0 && msg.Runes[len(msg.Runes)-1] == ':'
	if _, w := m.wordAtCursor(); typingTask && typedColon {
		switch {
		case strings.EqualFold(w, "alarm:"):
			m.openAlarmPicker()
			return m, cmd
		case strings.EqualFold(w, "due:"):
			m.openDuePicker()
			return m, cmd
		}
	}
	if m.input.Value() != before {
		m.updateSuggestions()
	} else {
		// Only moving the cursor: hide the list, so the arrows (and enter)
		// don't get caught on every +project or due: word passed over.
		m.sugg, m.suggIdx = nil, 0
	}
	if m.mode == modeFilter {
		m.filter = m.input.Value()
		m.refresh(nil)
	}
	return m, cmd
}

// moveSugg moves the highlight d places through the suggestions, wrapping
// around; false if there are none.
func (m *Model) moveSugg(d int) bool {
	if len(m.sugg) == 0 {
		return false
	}
	m.suggIdx = (m.suggIdx + d + len(m.sugg)) % len(m.sugg)
	return true
}

func (m *Model) submitInput() {
	val := strings.TrimSpace(m.input.Value())
	md := m.mode
	m.mode = modeNormal
	m.input.Blur()
	m.sugg = nil

	switch md {
	case modeSaveName:
		m.saveFilter(val)
	case modeFilter:
		m.filter = val
		m.refresh(nil)
	case modeAdd:
		if val == "" {
			m.setStatus("nothing added")
			return
		}
		t := todo.NewTask(val, time.Now(), m.cfg.Tasks.AddCreationDate)
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
		nt.Normalize(time.Now())
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
	"due:", "due:today", "due:tomorrow", "due:mon", "due:tue", "due:wed", "due:thu", "due:fri", "due:sat", "due:sun", "due:+1w", "due:+1m",
	"alarm:",
	"rec:1d", "rec:1b", "rec:1w", "rec:2w", "rec:1m", "rec:1y", "rec:+1w", "rec:+1m", "rec:+1y",
}

// atSuggestions are at: times for word (lowercase, starting "at:"), soonest
// after now first. "at:" alone gives the next half hours; a partial time
// ("at:9", "at:143") gives the quarter hours it could be, and one with a or
// p in it ("at:3p") gives 12-hour times. They're written without a colon
// (at:930, at:1430, at:315pm), as todo.txt wants tag values.
func atSuggestions(word string, now time.Time) []string {
	v := strings.TrimPrefix(word, "at:")
	step := 15
	if v == "" {
		step = 30
	}
	twelve := strings.ContainsAny(v, "ap")
	start := now.Hour()*60 + now.Minute()
	start += step - start%step // the next slot after now
	var out []string
	for i := 0; i < 24*60/step; i++ {
		min := (start + i*step) % (24 * 60)
		if !twelve {
			out = append(out, "at:"+todo.FormatClockTag(min))
			continue
		}
		h, mm, ap := min/60, min%60, "am"
		if h >= 12 {
			ap = "pm"
		}
		if h = h % 12; h == 0 {
			h = 12
		}
		if mm == 0 {
			out = append(out, fmt.Sprintf("at:%d%s", h, ap))
		} else {
			out = append(out, fmt.Sprintf("at:%d%02d%s", h, mm, ap))
		}
	}
	return out
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

// wordSpanAtCursor returns where (in runes) the whole word the cursor is
// in, or just after, starts and ends.
func (m *Model) wordSpanAtCursor() (int, int) {
	val := []rune(m.input.Value())
	pos := clamp(m.input.Position(), 0, len(val))
	start, end := pos, pos
	for start > 0 && val[start-1] != ' ' {
		start--
	}
	for end < len(val) && val[end] != ' ' {
		end++
	}
	return start, end
}

// wordKind is "project", "context" or "tag" for a word that has a list or
// a picker to change it with (+project, @context, due:, alarm:, rec:, at:),
// and "" for any other word.
func wordKind(w string) string {
	lw := strings.ToLower(w)
	switch {
	case len(w) > 1 && w[0] == '+':
		return "project"
	case len(w) > 1 && w[0] == '@':
		return "context"
	case isDueWord(w), isAlarmWord(w), strings.HasPrefix(lw, "rec:"), strings.HasPrefix(lw, "at:"):
		return "tag"
	}
	return ""
}

// inputWord is a word of the line being typed that wordKind knows, from
// rune start to end.
type inputWord struct {
	start, end int
	kind       string
}

// inputWords finds the words of val that can be changed with tab.
func inputWords(val string) []inputWord {
	var out []inputWord
	r := []rune(val)
	for i := 0; i < len(r); {
		if r[i] == ' ' {
			i++
			continue
		}
		j := i
		for j < len(r) && r[j] != ' ' {
			j++
		}
		if k := wordKind(string(r[i:j])); k != "" {
			out = append(out, inputWord{i, j, k})
		}
		i = j
	}
	return out
}

// changeWord handles tab on a word wordKind knows: it puts the cursor at
// the word's end and opens its picker (due:, alarm:) or lists the other
// values it could be (other projects or contexts, rec: intervals, at:
// times), to replace the whole word with. False if the cursor isn't on one.
func (m *Model) changeWord() bool {
	start, end := m.wordSpanAtCursor()
	w := string([]rune(m.input.Value())[start:end])
	if wordKind(w) == "" {
		return false
	}
	m.input.SetCursor(end)
	lw := strings.ToLower(w)
	var pool []string
	switch {
	case isDueWord(w):
		m.openDuePicker()
		return true
	case isAlarmWord(w):
		m.openAlarmPicker()
		return true
	case w[0] == '+' || w[0] == '@':
		for _, k := range m.known {
			if k[0] == w[0] {
				pool = append(pool, k)
			}
		}
	case strings.HasPrefix(lw, "rec:"):
		for _, k := range keywordWords {
			if strings.HasPrefix(k, "rec:") {
				pool = append(pool, k)
			}
		}
	default: // at:
		pool = atSuggestions("at:", time.Now())
	}
	m.sugg, m.suggIdx = nil, 0
	for _, c := range pool {
		if !strings.EqualFold(c, w) {
			m.sugg = append(m.sugg, c)
			if len(m.sugg) == 8 {
				break
			}
		}
	}
	return true
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
	case w[0] == '/' && (m.mode == modeAdd || m.mode == modeEdit):
		m.sugg = m.scriptSuggestions(w)
		for i, s := range m.sugg {
			if s == prev {
				m.suggIdx = i
			}
		}
		return
	case w[0] == '+' || w[0] == '@':
		pool = m.known
	case strings.HasPrefix(lw, "at:"):
		if m.mode == modeFilter {
			return
		}
		pool = atSuggestions(lw, time.Now())
		lw = strings.ReplaceAll(lw, ":", "")
		lw = "at:" + lw[len("at"):] // "at:14:3" matches at:1430
	case len(lw) >= 3 && (strings.HasPrefix("due:", lw) || strings.HasPrefix(lw, "due:") ||
		strings.HasPrefix("rec:", lw) || strings.HasPrefix(lw, "rec:") ||
		strings.HasPrefix("alarm:", lw) && !strings.HasPrefix(lw, "alarm:")):
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
	if m.isScriptSuggestion() {
		if sc, ok := m.findScript(s[1:]); ok {
			m.runInlineScript(sc, start, end)
		}
		m.updateSuggestions()
		return
	}
	if s == "alarm:" || s == "due:" { // completing to these opens a picker
		m.input.SetValue(string(val[:start]) + s + string(val[end:]))
		m.input.SetCursor(start + len([]rune(s)))
		if s == "due:" {
			m.openDuePicker()
		} else {
			m.openAlarmPicker()
		}
		return
	}
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
