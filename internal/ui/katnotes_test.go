package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/listview"
)

// fakeKatnotes writes a shell script standing in for katnotes: it logs its
// arguments, lists two notes, and on "tasks import" tags the line the way
// katnotes does.
func fakeKatnotes(t *testing.T) (prog, log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	dir := t.TempDir()
	log = filepath.Join(dir, "args.log")
	prog = filepath.Join(dir, "katnotes")
	script := `#!/bin/sh
echo "$*" >> "` + log + `"
case "$1 $2" in
"--json ls") echo '[{"kind":"page","id":"house","title":"House"},{"kind":"page","id":"garden","title":"Garden"}]' ;;
"tasks import")
	file=$4; to=$6; raw=$7
	tmp="$file.tmp"
	while IFS= read -r line; do
		if [ "$line" = "$raw" ]; then echo "$line kn:abc123 note:$to"; else echo "$line"; fi
	done < "$file" > "$tmp" && mv "$tmp" "$file"
	echo "$to:3  - [ ] $raw {kt:abc123}" ;;
"--open "*) ;;
*) echo "Error: unknown command" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(prog, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return prog, log
}

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}

func TestAddToNote(t *testing.T) {
	prog, log := fakeKatnotes(t)
	m := scriptModel(t, nil)
	m.cfg.Katnotes.Command = prog

	cmd := m.addToNote()
	if cmd == nil {
		t.Fatalf("no command: %q", m.status)
	}
	m, _ = update(m, cmd())
	if m.mode != modePalette || m.palettePrompt() != "add to note › " || len(m.palShown) != 3 {
		t.Fatalf("picker: mode %v prompt %q shown %d", m.mode, m.palettePrompt(), len(m.palShown))
	}
	if s := m.Snapshot(); s.PalettePrompt != "add to note › " || len(s.Palette) != 3 || s.Palette[0].Name != "Today's journal" {
		t.Fatalf("snapshot palette: %q %+v", s.PalettePrompt, s.Palette)
	}
	m = typeText(m, "gard")
	if it := m.palItems[m.palShown[0]]; it.name != "Garden" || it.desc != "page/garden" {
		t.Fatalf("top match %+v", it)
	}
	m, cmd = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("enter gave no command: %q", m.status)
	}
	m, _ = update(m, cmd())
	if !strings.HasPrefix(m.status, "linked to page/garden") {
		t.Fatalf("status %q", m.status)
	}
	got, _ := os.ReadFile(log)
	if !strings.Contains(string(got), "tasks import --file "+m.cfg.Files.Todo+" --to page/garden Old task") {
		t.Errorf("katnotes args:\n%s", got)
	}

	task := m.selected()
	if note, ok := listview.LinkedNote(task); !ok || note != "page/garden" {
		t.Fatalf("selected %q: %q %v", task, note, ok)
	}
	s := m.Snapshot()
	var row *TaskRow
	for i := range s.Rows {
		if s.Rows[i].Cursor {
			row = &s.Rows[i]
		}
	}
	if row == nil || !row.IsLinked || row.Linked != "page/garden" {
		t.Fatalf("row %+v", row)
	}
	styles := map[string]string{}
	for _, w := range row.Words {
		styles[w.Text] = w.Style
	}
	if styles["kn:abc123"] != "link" || styles["note:page/garden"] != "link" {
		t.Errorf("word styles %v", styles)
	}
	if v := m.View(); !strings.Contains(v, "⇄") || !strings.Contains(v, "Linked to notes") {
		t.Errorf("TUI view lacks the badge or sidebar entry:\n%s", v)
	}

	if m.addToNote() != nil || !strings.Contains(m.status, "already linked to page/garden") {
		t.Errorf("relink: %q", m.status)
	}
}

func TestAddToNoteErrors(t *testing.T) {
	m := scriptModel(t, nil)
	m.cfg.Katnotes.Command = filepath.Join(t.TempDir(), "no-such-katnotes")
	m, _ = update(m, m.addToNote()())
	if !m.statusErr || !strings.Contains(m.status, "set [katnotes] command") {
		t.Errorf("missing program: %q", m.status)
	}
}

func TestOpenNote(t *testing.T) {
	prog, log := fakeKatnotes(t)
	m := scriptModel(t, nil)
	if m.openNote() != nil || !strings.Contains(m.status, "isn't linked") {
		t.Fatalf("unlinked: %q", m.status)
	}
	os.WriteFile(m.cfg.Files.Todo, []byte("Old task kn:abc123 note:page/house\n"), 0o644)
	if err := m.load(); err != nil {
		t.Fatal(err)
	}
	m.cfg.Katnotes.Command = prog
	if cmd := m.openNote(); cmd == nil { // the terminal app takes over the terminal
		t.Fatalf("TUI: no command: %q", m.status)
	}
	m.noteOpened(noteOpenedMsg{key: "page/house"})
	if m.status != "back from page/house" {
		t.Errorf("after: %q", m.status)
	}

	m.windowed, m.cfg.Katnotes.GUICommand = true, prog
	if m.openNote() != nil || !strings.Contains(m.status, "opened page/house") {
		t.Fatalf("GUI: %q", m.status)
	}
	for i := 0; i < 50; i++ {
		if b, _ := os.ReadFile(log); strings.Contains(string(b), "--open page/house") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	t.Errorf("katnotes-gui not run with --open:\n%s", b)
}
