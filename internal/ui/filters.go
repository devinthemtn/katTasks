package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/listview"
)

// currentQuery is what "save search" saves: the / filter, or the sidebar's
// project or context if there is no filter.
func (m *Model) currentQuery() string {
	return listview.CurrentQuery(m.currentEntry(), m.filter, m.cfg.Filters)
}

// startSaveFilter asks for a name for the current search.
func (m *Model) startSaveFilter() {
	if m.currentQuery() == "" {
		m.setStatus("nothing to save — search with / or pick a project first")
		return
	}
	m.mode = modeSaveName
	m.startInput("", -1)
}

func (m *Model) saveFilter(name string) {
	q := m.currentQuery()
	name = strings.TrimSpace(name)
	if name == "" {
		m.setStatus("not saved")
		return
	}
	if err := config.Save(m.opts.ConfigPath, "filters", name, config.String(q)); err != nil {
		m.setError("couldn't save search: %v", err)
		return
	}
	if m.cfg.Filters == nil {
		m.cfg.Filters = map[string]string{}
	}
	m.cfg.Filters[name] = q
	m.filter = ""
	m.refresh(nil)
	m.selectSaved(name)
	m.setStatus("saved search %q = %q", name, q)
}

// selectSaved shows the named saved search, or the first one if name is "".
func (m *Model) selectSaved(name string) bool {
	for i, e := range m.side {
		if e.Kind == entrySaved && (name == "" || e.Value == name) {
			m.sideCursor = i
			m.cursor, m.offset = 0, 0
			m.refresh(nil)
			m.cursor = 0
			m.clampCursor()
			return true
		}
	}
	return false
}

func (m *Model) jumpToSaved() {
	if !m.selectSaved("") {
		m.setStatus("no saved searches yet — fs saves the current one")
		return
	}
	m.focus = focusSidebar
}

// savedFilterItems lists saved searches for the command palette.
func (m *Model) savedFilterItems() []paletteItem {
	var items []paletteItem
	for _, n := range m.cfg.FilterNames() {
		name := n
		items = append(items, paletteItem{
			name: "filter: " + name, desc: m.cfg.Filters[name],
			run: func(m *Model) tea.Cmd { m.selectSaved(name); return nil },
		})
	}
	return items
}
