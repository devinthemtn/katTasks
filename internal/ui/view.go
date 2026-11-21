package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/devinthemtn/katTasks/internal/listview"
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
	body := m.viewBody()
	if m.mode == modeAdd || m.mode == modeEdit {
		body = overlay(body, m.viewTaskPopup())
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.viewHeader(), body, m.viewFooter())
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
	if m.mode == modePalette {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewPalette(lw-4, inner))
	} else if m.mode == modeTheme {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewThemePicker())
	} else if m.showHelp {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewHelp(lw-4, inner))
	} else if m.agenda {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewAgenda(lw-4, inner))
	} else if m.calendar {
		main = lipgloss.NewStyle().Padding(0, 1).Render(m.viewCalendar(lw-4, inner))
	} else {
		main = m.viewList(lw-2, inner)
	}
	list := listStyle.Width(lw - 2).Height(inner).Render(clipLines(main, inner))
	return lipgloss.JoinHorizontal(lipgloss.Top, side, list)
}

// typingHelp and tagsHelp are the help screen's text sections, shared with
// katTasks-gui through the snapshot.
var (
	typingHelp = []string{
		"tab        complete +project / @context / due: / rec: / at:",
		"↑/↓        choose suggestion",
		"enter      save        esc  cancel",
		"due:       today, tomorrow, mon..sun, +3d, +2w, +1m",
		"rec:       1d 2w 1m 1y 5b (b = business days); rec:+1w = from due date",
		"at:        930 1400 3pm; on the due: day, or every day without one",
		"alarm:     opens a picker: a date and time that rings like an alarm clock",
		"/name      run a Lua script from scripts/ (tab, or when saving)",
	}
	tagsHelp = []string{
		"status:doing progress:40   in progress; i, > and < set them",
		"at:1430                    a time: the agenda, and an alert ([alerts])",
		"alarm:2026-12-01T0730      rings like an alarm clock ([alarms])",
		"search     words +proj @ctx due:+1w due:overdue pri:A-C -term",
		"           status:doing status:any progress:>50 progress:none",
		"commands   katTasks alarm +5m Tea · katTasks alert 3 in 20m",
		"           katTasks alerts (what's coming) · katTasks -h for all",
		"remind     katTasks remind -test / -test-alarm checks alerts;",
		"           -install sends them at login, with katTasks closed",
	}
)

// helpLines is the whole help screen, w cells wide, one line each.
func (m Model) helpLines(w int) []string {
	section := func(title string, lines []string) string {
		return m.st.header.Render(title) + "\n\n" + m.st.faint.Render(strings.Join(lines, "\n"))
	}
	s := m.st.header.Render("KEYS") + m.st.faint.Render("   katTasks "+m.opts.Version) + "\n\n" + m.fullHelp(w) +
		"\n\n" + section("WHILE TYPING", typingHelp) + "\n\n" + section("TAGS & SEARCH", tagsHelp)
	return strings.Split(s, "\n")
}

// viewHelp draws the help screen in h lines, from m.helpScroll on. When it
// doesn't all fit, the last line says how to scroll.
func (m Model) viewHelp(w, h int) string {
	lines := m.helpLines(w)
	footer := m.st.faint.Render("press any key to close")
	if len(lines)+2 <= h {
		return strings.Join(lines, "\n") + "\n\n" + footer
	}
	rows := max(h-1, 1)
	top := clamp(m.helpScroll, 0, max(len(lines)-rows, 0))
	more := "↓ more"
	switch {
	case top+rows >= len(lines):
		more = "↑ the end"
	case top > 0:
		more = "↕ more"
	}
	footer = m.st.prompt.Render(more) + m.st.faint.Render(" · j/k ↑↓ pgup/pgdn g/G scroll · any other key closes")
	return strings.Join(lines[top:min(top+rows, len(lines))], "\n") + "\n" + ansi.Truncate(footer, w, "…")
}

