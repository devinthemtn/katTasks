package ui

import "github.com/charmbracelet/lipgloss"

type ac = lipgloss.AdaptiveColor

var (
	cAccent   = ac{Light: "#6D4AE8", Dark: "#A78BFA"}
	cMuted    = ac{Light: "#8A8A8A", Dark: "#6E6E7A"}
	cText     = ac{Light: "#1F1F24", Dark: "#E6E6EA"}
	cSelBg    = ac{Light: "#E9E4FF", Dark: "#34304A"}
	cProject  = ac{Light: "#2E7D32", Dark: "#86D99A"}
	cContext  = ac{Light: "#0B6FA4", Dark: "#7CC4FA"}
	cRed      = ac{Light: "#C62828", Dark: "#FF7A7A"}
	cOrange   = ac{Light: "#D35400", Dark: "#FFB062"}
	cYellow   = ac{Light: "#9A7B00", Dark: "#F2D46B"}
	cBlueGray = ac{Light: "#546E7A", Dark: "#9FB3C8"}

	sTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(cAccent).Padding(0, 1)
	sFaint     = lipgloss.NewStyle().Foreground(cMuted)
	sText      = lipgloss.NewStyle().Foreground(cText)
	sProject   = lipgloss.NewStyle().Foreground(cProject)
	sContext   = lipgloss.NewStyle().Foreground(cContext)
	sTag       = lipgloss.NewStyle().Foreground(cMuted)
	sDone      = lipgloss.NewStyle().Foreground(cMuted).Strikethrough(true)
	sHeader    = lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	sError     = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sOK        = lipgloss.NewStyle().Foreground(cProject)
	sPrompt    = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	sSuggest   = lipgloss.NewStyle().Foreground(cMuted)
	sSuggestOn = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(cAccent).Padding(0, 1)

	sPane        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMuted)
	sPaneFocused = sPane.BorderForeground(cAccent)
)

func priorityStyle(p byte) lipgloss.Style {
	switch p {
	case 'A':
		return lipgloss.NewStyle().Foreground(cRed).Bold(true)
	case 'B':
		return lipgloss.NewStyle().Foreground(cOrange).Bold(true)
	case 'C':
		return lipgloss.NewStyle().Foreground(cYellow).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(cBlueGray)
	}
}
