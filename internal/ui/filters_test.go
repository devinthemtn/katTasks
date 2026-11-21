package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/devinthemtn/katTasks/internal/config"
)

// typeKeys types s, a key per character.
func typeKeys(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = send(t, m, string(r))
	}
	return m
}

func shownTexts(m Model) []string {
	var out []string
	for _, t := range m.visible {
		out = append(out, t.Text)
	}
	return out
}

// fs saves the current search to [filters] in the settings file, and the
// sidebar then shows it.
func TestSaveSearch(t *testing.T) {
	m := fileModel(t, fileOpts{}, "report +Work", "groceries +Home", "plan +Work @desk")
	m = typeKeys(t, send(t, m, "/"), "+Work")
	m = send(t, m, "enter")
	if got := shownTexts(m); len(got) != 2 {
		t.Fatalf("search +Work shows %q", got)
	}

	m = send(t, m, "f", "s")
	if m.mode != modeSaveName {
		t.Fatalf("fs: mode %v", m.mode)
	}
	m = send(t, typeKeys(t, m, `work "week"`), "enter")
	if m.statusErr {
		t.Fatalf("status %q", m.status)
	}

	c, _, errs := config.Load(m.cfg.Path)
	if len(errs) != 0 || c.Filters[`work "week"`] != "+Work" {
		t.Fatalf("settings file filters %v (errs %v)", c.Filters, errs)
	}
	if e := m.currentEntry(); e.Kind != entrySaved || e.Value != `work "week"` {
		t.Errorf("sidebar on %+v, want the saved search", e)
	}
	if m.filter != "" || len(m.visible) != 2 {
		t.Errorf("filter %q, %d shown; want the saved search alone to show the two", m.filter, len(m.visible))
	}

	// It survives reloading the settings (ctrl+r).
	m = send(t, m, "ctrl+r")
	if m.cfg.Filters[`work "week"`] != "+Work" {
		t.Errorf("after reload: %v", m.cfg.Filters)
	}
}

// A project picked in the sidebar is saved when there is no search.
func TestSaveSearchFromSidebar(t *testing.T) {
	m := fileModel(t, fileOpts{}, "report +Work", "groceries +Home")
	for i, e := range m.side {
		if e.Kind == entryProject && e.Value == "+Home" {
			m.sideCursor = i
			m.refresh(nil)
		}
	}
	if got := m.currentQuery(); got != "+Home" {
		t.Fatalf("current query %q", got)
	}
	m = send(t, typeKeys(t, send(t, m, "f", "s"), "home"), "enter")
	if c, _, _ := config.Load(m.cfg.Path); c.Filters["home"] != "+Home" {
		t.Errorf("filters %v", c.Filters)
	}
}

func TestSaveSearchNothingOrCancelled(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one")
	m = send(t, m, "f", "s")
	if m.mode == modeSaveName || !strings.HasPrefix(m.status, "nothing to save") {
		t.Errorf("mode %v status %q", m.mode, m.status)
	}

	m = send(t, typeKeys(t, send(t, m, "/"), "one"), "enter")
	m = send(t, typeKeys(t, send(t, m, "f", "s"), "x"), "esc")
	if _, err := os.Stat(m.cfg.Path); err == nil {
		if c, _, _ := config.Load(m.cfg.Path); len(c.Filters) != 0 {
			t.Errorf("esc saved %v", c.Filters)
		}
	}

	// An empty name doesn't save either.
	m = send(t, send(t, m, "f", "s"), "enter")
	if m.status != "not saved" {
		t.Errorf("empty name: status %q", m.status)
	}
}

func TestJumpToSaved(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one +A", "two +B")
	m = send(t, m, "f", "f")
	if !strings.HasPrefix(m.status, "no saved searches") {
		t.Errorf("no searches: status %q", m.status)
	}
	m.cfg.Filters = map[string]string{"bees": "+B"}
	m.refresh(nil)
	m = send(t, m, "f", "f")
	if e := m.currentEntry(); e.Kind != entrySaved || m.focus != focusSidebar {
		t.Errorf("ff: entry %+v focus %v", e, m.focus)
	}
	if got := shownTexts(m); len(got) != 1 || got[0] != "two +B" {
		t.Errorf("shows %q", got)
	}
}
