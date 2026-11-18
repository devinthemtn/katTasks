package ui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/devinthemtn/katTasks/internal/todo"
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
	title := m.st.title.Render("katTasks") + " "
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
	right := m.st.faint.Render(strings.Join(parts, " · ") + " ")
	path := m.cfg.Files.Todo
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
	left := title + m.st.faint.Render(path)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", max(gap, 0)) + right
}

func (m Model) viewBody() string {
	h := m.bodyHeight()
	sw := m.sidebarWidth()
	lw := m.width - sw
	inner := h - 2

	sideStyle, listStyle := m.st.pane, m.st.paneFocused
	if m.focus == focusSidebar {
		sideStyle, listStyle = m.st.paneFocused, m.st.pane
	}
	side := sideStyle.Width(sw - 2).Height(inner).Render(m.viewSidebar(sw-2, inner))

	var main string
	if m.mode == modeTheme {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewThemePicker())
	} else if m.showHelp {
		main = lipgloss.NewStyle().Padding(0, 1).Render(
			m.st.header.Render("KEYS") + "\n\n" + m.fullHelp(lw-4) + "\n\n" + m.inputHelp() +
				"\n\n" + m.st.faint.Render("press any key to close"))
	} else {
		main = m.viewList(lw-2, inner)
	}
	list := listStyle.Width(lw - 2).Height(inner).Render(clipLines(main, inner))
	return lipgloss.JoinHorizontal(lipgloss.Top, side, list)
}

func (m Model) inputHelp() string {
	return m.st.header.Render("WHILE TYPING") + "\n\n" + m.st.faint.Render(strings.Join([]string{
		"tab        complete +project / @context / due:",
		"↑/↓        choose suggestion",
		"enter      save        esc  cancel",
		"due:       today, tomorrow, mon..sun, +3d, +2w, +1m",
		"rec:       1d 2w 1m 1y 5b (b = business days); rec:+1w = from due date",
	}, "\n"))
}

func (m Model) viewSidebar(w, h int) string {
	var lines []string
	end := min(len(m.side), m.sideOffset+h)
	for i := m.sideOffset; i < end; i++ {
		e := m.side[i]
		if e.kind == entryHeader {
			lines = append(lines, m.st.header.Render(" "+e.value))
			continue
		}
		label := e.label()
		count := fmt.Sprint(e.count)
		labelW := w - len(count) - 3
		label = truncate(label, max(labelW, 1))
		var ls lipgloss.Style
		switch e.kind {
		case entryProject:
			ls = m.st.project
		case entryContext:
			ls = m.st.context
		default:
			ls = m.st.text
		}
		cs := m.st.faint
		pad := w - 1 - lipgloss.Width(label) - len(count) - 1
		marker := " "
		bg := lipgloss.NewStyle()
		if i == m.sideCursor {
			marker = "▌"
			if m.focus == focusSidebar {
				bg = bg.Background(m.st.cursorBg)
				ls, cs = ls.Background(m.st.cursorBg).Bold(true), cs.Background(m.st.cursorBg)
			} else {
				ls = ls.Bold(true)
			}
		}
		line := m.st.prompt.Inherit(bg).Render(marker) + ls.Render(label) +
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
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, m.st.faint.Render(msg))
	}
	var lines []string
	end := min(len(m.visible), m.offset+h)
	now := time.Now()
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.renderTask(m.visible[i], i == m.cursor, m.focus == focusList, w, now))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTask(t *todo.Task, selected, focused bool, w int, now time.Time) string {
	on := func(s lipgloss.Style) lipgloss.Style {
		if selected && focused {
			return s.Background(m.st.cursorBg)
		}
		return s
	}
	marker := on(m.st.prompt).Render(" ")
	if selected {
		marker = on(m.st.prompt).Render("▌")
	}

	box := on(m.st.faint).Render("[ ] ")
	if t.Done {
		box = on(m.st.ok).Render("[x] ")
	}

	pri := on(m.st.text).Render("    ")
	if t.Priority != 0 {
		pri = on(m.st.priority(t.Priority)).Render("(" + string(t.Priority) + ") ")
	}

	badge := ""
	if _, ok := t.Recurrence(); ok && !t.Done {
		badge = on(m.st.context).Render(" ↻")
	}
	if d, ok := t.Due(); ok {
		label, st := m.st.dueLabel(d, now)
		if t.Done {
			st = m.st.faint
		}
		badge += on(st).Render(" " + label + " ")
	} else if badge != "" {
		badge += on(m.st.text).Render(" ")
	}

	_, hasDue := t.Due()

	// Description, coloring tokens. The due: tag is shown as the badge instead.
	var parts []string
	for _, word := range strings.Fields(t.Text) {
		st := m.st.text
		switch {
		case strings.HasPrefix(word, "due:") && hasDue:
			continue
		case t.Done:
			st = m.st.done
		case len(word) > 1 && word[0] == '+':
			st = m.st.project
		case len(word) > 1 && word[0] == '@':
			st = m.st.context
		case isTag(word):
			st = m.st.tag
		}
		parts = append(parts, on(st).Render(word))
	}
	text := strings.Join(parts, on(m.st.text).Render(" "))

	prefix := marker + box + pri
	avail := w - lipgloss.Width(prefix) - lipgloss.Width(badge)
	if avail < 4 {
		avail = 4
	}
	if lipgloss.Width(text) > avail {
		text = ansi.Truncate(text, avail-1, "") + on(m.st.faint).Render("…")
	}
	pad := w - lipgloss.Width(prefix) - lipgloss.Width(text) - lipgloss.Width(badge)
	return prefix + text + on(m.st.text).Render(strings.Repeat(" ", max(pad, 0))) + badge
}

