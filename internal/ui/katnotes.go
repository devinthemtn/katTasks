// Linking tasks to katnotes notes (github.com/devinthemtn/katnotes).
//
// katnotes does the linking: "katnotes tasks import" tags the todo.txt line
// with kn:<id> note:<kind/id> and adds the task to the note. katTasks only
// picks the note, runs it, and reloads todo.txt; open_note runs katnotes
// (or katnotes-gui) with --open <kind/id>.

package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// katnotesNote is one entry of "katnotes --json ls".
type katnotesNote struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// notesListedMsg is the note list for the add_to_note picker.
type notesListedMsg struct {
	raw   string // the task's todo.txt line
	notes []katnotesNote
	err   error
}

// noteLinkedMsg is the result of "katnotes tasks import".
type noteLinkedMsg struct {
	key, raw, out string
	err           error
}

// noteOpenedMsg is sent when the katnotes TUI that open_note ran exits.
type noteOpenedMsg struct {
	key string
	err error
}

// katnotesErr explains a failure to run prog: a missing program names the
// setting to change; otherwise the program's own message.
func katnotesErr(prog, setting string, out []byte, err error) error {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s not found: set [katnotes] %s in the settings", prog, setting)
	}
	if msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "Error:")); msg != "" {
		return fmt.Errorf("%s: %s", prog, lastLine(msg))
	}
	return fmt.Errorf("%s: %v", prog, err)
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// addToNote starts add_to_note for the task under the cursor: it lists the
// notes in the background, then opens the picker (notesListed).
func (m *Model) addToNote() tea.Cmd {
	t := m.selected()
	if t == nil {
		return nil
	}
	if note, ok := listview.LinkedNote(t); ok {
		m.setStatus("already linked to %s (open_note opens it)", orUnknown(note))
		return nil
	}
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return nil
	}
	prog, raw := m.cfg.Katnotes.Command, t.String()
	m.setStatus("listing katnotes notes…")
	return func() tea.Msg {
		out, err := exec.Command(prog, "--json", "ls").Output()
		if err != nil {
			var stderr []byte
			if ee, ok := err.(*exec.ExitError); ok {
				stderr = ee.Stderr
			}
			return notesListedMsg{raw: raw, err: katnotesErr(prog, "command", stderr, err)}
		}
		var ns []katnotesNote
		if err := json.Unmarshal(out, &ns); err != nil {
			return notesListedMsg{raw: raw, err: fmt.Errorf("%s --json ls: %v", prog, err)}
		}
		return notesListedMsg{raw: raw, notes: ns}
	}
}

// notesListed opens the command palette as a note picker: typing filters
// the notes, enter links the task to the chosen one.
func (m *Model) notesListed(msg notesListedMsg) {
	if msg.err != nil {
		m.setError("%v", msg.err)
		return
	}
	pick := func(key string) func(m *Model) tea.Cmd {
		return func(m *Model) tea.Cmd { return m.linkToNote(msg.raw, key) }
	}
	items := []paletteItem{{name: "Today's journal", desc: "journal/today", run: pick("today")}}
	for _, n := range msg.notes {
		key := n.Kind + "/" + n.ID
		title := n.Title
		if title == "" {
			title = n.ID
		}
		items = append(items, paletteItem{name: title, desc: key, run: pick(key)})
	}
	m.status = ""
	m.mode = modePalette
	m.palPrompt, m.palEmpty = "add to note › ", "no matching note"
	m.palItems, m.palIdx = items, 0
	m.input.SetValue("")
	m.input.Focus()
	m.filterPalette()
}

// linkToNote runs "katnotes tasks import" for the task line raw.
func (m *Model) linkToNote(raw, key string) tea.Cmd {
	if m.stale() {
		m.setError("todo.txt changed on disk — press r to reload first")
		return nil
	}
	prog, file := m.cfg.Katnotes.Command, m.cfg.Files.Todo
	m.setStatus("linking to %s…", key)
	return func() tea.Msg {
		out, err := exec.Command(prog, "tasks", "import", "--file", file, "--to", key, raw).CombinedOutput()
		if err != nil {
			return noteLinkedMsg{key: key, raw: raw, err: katnotesErr(prog, "command", out, err)}
		}
		return noteLinkedMsg{key: key, raw: raw, out: string(out)}
	}
}

// noteLinked reloads todo.txt after katnotes tagged the line, keeping the
// cursor on the task.
func (m *Model) noteLinked(msg noteLinkedMsg) {
	if msg.err != nil {
		m.setError("%v", msg.err)
		return
	}
	if err := m.load(); err != nil {
		m.setError("linked, but reloading todo.txt failed: %v", err)
		return
	}
	m.undo = nil // the file changed outside the undo history
	var linked *todo.Task
	for _, t := range m.tasks {
		if strings.HasPrefix(t.String(), msg.raw) {
			if _, ok := listview.LinkedNote(t); ok {
				linked = t
				break
			}
		}
	}
	m.refresh(linked)
	where := msg.key
	if linked != nil {
		if note, _ := listview.LinkedNote(linked); note != "" {
			where = note
		}
	}
	if strings.Contains(msg.out, "warning") {
		m.setStatus("linked to %s — %s", where, lastLine(strings.TrimSpace(msg.out)))
		return
	}
	m.setStatus("linked to %s", where)
}

// openNote opens the note the task under the cursor is linked to: the
// katnotes TUI in place of katTasks in a terminal, katnotes-gui from
// katTasks-gui.
func (m *Model) openNote() tea.Cmd {
	t := m.selected()
	if t == nil {
		return nil
	}
	key, ok := listview.LinkedNote(t)
	if !ok || key == "" {
		m.setError("this task isn't linked to a note (add_to_note links it)")
		return nil
	}
	if m.windowed {
		prog := m.cfg.Katnotes.GUICommand
		c := exec.Command(prog, "--open", key)
		if err := c.Start(); err != nil {
			m.setError("%v", katnotesErr(prog, "gui_command", nil, err))
			return nil
		}
		go c.Wait() //nolint:errcheck // it runs on its own; a second launch hands over and exits
		m.setStatus("opened %s in %s", key, prog)
		return nil
	}
	prog := m.cfg.Katnotes.Command
	if _, err := exec.LookPath(prog); err != nil {
		m.setError("%v", katnotesErr(prog, "command", nil, err))
		return nil
	}
	return tea.ExecProcess(exec.Command(prog, "--open", key), func(err error) tea.Msg { return noteOpenedMsg{key, err} })
}

// noteOpened picks up what katnotes changed in todo.txt (say, a task
// checked in the note) once its TUI exits.
func (m *Model) noteOpened(msg noteOpenedMsg) {
	if msg.err != nil {
		m.setError("%s: %v", m.cfg.Katnotes.Command, msg.err)
		return
	}
	if m.stale() {
		keep := m.selected()
		id := ""
		if keep != nil {
			id, _ = keep.Tag("kn")
		}
		if err := m.load(); err != nil {
			m.setError("reloading todo.txt failed: %v", err)
			return
		}
		m.undo = nil
		var again *todo.Task
		for _, t := range m.tasks {
			if v, ok := t.Tag("kn"); ok && id != "" && v == id {
				again = t
			}
		}
		m.refresh(again)
		m.setStatus("back from %s; todo.txt reloaded", msg.key)
		return
	}
	m.setStatus("back from %s", msg.key)
}

func orUnknown(note string) string {
	if note == "" {
		return "a note"
	}
	return note
}
