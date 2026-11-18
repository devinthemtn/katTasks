package ui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"katTasks/internal/todo"
)

const (
	headerHeight = 1
	footerHeight = 2
)

func (m *Model) bodyHeight() int {
	h := m.height - headerHeight - footerHeight
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) sidebarWidth() int {
	w := m.width / 4
	return clamp(w, 20, 32)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.viewHeader(), m.viewBody(), m.viewFooter())
}

func (m Model) viewHeader() string {
	title := sTitle.Render("katTasks") + " "
	pending := 0
	for _, t := range m.tasks {
		if !t.Done {
			pending++
		}
	}
	parts := []string{fmt.Sprintf("%d pending", pending), "sort: " + sortModes[m.sortIdx]}
	if m.hideDone {
		parts = append(parts, "done hidden")
	}
	if m.filter != "" && m.mode != modeFilter {
		parts = append(parts, fmt.Sprintf("filter: %q", m.filter))
	}
	right := sFaint.Render(strings.Join(parts, " · ") + " ")
	path := m.cfg.TodoFile
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+"/") {
		path = "~" + path[len(home):]
	}
	room := m.width - lipgloss.Width(title) - lipgloss.Width(right) - 1
	if room < 4 {
		return ansi.Truncate(title+right, m.width, "…")
	}
	if r := []rune(path); len(r) > room {
		path = "…" + string(r[len(r)-room+1:])
	}
	left := title + sFaint.Render(path)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", max(gap, 0)) + right
}

func (m Model) viewBody() string {
	h := m.bodyHeight()
	sw := m.sidebarWidth()
	lw := m.width - sw
	inner := h - 2

	sideStyle, listStyle := sPane, sPaneFocused
	if m.focus == focusSidebar {
		sideStyle, listStyle = sPaneFocused, sPane
	}
	side := sideStyle.Width(sw - 2).Height(inner).Render(m.viewSidebar(sw-2, inner))

	var main string
	if m.showHelp {
		m.help.ShowAll = true
		m.help.Width = lw - 4
		main = lipgloss.NewStyle().Padding(0, 1).Render(
			sHeader.Render("KEYS") + "\n\n" + m.help.View(m.keys) + "\n\n" + m.inputHelp() +
				"\n\n" + sFaint.Render("press any key to close"))
	} else {
		main = m.viewList(lw-2, inner)
	}
	list := listStyle.Width(lw - 2).Height(inner).Render(clipLines(main, inner))
	return lipgloss.JoinHorizontal(lipgloss.Top, side, list)
}

func (m Model) inputHelp() string {
	return sHeader.Render("WHILE TYPING") + "\n\n" + sFaint.Render(strings.Join([]string{
		"tab        complete +project / @context / due:",
		"↑/↓        choose suggestion",
		"enter      save        esc  cancel",
		"due:       today, tomorrow, mon..sun, +3d, +2w, +1m",
	}, "\n"))
}