func isTag(w string) bool {
	k, v, ok := strings.Cut(w, ":")
	return ok && k != "" && v != "" && !strings.HasPrefix(v, "//")
}

func (st styles) dueLabel(d, now time.Time) (string, lipgloss.Style) {
	y, mo, dd := now.Date()
	today := time.Date(y, mo, dd, 0, 0, 0, 0, time.Local)
	days := int(math.Round(d.Sub(today).Hours() / 24))
	switch {
	case days < 0:
		return fmt.Sprintf("overdue %dd", -days), st.overdue
	case days == 0:
		return "today", st.today
	case days == 1:
		return "tomorrow", st.due
	case days < 7:
		return d.Format("Mon"), st.due
	case d.Year() == today.Year():
		return d.Format("Jan 2"), st.faint
	default:
		return d.Format("2006-01-02"), st.faint
	}
}

func (m Model) viewFooter() string {
	var l1, l2 string
	switch m.mode {
	case modeAdd, modeEdit, modeFilter:
		label := map[mode]string{modeAdd: "add", modeEdit: "edit", modeFilter: "filter"}[m.mode]
		l1 = m.st.prompt.Render(" "+label+" › ") + m.input.View()
		if len(m.sugg) > 0 {
			var ss []string
			for i, s := range m.sugg {
				if i == m.suggIdx {
					ss = append(ss, m.st.suggestOn.Render(s))
				} else {
					ss = append(ss, m.st.suggest.Render(" "+s+" "))
				}
			}
			l2 = " " + strings.Join(ss, "") + m.st.faint.Render("   tab complete · ↑↓ choose")
		} else if m.mode == modeFilter {
			l2 = m.st.faint.Render(" type to filter · enter keep · esc clear")
		} else {
			l2 = m.st.faint.Render(" enter save · esc cancel · tab completes +project @context due:")
		}
	case modeConfirm:
		l1 = m.st.prompt.Render(" " + m.confirm)
		l2 = ""
	case modePriority:
		l1 = m.st.prompt.Render(" priority › ") + m.st.faint.Render("press A–Z, space to clear, esc to cancel")
	case modeTheme:
		l1 = m.st.prompt.Render(" theme › ") + m.st.text.Render(m.theme)
	default:
		if m.status != "" {
			st := m.st.ok
			if m.statusErr {
				st = m.st.err
			}
			l1 = " " + st.Render(m.status)
		} else if m.configWarn != "" {
			l1 = " " + m.st.err.Render("⚠ "+m.configWarn) + m.st.faint.Render("  (fix, then ctrl+r)")
		}
		l2 = " " + m.shortHelp()
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

// shortHelp is the one-line key summary in the footer.
func (m Model) shortHelp() string {
	var parts []string
	for _, name := range shortHelpActions {
		ks := m.km.keysFor(name)
		if len(ks) == 0 {
			continue
		}
		parts = append(parts, m.st.helpKey.Render(ks[0])+" "+m.st.faint.Render(shortLabel(name)))
	}
	return strings.Join(parts, m.st.faint.Render(" • "))
}

// fullHelp lists every bound action, one column per group.
func (m Model) fullHelp(w int) string {
	var cols []string
	for _, g := range []string{groupMove, groupEdit, groupView} {
		type row struct{ k, d string }
		var rows []row
		kw := 0
		for _, a := range actions {
			if a.group != g {
				continue
			}
			ks := m.km.keysFor(a.name)
			if len(ks) == 0 {
				continue
			}
			k := strings.Join(ks, " ")
			kw = max(kw, lipgloss.Width(k))
			rows = append(rows, row{k, a.desc})
		}
		lines := []string{m.st.header.Render(g)}
		for _, r := range rows {
			lines = append(lines, m.st.helpKey.Render(r.k)+strings.Repeat(" ", kw-lipgloss.Width(r.k)+2)+m.st.faint.Render(r.d))
		}
		cols = append(cols, strings.Join(lines, "\n"))
	}
	gap := "   "
	if j := lipgloss.JoinHorizontal(lipgloss.Top, cols[0], gap, cols[1], gap, cols[2]); lipgloss.Width(j) <= w {
		return j
	}
	if j := lipgloss.JoinHorizontal(lipgloss.Top, cols[0], gap, cols[1]); lipgloss.Width(j) <= w {
		return j + "\n\n" + cols[2]
	}
	return strings.Join(cols, "\n\n")
}

// shortLabel is an action's footer label.
func shortLabel(name string) string {
	switch name {
	case "add", "edit", "filter", "help", "quit":
		return name
	case "toggle_done":
		return "done"
	case "set_priority":
		return "priority"
	case "cycle_sort":
		return "sort"
	case "switch_pane":
		return "pane"
	case "palette":
		return "commands"
	}
	return actionByName[name].desc
}