func (m Model) viewSidebar(w, h int) string {
	var lines []string
	end := min(len(m.side), m.sideOffset+h)
	for i := m.sideOffset; i < end; i++ {
		e := m.side[i]
		if e.Kind == entryHeader {
			lines = append(lines, m.st.header.Render(" "+e.Value))
			continue
		}
		label := e.Label()
		count := fmt.Sprint(e.Count)
		labelW := w - len(count) - 3
		label = truncate(label, max(labelW, 1))
		var ls lipgloss.Style
		switch e.Kind {
		case entryProject:
			ls = m.st.project
		case entryContext:
			ls = m.st.context
		case entrySaved:
			ls = m.st.prompt.UnsetBold()
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
	var nums map[*todo.Task]int
	numW := 0
	if m.lineNums {
		nums = make(map[*todo.Task]int, len(m.tasks))
		for i, t := range m.tasks {
			nums[t] = i + 1
		}
		numW = len(fmt.Sprint(len(m.tasks)))
	}
	hide := map[string]bool{}
	for _, k := range m.cfg.View.HideKeys {
		hide[k] = true
	}
	for i := m.offset; i < end; i++ {
		t := m.visible[i]
		r := row{cursor: i == m.cursor, focused: m.focus == focusList, marked: m.sel[t], num: nums[t], numW: numW, hide: hide}
		lines = append(lines, m.renderTask(t, r, w, now))
	}
	return strings.Join(lines, "\n")
}

// row says how one task line is drawn.
type row struct {
	cursor, focused, marked bool
	num, numW               int // task number and its column width; 0 = off
	hide                    map[string]bool
}

func (m Model) renderTask(t *todo.Task, r row, w int, now time.Time) string {
	on := m.rowStyle(r)
	prefix := m.taskPrefix(t, r, on)
	badge := m.taskBadges(t, now, on)
	text := m.taskWords(t, r, on)

	avail := max(w-lipgloss.Width(prefix)-lipgloss.Width(badge), 4)
	if lipgloss.Width(text) > avail {
		text = ansi.Truncate(text, avail-1, "") + on(m.st.faint).Render("…")
	}
	pad := w - lipgloss.Width(prefix) - lipgloss.Width(text) - lipgloss.Width(badge)
	return prefix + text + on(m.st.text).Render(strings.Repeat(" ", max(pad, 0))) + badge
}

// rowStyle returns what a style becomes on the row: with the cursor's
// background on the cursor row, or the selection's on a marked one.
func (m Model) rowStyle(r row) func(lipgloss.Style) lipgloss.Style {
	return func(s lipgloss.Style) lipgloss.Style {
		switch {
		case r.cursor && r.focused:
			return s.Background(m.st.cursorBg)
		case r.marked:
			return s.Background(m.st.selectedBg)
		}
		return s
	}
}

// taskPrefix is the start of a row: the cursor or mark, the task number if
// shown, the checkbox and the priority.
func (m Model) taskPrefix(t *todo.Task, r row, on func(lipgloss.Style) lipgloss.Style) string {
	marker := on(m.st.prompt).Render(" ")
	switch {
	case r.cursor:
		marker = on(m.st.prompt).Render("▌")
	case r.marked:
		marker = on(m.st.prompt).Render("•")
	}
	if r.numW > 0 {
		marker += on(m.st.faint).Render(fmt.Sprintf("%*d ", r.numW, r.num))
	}

	box := on(m.st.faint).Render("[ ] ")
	if t.Done {
		box = on(m.st.ok).Render("[x] ")
	}

	pri := on(m.st.text).Render("    ")
	if t.Priority != 0 {
		pri = on(m.st.priority(t.Priority)).Render("(" + string(t.Priority) + ") ")
	}
	return marker + box + pri
}

// taskBadges is the end of a row: status, progress, alarm, recurrence and
// due date.
func (m Model) taskBadges(t *todo.Task, now time.Time, on func(lipgloss.Style) lipgloss.Style) string {
	badge := ""
	status, doing, progress := listview.WorkLabels(t)
	if status != "" {
		st := m.st.faint
		if doing {
			st = m.st.due
		}
		badge += on(st).Render(" " + status)
	}
	if progress != "" {
		badge += on(m.st.context).Render(" " + progress)
	}
	if label, past := listview.AlarmLabel(t, now); label != "" {
		st := m.st.today
		if past {
			st = m.st.faint
		}
		badge += on(st).Render(" " + label)
	}
	if _, ok := t.Recurrence(); ok && !t.Done {
		badge += on(m.st.context).Render(" ↻")
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
	return badge
}

// taskWords is the task's text with its words colored: projects, contexts,
// tags and search matches. Tags shown as badges, and hidden ones, are left
// out.
func (m Model) taskWords(t *todo.Task, r row, on func(lipgloss.Style) lipgloss.Style) string {
	_, hasDue := t.Due()
	var parts []string
	for _, word := range strings.Fields(t.Text) {
		st := m.st.text
		switch {
		case strings.HasPrefix(word, "due:") && hasDue:
			continue
		case !t.Done && (todo.IsWorkTag(word) || todo.IsAlarmTag(word)): // shown as a badge
			continue
		case t.Done:
			st = m.st.done
		case len(word) > 1 && word[0] == '+':
			st = m.st.project
		case len(word) > 1 && word[0] == '@':
			st = m.st.context
		case isTag(word):
			if k, _, _ := strings.Cut(word, ":"); r.hide[strings.ToLower(k)] {
				continue
			}
			st = m.st.tag
		}
		st = on(st)
		if !t.Done && m.q.Highlights(word) {
			st = st.Background(m.st.matchBg)
		}
		parts = append(parts, st.Render(word))
	}
	return strings.Join(parts, on(m.st.text).Render(" "))
}

func isTag(w string) bool { return listview.IsTag(w) }

func (st styles) dueLabel(d, now time.Time) (string, lipgloss.Style) {
	label, kind := listview.DueLabel(d, now)
	return label, map[listview.DueKind]lipgloss.Style{
		listview.DueOverdue: st.overdue, listview.DueToday: st.today, listview.DueSoon: st.due, listview.DueLater: st.faint,
	}[kind]
}

func (m Model) viewFooter() string {
	var l1, l2 string
	switch m.mode {
	case modeAdd, modeEdit:
		// The popup in the middle of the screen has the input and its help.
		l1 = m.st.prompt.Render(map[mode]string{modeAdd: " add task", modeEdit: " edit task"}[m.mode])
		l2 = m.st.faint.Render(" enter save · esc cancel")
	case modeFilter, modeSaveName:
		label := map[mode]string{modeFilter: "filter", modeSaveName: "save search as"}[m.mode]
		l1 = m.st.prompt.Render(" "+label+" › ") + m.input.View()
		if len(m.sugg) > 0 {
			l2 = " " + m.suggestionChips() + m.st.faint.Render("   tab complete · ↑↓ choose")
		} else if hint := m.inputHint(); hint != "" {
			l2 = m.st.faint.Render(" " + hint)
		}
	case modeConfirm:
		l1 = m.st.prompt.Render(" " + m.confirm)
		yes, no := m.st.suggest.Render(" y yes "), m.st.suggest.Render(" n no ")
		if m.confirmNo {
			no = m.st.suggestOn.Render("n no")
		} else {
			yes = m.st.suggestOn.Render("y yes")
		}
		l2 = " " + yes + "  " + no + m.st.faint.Render("   ←→ choose · enter ok")
	default:
		// The same lines as the desktop app's footer.
		segs := m.footerSegs()
		l1, l2 = m.footerLine(segs[0]), m.footerLine(segs[1])
	}
	return ansi.Truncate(l1, m.width, "…") + "\n" + ansi.Truncate(l2, m.width, "…")
}

// suggestionChips are the completions, the highlighted one picked out.
func (m Model) suggestionChips() string {
	var ss []string
	for i, s := range m.sugg {
		if i == m.suggIdx {
			ss = append(ss, m.st.suggestOn.Render(s))
		} else {
			ss = append(ss, m.st.suggest.Render(" "+s+" "))
		}
	}
	return strings.Join(ss, "")
}

// footerLine draws one line of footerSegs in the terminal's styles,
// indented one space unless it starts with a chip, which is padded.
func (m Model) footerLine(segs []Seg) string {
	if len(segs) == 0 {
		return ""
	}
	var b strings.Builder
	if segs[0].Style != "chip" {
		b.WriteByte(' ')
	}
	for _, sg := range segs {
		if st, ok := m.segStyles()[sg.Style]; ok {
			b.WriteString(st.Render(sg.Text))
		} else {
			b.WriteString(sg.Text)
		}
	}
	return b.String()
}

// segStyles are the terminal styles for footerSegs' style names.
func (m Model) segStyles() map[string]lipgloss.Style {
	return map[string]lipgloss.Style{
		"prompt": m.st.prompt, "dim": m.st.faint, "fg": m.st.text, "ok": m.st.ok,
		"error": m.st.err, "chip": m.st.suggestOn, "key": m.st.helpKey,
	}
}

func clipLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
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
	// MOVE is the shortest column, so VIEW goes under it.
	if j := lipgloss.JoinHorizontal(lipgloss.Top, cols[0]+"\n\n"+cols[2], gap, cols[1]); lipgloss.Width(j) <= w {
		return j
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

func widthOf(s string) int { return lipgloss.Width(s) }

func truncateANSI(s string, n int) string { return ansi.Truncate(s, max(n, 0), "…") }

// scrollHelp moves the help screen for a scrolling key and reports whether
// k was one; any other key closes the help.
func (m *Model) scrollHelp(k string) bool {
	rows := max(m.listRows()-1, 1) // as viewHelp shows it
	sw := m.sidebarWidth()
	total := len(m.helpLines(m.width - sw - 4))
	if total+2 <= m.listRows() {
		return false // it all fits; nothing to scroll
	}
	last := max(total-rows, 0)
	switch k {
	case "j", "down":
		m.helpScroll++
	case "k", "up":
		m.helpScroll--
	case "ctrl+d", "pgdown", " ":
		m.helpScroll += rows / 2
	case "ctrl+u", "pgup":
		m.helpScroll -= rows / 2
	case "g", "home":
		m.helpScroll = 0
	case "G", "end":
		m.helpScroll = last
	default:
		return false
	}
	m.helpScroll = clamp(m.helpScroll, 0, last)
	return true
}
