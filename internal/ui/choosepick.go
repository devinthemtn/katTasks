package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/devinthemtn/katTasks/internal/script"
)

// A /name script that calls kat.choose opens this list over the task
// popup. Picking (or cancelling) runs the script again from the start with
// the answer, so a script can ask several questions in a row.

type scriptChoice struct {
	s          script.Script
	start, end int      // the /name word, in runes of the input
	answers    []string // earlier picks, in kat.choose call order
	prompt     string
	options    []string
	idx        int
	submit     bool // opened by enter: save the task once it's answered
}

func (m *Model) openChoice(s script.Script, start, end int, answers []string, c *script.Choice) {
	m.choice = &scriptChoice{
		s: s, start: start, end: end, answers: answers,
		prompt: c.Prompt, options: c.Options,
	}
	m.sugg = nil
}

// choiceKey handles a key while the choice list is open. Besides ↑↓ and
// enter, a digit picks that row and a letter picks the only option that
// starts with it, so "/pri" then "b" sets (B).
func (m *Model) choiceKey(k string) {
	c := m.choice
	switch k {
	case "esc":
		m.answerChoice("")
		return
	case "enter", "tab":
		m.answerChoice(c.options[c.idx])
		return
	case "down", "j", "ctrl+n":
		c.idx = (c.idx + 1) % len(c.options)
		return
	case "up", "k", "ctrl+p", "shift+tab":
		c.idx = (c.idx - 1 + len(c.options)) % len(c.options)
		return
	}
	if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(c.options) {
		m.answerChoice(c.options[n-1])
		return
	}
	if len([]rune(k)) == 1 {
		hit := -1
		for i, o := range c.options {
			if strings.HasPrefix(strings.ToLower(o), strings.ToLower(k)) {
				if hit >= 0 {
					return // more than one; use the arrows
				}
				hit = i
			}
		}
		if hit >= 0 {
			m.answerChoice(c.options[hit])
		}
	}
}

// answerChoice closes the list and runs the script again with ans ("" for
// cancel).
func (m *Model) answerChoice(ans string) {
	c := m.choice
	m.choice = nil
	answers := append(append([]string(nil), c.answers...), ans)
	ok := m.runInlineScriptWith(c.s, c.start, c.end, answers)
	if m.choice != nil { // it asked another question
		m.choice.submit = c.submit
		return
	}
	m.updateSuggestions()
	if ok && c.submit && m.expandScripts() {
		if m.choice != nil { // another /name word asked something
			m.choice.submit = true
			return
		}
		m.submitInput()
	}
}

// viewChoice draws the list for the task popup, w cells wide.
func (m Model) viewChoice(w int) []string {
	c := m.choice
	lines := []string{m.st.prompt.Render(ansi.Truncate(c.prompt, w, "…")), ""}
	for i, o := range c.options {
		row := " " + strconv.Itoa(i+1) + "  " + o + " "
		if i == c.idx {
			lines = append(lines, m.st.suggestOn.Render(ansi.Truncate(row, w, "…")))
		} else {
			lines = append(lines, m.st.suggest.Render(ansi.Truncate(row, w, "…")))
		}
	}
	key := func(k string) string { return m.st.helpKey.Render(k) }
	lines = append(lines, "", ansi.Truncate(key("↑↓")+m.st.faint.Render(" choose · ")+
		key("enter")+m.st.faint.Render(" pick · ")+key("esc")+m.st.faint.Render(" cancel"), w, "…"))
	return lines
}