func (m Model) viewSidebar(w, h int) string {
	var lines []string
	end := min(len(m.side), m.sideOffset+h)
	for i := m.sideOffset; i < end; i++ {
		e := m.side[i]
		if e.kind == entryHeader {
			lines = append(lines, sHeader.Render(" "+e.value))
			continue
		}
		label := e.label()
		count := fmt.Sprint(e.count)
		labelW := w - len(count) - 3
		label = truncate(label, max(labelW, 1))
		var ls lipgloss.Style
		switch e.kind {
		case entryProject:
			ls = sProject
		case entryContext:
			ls = sContext
		default:
			ls = sText
		}
		cs := sFaint
		pad := w - 1 - lipgloss.Width(label) - len(count) - 1
		marker := " "
		bg := lipgloss.NewStyle()
		if i == m.sideCursor {
			marker = "▌"
			if m.focus == focusSidebar {
				bg = bg.Background(cSelBg)
				ls, cs = ls.Background(cSelBg).Bold(true), cs.Background(cSelBg)
			} else {
				ls = ls.Bold(true)
			}
		}
		line := sPrompt.Inherit(bg).Render(marker) + ls.Render(label) +
			bg.Render(strings.Repeat(" ", max(pad, 1))) + cs.Render(count) + bg.Render(" ")
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewList(w, h int) string {
	if len(m.visible) == 0 {
		msg := "No tasks. Press a to add one."
		if m.filter != "" || m.sideCursor != 0 {
			msg = "No matching tasks. Press esc to clear the filter."
		}
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, sFaint.Render(msg))
	}
	var lines []string
	end := min(len(m.visible), m.offset+h)
	now := time.Now()
	for i := m.offset; i < end; i++ {
		lines = append(lines, renderTask(m.visible[i], i == m.cursor, m.focus == focusList, w, now))
	}
	return strings.Join(lines, "\n")
}

func renderTask(t *todo.Task, selected, focused bool, w int, now time.Time) string {
	on := func(s lipgloss.Style) lipgloss.Style {
		if selected && focused {
			return s.Background(cSelBg)
		}
		return s
	}
	marker := on(sPrompt).Render(" ")
	if selected {
		marker = on(sPrompt).Render("▌")
	}

	box := on(sFaint).Render("[ ] ")
	if t.Done {
		box = on(sOK).Render("[x] ")
	}

	pri := on(sText).Render("    ")
	if t.Priority != 0 {
		pri = on(priorityStyle(t.Priority)).Render("(" + string(t.Priority) + ") ")
	}

	badge := ""
	if d, ok := t.Due(); ok {
		label, st := dueLabel(d, now)
		if t.Done {
			st = sFaint
		}
		badge = on(st).Render(" " + label + " ")
	}

	// Description, coloring tokens. The due: tag is shown as the badge instead.
	var parts []string
	for _, word := range strings.Fields(t.Text) {
		st := sText
		switch {
		case strings.HasPrefix(word, "due:") && badge != "":
			continue
		case t.Done:
			st = sDone
		case len(word) > 1 && word[0] == '+':
			st = sProject
		case len(word) > 1 && word[0] == '@':
			st = sContext
		case isTag(word):
			st = sTag
		}
		parts = append(parts, on(st).Render(word))
	}
	text := strings.Join(parts, on(sText).Render(" "))

	prefix := marker + box + pri
	avail := w - lipgloss.Width(prefix) - lipgloss.Width(badge)
	if avail < 4 {
		avail = 4
	}
	if lipgloss.Width(text) > avail {
		text = ansi.Truncate(text, avail-1, "") + on(sFaint).Render("…")
	}
	pad := w - lipgloss.Width(prefix) - lipgloss.Width(text) - lipgloss.Width(badge)
	return prefix + text + on(sText).Render(strings.Repeat(" ", max(pad, 0))) + badge
}

func isTag(w string) bool {
	k, v, ok := strings.Cut(w, ":")
	return ok && k != "" && v != "" && !strings.HasPrefix(v, "//")
}

func dueLabel(d, now time.Time) (string, lipgloss.Style) {
	y, mo, dd := now.Date()
	today := time.Date(y, mo, dd, 0, 0, 0, 0, time.Local)
	days := int(math.Round(d.Sub(today).Hours() / 24))
	switch {
	case days < 0:
		return fmt.Sprintf("overdue %dd", -days), lipgloss.NewStyle().Foreground(cRed).Bold(true)
	case days == 0:
		return "today", lipgloss.NewStyle().Foreground(cOrange).Bold(true)
	case days == 1:
		return "tomorrow", lipgloss.NewStyle().Foreground(cYellow)
	case days < 7:
		return d.Format("Mon"), lipgloss.NewStyle().Foreground(cYellow)
	case d.Year() == today.Year():
		return d.Format("Jan 2"), sFaint
	default:
		return d.Format("2006-01-02"), sFaint
	}
}

func (m Model) viewFooter() string {
	var l1, l2 string
	switch m.mode {
	case modeAdd, modeEdit, modeFilter:
		label := map[mode]string{modeAdd: "add", modeEdit: "edit", modeFilter: "filter"}[m.mode]
		l1 = sPrompt.Render(" "+label+" › ") + m.input.View()
		if len(m.sugg) > 0 {
			var ss []string
			for i, s := range m.sugg {
				if i == m.suggIdx {
					ss = append(ss, sSuggestOn.Render(s))
				} else {
					ss = append(ss, sSuggest.Render(" "+s+" "))
				}
			}
			l2 = " " + strings.Join(ss, "") + sFaint.Render("   tab complete · ↑↓ choose")
		} else if m.mode == modeFilter {
			l2 = sFaint.Render(" type to filter · enter keep · esc clear")
		} else {
			l2 = sFaint.Render(" enter save · esc cancel · tab completes +project @context due:")
		}
	case modeConfirm:
		l1 = sPrompt.Render(" " + m.confirm)
		l2 = ""
	case modePriority:
		l1 = sPrompt.Render(" priority › ") + sFaint.Render("press A–Z, space to clear, esc to cancel")
	default:
		if m.status != "" {
			st := sOK
			if m.statusErr {
				st = sError
			}
			l1 = " " + st.Render(m.status)
		}
		m.help.ShowAll = false
		l2 = " " + m.help.View(m.keys)
	}
	return ansi.Truncate(l1, m.width, "…") + "\n" + ansi.Truncate(l2, m.width, "…")
}

func clipLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
