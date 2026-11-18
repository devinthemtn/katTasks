package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
)

func scriptModel(t *testing.T, scripts map[string]string) Model {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	for name, src := range scripts {
		os.WriteFile(filepath.Join(dir, "scripts", name+".lua"), []byte(src), 0o644)
	}
	todoPath := filepath.Join(dir, "todo.txt")
	os.WriteFile(todoPath, []byte("Old task\n"), 0o644)
	cfg := config.Defaults()
	cfg.Path = filepath.Join(dir, "config.toml")
	cfg.Files.Todo = todoPath
	cfg.Tasks.AddCreationDate = false
	m, err := New(cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return mm.(Model)
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}
	return m
}

func press(m Model, k tea.KeyType) Model {
	mm, _ := m.Update(tea.KeyMsg{Type: k})
	return mm.(Model)
}

func TestInlineScript(t *testing.T) {
	m := scriptModel(t, map[string]string{
		"week":  "-- due in a week\nreturn 'due:2026-12-01'",
		"weird": "return 'x'",
		"empty": "return nil",
	})
	m.startAdd()
	m = typeText(m, "Pay rent /we")
	if len(m.sugg) != 2 || m.sugg[0] != "/week" {
		t.Fatalf("suggestions %v", m.sugg)
	}
	m = press(m, tea.KeyTab)
	if v := m.input.Value(); v != "Pay rent due:2026-12-01 " {
		t.Fatalf("input after tab %q", v)
	}

	// /name words left in the text run on save; ones that aren't scripts stay.
	m = typeText(m, "/empty and/or /nope")
	m = press(m, tea.KeyEnter)
	if got := m.tasks[len(m.tasks)-1].Text; got != "Pay rent due:2026-12-01 and/or /nope" {
		t.Fatalf("saved %q", got)
	}
}

func TestPaletteScript(t *testing.T) {
	m := scriptModel(t, map[string]string{
		"star": "-- star it\nkat.update('(A) ' .. kat.task.text)\nkat.add('Follow up')",
	})
	m.openPalette()
	m = typeText(m, "star")
	if it := m.palItems[m.palShown[0]]; it.name != "script:star" || it.desc != "star it" {
		t.Fatalf("top palette item %+v", it)
	}
	m = press(m, tea.KeyEnter)
	if len(m.tasks) != 2 || m.tasks[0].String() != "(A) Old task" || m.tasks[1].Text != "Follow up" {
		t.Fatalf("tasks %v %v (%s)", m.tasks[0], m.tasks[len(m.tasks)-1], m.status)
	}
	m.undoLast()
	if len(m.tasks) != 1 || m.tasks[0].String() != "Old task" {
		t.Fatalf("undo should revert the whole script: %v", m.tasks)
	}
}
