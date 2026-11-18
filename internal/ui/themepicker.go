package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
)

func (m *Model) openThemePicker() {
	m.mode = modeTheme
	m.pickPrev = m.theme
	m.pickIdx = 0
	for i, t := range m.themes {
		if t.Name == m.theme {
			m.pickIdx = i
		}
	}
}

func (m *Model) updateThemePicker(msg tea.KeyMsg) {
	switch msg.String() {
	case "j", "down", "tab", "ctrl+n":
		m.pickIdx = (m.pickIdx + 1) % len(m.themes)
	case "k", "up", "shift+tab", "ctrl+p":
		m.pickIdx = (m.pickIdx - 1 + len(m.themes)) % len(m.themes)
	case "enter":
		m.mode = modeNormal
		m.saveTheme(m.themes[m.pickIdx].Name)
		return
	case "esc", "q":
		m.mode = modeNormal
		m.useTheme(m.pickPrev)
		return
	default:
		return
	}
	m.useTheme(m.themes[m.pickIdx].Name)
}

// saveTheme switches to the theme and records it in config.toml.
func (m *Model) saveTheme(name string) {
	m.useTheme(name)
	if err := config.Save(m.opts.ConfigPath, "", "theme", config.String(name)); err != nil {
		m.setError("theme %s applied, but not saved: %v", name, err)
		return
	}
	m.cfg.Theme = name
	m.setStatus("theme %s saved to %s", name, m.opts.ConfigPath)
}

func (m *Model) cycleTheme() {
	i := 0
	for j, t := range m.themes {
		if t.Name == m.theme {
			i = j
		}
	}
	m.saveTheme(m.themes[(i+1)%len(m.themes)].Name)
}

func (m Model) viewThemePicker() string {
	lines := []string{m.st.header.Render("THEME"), ""}
	for i, t := range m.themes {
		marker, name := "  ", m.st.text.Render(t.Name)
		if i == m.pickIdx {
			marker, name = m.st.prompt.Render("▌ "), m.st.prompt.Render(t.Name)
		}
		src := ""
		if t.File != "" {
			src = m.st.faint.Render("  " + filepath.Join("themes", filepath.Base(t.File)))
		}
		lines = append(lines, marker+name+src)
	}
	lines = append(lines, "",
		m.st.faint.Render("j/k preview · enter save · esc cancel"),
		m.st.faint.Render("katTasks -maketheme NAME copies the current theme to edit"))
	return strings.Join(lines, "\n")
}
