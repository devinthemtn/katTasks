package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// viewTaskPopup renders the centered box used to add or edit a task: the
// input, any completions, and a short reminder of the todo.txt syntax.
func (m Model) viewTaskPopup() string {
	w := m.popupWidth()
	title := "New task"
	if m.mode == modeEdit {
		title = "Edit task"
	}

	lines := []string{m.st.prompt.Render(title), ""}
	for i, l := range m.viewWrappedInput(w - 2) {
		if i == 0 {
			lines = append(lines, m.st.prompt.Render("› ")+l)
		} else {
			lines = append(lines, "  "+l) // lined up under the text
		}
	}
	if len(m.sugg) > 0 {
		var ss []string
		for i, s := range m.sugg {
			if i == m.suggIdx {
				ss = append(ss, m.st.suggestOn.Render(s))
			} else {
				ss = append(ss, m.st.suggest.Render(" "+s+" "))
			}
		}
		row := strings.Join(ss, "")
		if m.isScriptSuggestion() {
			if d := m.scriptDesc(m.sugg[m.suggIdx]); d != "" {
				row += m.st.faint.Render("  " + d)
			}
		}
		lines = append(lines, ansi.Truncate(row, w, "…"))
	} else {
		lines = append(lines, "")
	}

	if m.pick != nil {
		lines = append(lines, m.st.faint.Render(strings.Repeat("─", w)))
		lines = append(lines, m.viewAlarmPicker(w)...)
		return m.st.paneFocused.Padding(0, 1).Width(w + 2).Render(strings.Join(lines, "\n"))
	}
	if m.choice != nil {
		lines = append(lines, m.st.faint.Render(strings.Repeat("─", w)))
		lines = append(lines, m.viewChoice(w)...)
		return m.st.paneFocused.Padding(0, 1).Width(w + 2).Render(strings.Join(lines, "\n"))
	}

	key := func(k string) string { return m.st.helpKey.Render(k) }
	help := []string{
		key("(A) ") + m.st.faint.Render("priority, at the start"),
		m.st.project.Render("+project ") + m.st.context.Render("@context ") + m.st.tag.Render("key:value ") + m.st.highlight.Render("{{highlight}}"),
		m.st.tag.Render("due:") + m.st.faint.Render("opens a calendar, or type today fri +3d 2026-12-01"),
		m.st.tag.Render("rec:") + m.st.faint.Render("1w 3d 5b 1m 1y · rec:+1w counts from due"),
		m.st.tag.Render("alarm:") + m.st.faint.Render("opens a date and time picker; rings like an alarm clock"),
		m.st.tag.Render("at:") + m.st.faint.Render("930 1400 3pm · on its due: day, or daily; agenda + alerts"),
		m.st.tag.Render("status:doing progress:40 ") + m.st.faint.Render("in progress · i > < in the list"),
		key("/name ") + m.st.faint.Render("run a Lua script from scripts/"),
		"",
		key("enter") + m.st.faint.Render(" save · ") + key("esc") + m.st.faint.Render(" cancel · ") +
			key("tab") + m.st.faint.Render(" complete/change ") + m.st.text.Underline(true).Render("this") + m.st.faint.Render(" · ") + key("↑↓") + m.st.faint.Render(" choose"),
	}
	lines = append(lines, m.st.faint.Render(strings.Repeat("─", w)))
	for _, l := range help {
		lines = append(lines, ansi.Truncate(l, w, "…"))
	}

	return m.st.paneFocused.Padding(0, 1).Width(w + 2).Render(strings.Join(lines, "\n"))
}

// wrapInput splits r into lines at most width cells wide, breaking after
// a space where it can (the space stays at the end of its line, so the
// cursor can sit on it) and inside a word only when it's longer than a
// line. It returns each line's start and end in runes; there's always at
// least one, and a last empty one when the cursor at the end of a full line
// needs somewhere to go.
func wrapInput(r []rune, width int) [][2]int {
	width = max(width, 1)
	var out [][2]int
	i := 0
	for {
		cells, j, brk := 0, i, -1
		for j < len(r) {
			cw := max(runewidth.RuneWidth(r[j]), 1)
			if cells+cw > width {
				break
			}
			cells += cw
			if r[j] == ' ' {
				brk = j + 1
			}
			j++
		}
		if j == len(r) {
			out = append(out, [2]int{i, j})
			if cells >= width { // no room left for the cursor
				out = append(out, [2]int{j, j})
			}
			return out
		}
		if brk > i {
			j = brk
		} else if j == i {
			j++ // a rune wider than the line
		}
		out = append(out, [2]int{i, j})
		i = j
	}
}

// viewWrappedInput draws the task being typed over as many lines as it
// takes, width cells each, with the cursor and the words tab can change
// underlined in their colours.
func (m Model) viewWrappedInput(width int) []string {
	r := []rune(m.input.Value())
	pos := clamp(m.input.Position(), 0, len(r))
	kind := make([]string, len(r))
	for _, w := range inputWords(m.input.Value()) {
		for i := w.start; i < w.end; i++ {
			kind[i] = w.kind
		}
	}
	style := func(k string) lipgloss.Style {
		switch k {
		case "project":
			return m.st.project.Underline(true)
		case "context":
			return m.st.context.Underline(true)
		case "tag":
			return m.st.tag.Underline(true)
		}
		return m.st.text
	}
	cursor := lipgloss.NewStyle().Reverse(true)
	var lines []string
	spans := wrapInput(r, width)
	for n, sp := range spans {
		var b strings.Builder
		for i := sp[0]; i < sp[1]; {
			if i == pos {
				b.WriteString(cursor.Render(string(r[i])))
				i++
				continue
			}
			j := i
			for j < sp[1] && j != pos && kind[j] == kind[i] {
				j++
			}
			b.WriteString(style(kind[i]).Render(string(r[i:j])))
			i = j
		}
		if pos == len(r) && n == len(spans)-1 {
			b.WriteString(cursor.Render(" "))
		}
		lines = append(lines, b.String())
	}
	return lines
}

// overlay draws fg on top of bg, centered.
func overlay(bg, fg string) string {
	bl := strings.Split(bg, "\n")
	fl := strings.Split(fg, "\n")
	bw := 0
	for _, l := range bl {
		bw = max(bw, ansi.StringWidth(l))
	}
	fw := lipgloss.Width(fg)
	x := max((bw-fw)/2, 0)
	y := max((len(bl)-len(fl))/2, 0)
	for i, f := range fl {
		r := y + i
		if r >= len(bl) {
			break
		}
		b := bl[r]
		left := ansi.Truncate(b, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		bl[r] = left + "\x1b[0m" + f + "\x1b[0m" + ansi.TruncateLeft(b, x+ansi.StringWidth(f), "")
	}
	return strings.Join(bl, "\n")
}
