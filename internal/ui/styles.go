package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/devinthemtn/katTasks/internal/theme"
)

// styles are the lipgloss styles for one theme.
type styles struct {
	title, faint, text, project, context, tag, highlight, link, done, header lipgloss.Style
	err, ok, prompt, helpKey, suggest, suggestOn                             lipgloss.Style
	pane, paneFocused                                                        lipgloss.Style
	due, today, overdue                                                      lipgloss.Style
	pri                                                                      [5]lipgloss.Style // A, B, C, D, E–Z

	cursorBg, selectedBg, matchBg lipgloss.TerminalColor
}

func termColor(c theme.Color) lipgloss.TerminalColor {
	switch {
	case c.Light == "" && c.Dark == "":
		return lipgloss.NoColor{}
	case c.Light == c.Dark:
		return lipgloss.Color(c.Dark)
	}
	return lipgloss.AdaptiveColor{Light: c.Light, Dark: c.Dark}
}

func newStyles(t theme.Theme) styles {
	c := func(k string) lipgloss.TerminalColor { return termColor(t.Get(k)) }
	fg := func(k string) lipgloss.Style { return lipgloss.NewStyle().Foreground(c(k)) }
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c("border"))
	return styles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(c("title_fg")).Background(c("title_bg")).Padding(0, 1),
		faint:     fg("dim"),
		text:      fg("fg"),
		project:   fg("project"),
		context:   fg("context"),
		tag:       fg("tag"),
		highlight: fg("highlight"),
		link:      fg("link"),
		done:      fg("done").Strikethrough(true),
		header:    fg("dim").Bold(true),
		err:       fg("error").Bold(true),
		ok:        fg("ok"),
		prompt:    fg("accent").Bold(true),
		helpKey:   fg("fg").Bold(true),
		suggest:   fg("dim"),
		suggestOn: lipgloss.NewStyle().Foreground(c("title_fg")).Background(c("accent")).Padding(0, 1),

		pane:        pane,
		paneFocused: pane.BorderForeground(c("border_focus")),

		due:     fg("due"),
		today:   fg("today").Bold(true),
		overdue: fg("overdue").Bold(true),
		pri: [5]lipgloss.Style{
			fg("pri_a").Bold(true), fg("pri_b").Bold(true), fg("pri_c").Bold(true), fg("pri_d"), fg("pri_other"),
		},

		cursorBg:   c("cursor_bg"),
		selectedBg: c("selected_bg"),
		matchBg:    c("match_bg"),
	}
}

func (st styles) priority(p byte) lipgloss.Style {
	if p >= 'A' && p <= 'D' {
		return st.pri[p-'A']
	}
	return st.pri[4]
}
