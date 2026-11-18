package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// viewTaskPopup renders the centered box used to add or edit a task: the
// input, any completions, and a short reminder of the todo.txt syntax.
func (m Model) viewTaskPopup(maxW int) string {
	w := clamp(maxW-4, 20, 64) // content width inside border and padding
	title := "New task"
	if m.mode == modeEdit {
		title = "Edit task"
	}

	in := m.input
	in.Width = w - 3 // "› " prompt plus the cursor cell
	lines := []string{
		m.st.prompt.Render(title),
		"",
		m.st.prompt.Render("› ") + in.View(),
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
		lines = append(lines, ansi.Truncate(strings.Join(ss, ""), w, "…"))
	} else {
		lines = append(lines, "")
	}

	key := func(k string) string { return m.st.helpKey.Render(k) }
	help := []string{
		key("(A) ") + m.st.faint.Render("priority, at the start"),
		m.st.project.Render("+project ") + m.st.context.Render("@context ") + m.st.tag.Render("key:value"),
		m.st.tag.Render("due:") + m.st.faint.Render("today tomorrow fri +3d +2w +1m 2026-12-01"),
		m.st.tag.Render("rec:") + m.st.faint.Render("1w 3d 5b 1m 1y · rec:+1w counts from due"),
		"",
		key("enter") + m.st.faint.Render(" save · ") + key("esc") + m.st.faint.Render(" cancel · ") +
			key("tab") + m.st.faint.Render(" complete · ") + key("↑↓") + m.st.faint.Render(" choose"),
	}
	lines = append(lines, m.st.faint.Render(strings.Repeat("─", w)))
	for _, l := range help {
		lines = append(lines, ansi.Truncate(l, w, "…"))
	}

	return m.st.paneFocused.Padding(0, 1).Width(w + 2).Render(strings.Join(lines, "\n"))
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
