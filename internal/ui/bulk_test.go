package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/config"
)

// These tests cover the actions that change todo.txt and done.txt: delete,
// priority, reordering, visual-mode bulk edits and archiving. They press
// keys as a user would and check the files on disk.

type fileOpts struct {
	sort        string // default "file", so cursor positions follow the file
	autoArchive bool
	showDone    bool // done tasks are hidden by default
}

// fileModel writes lines to a fresh todo.txt and opens it.
func fileModel(t *testing.T, o fileOpts, lines ...string) Model {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Path = filepath.Join(dir, "config.toml")
	cfg.Files.Todo = filepath.Join(dir, "todo.txt")
	cfg.Files.Done = filepath.Join(dir, "done.txt")
	cfg.Tasks.AddCreationDate = false
	cfg.Tasks.AutoArchive = o.autoArchive
	cfg.View.HideCompleted = !o.showDone
	cfg.View.Sort = o.sort
	if cfg.View.Sort == "" {
		cfg.View.Sort = "file"
	}
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(cfg.Files.Todo, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(cfg, Options{ConfigPath: cfg.Path})
	if err != nil {
		t.Fatal(err)
	}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return mm.(Model)
}

// send presses keys, named as ParseKeyMsg names them. Background commands
// (chord timers) are dropped.
func send(t *testing.T, m Model, ks ...string) Model {
	t.Helper()
	for _, k := range ks {
		msg, ok := ParseKeyMsg(k)
		if !ok {
			t.Fatalf("bad key %q", k)
		}
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

// onTask puts the cursor on the visible task whose text is text.
func onTask(t *testing.T, m Model, text string) Model {
	t.Helper()
	for i, x := range m.visible {
		if x.Text == text {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no visible task %q", text)
	return m
}

// lines returns the file's lines, or nil if it doesn't exist.
func lines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	s := strings.TrimRight(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func wantLines(t *testing.T, path string, want ...string) {
	t.Helper()
	if got := lines(t, path); !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", filepath.Base(path), got, want)
	}
}

// editExternally changes todo.txt as another program would, with a
// modification time that can't match the one katTasks saw.
func editExternally(t *testing.T, m Model, content string) {
	t.Helper()
	if err := os.WriteFile(m.cfg.Files.Todo, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(m.cfg.Files.Todo, future, future); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteAsksAndUndoes(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two", "three")
	m = send(t, onTask(t, m, "two"), "d")
	if m.mode != modeConfirm || !strings.Contains(m.confirm, `"two"`) {
		t.Fatalf("d: mode %v, prompt %q", m.mode, m.confirm)
	}
	m = send(t, m, "n")
	wantLines(t, m.cfg.Files.Todo, "one", "two", "three")

	m = send(t, m, "d", "y")
	wantLines(t, m.cfg.Files.Todo, "one", "three")
	if !strings.HasPrefix(m.status, "deleted 1 task") {
		t.Errorf("status %q", m.status)
	}

	m = send(t, m, "u")
	wantLines(t, m.cfg.Files.Todo, "one", "two", "three")
}

func TestVisualDeleteMarkedTasks(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two", "three", "four")
	// v, then space marks a row and moves down: mark one and two, skip
	// three, mark four.
	m = send(t, onTask(t, m, "one"), "v", " ", " ", "j", " ")
	if !m.visual || len(m.sel) != 3 {
		t.Fatalf("visual %v, %d marked", m.visual, len(m.sel))
	}
	m = send(t, m, "d")
	if !strings.Contains(m.confirm, "3 tasks") {
		t.Fatalf("prompt %q", m.confirm)
	}
	m = send(t, m, "y")
	wantLines(t, m.cfg.Files.Todo, "three")
	if m.visual || m.sel != nil {
		t.Error("visual mode still on after the delete")
	}

	// One undo step brings all three back, in their places.
	m = send(t, m, "u")
	wantLines(t, m.cfg.Files.Todo, "one", "two", "three", "four")
}

func TestSelectAllDelete(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two")
	m = send(t, m, "V", "d", "y")
	wantLines(t, m.cfg.Files.Todo)
}

// With v on but nothing marked, actions apply to the row under the cursor.
func TestVisualWithoutMarksUsesCursor(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two")
	m = send(t, onTask(t, m, "two"), "v", "d", "y")
	wantLines(t, m.cfg.Files.Todo, "one")
}

func TestSetPriority(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "(C) two")
	m = send(t, onTask(t, m, "one"), "p", "b")
	wantLines(t, m.cfg.Files.Todo, "(B) one", "(C) two")

	m = send(t, onTask(t, m, "two"), "p", " ")
	wantLines(t, m.cfg.Files.Todo, "(B) one", "two")

	// Any other key cancels without saving.
	m = send(t, onTask(t, m, "one"), "p", "!")
	if m.status != "cancelled" {
		t.Errorf("status %q", m.status)
	}
	wantLines(t, m.cfg.Files.Todo, "(B) one", "two")
}

func TestSetPriorityOnDoneTask(t *testing.T) {
	m := fileModel(t, fileOpts{showDone: true}, "x 2026-01-02 done one")
	m = send(t, onTask(t, m, "done one"), "p", "a")
	if !m.statusErr {
		t.Errorf("no error setting a priority on a done task (status %q)", m.status)
	}
	wantLines(t, m.cfg.Files.Todo, "x 2026-01-02 done one")
}

// A visual selection of open and done tasks: a letter goes on the open ones
// only.
func TestSetPriorityMixedSelection(t *testing.T) {
	m := fileModel(t, fileOpts{showDone: true}, "open one", "x 2026-01-02 done one", "open two")
	m = send(t, m, "V")
	if len(m.sel) != 3 {
		t.Fatalf("V selected %d tasks, want 3", len(m.sel))
	}
	m = send(t, m, "p", "a")
	wantLines(t, m.cfg.Files.Todo, "(A) open one", "x 2026-01-02 done one", "(A) open two")
}

func TestBumpPriority(t *testing.T) {
	m := fileModel(t, fileOpts{}, "none", "(A) top", "(B) bee", "(Z) last")
	m = send(t, onTask(t, m, "none"), "+")
	m = send(t, onTask(t, m, "top"), "+") // already A: unchanged
	m = send(t, onTask(t, m, "bee"), "-")
	m = send(t, onTask(t, m, "last"), "-") // below Z: no priority
	wantLines(t, m.cfg.Files.Todo, "(A) none", "(A) top", "(C) bee", "last")
	// Each change is one undo step; the no-op on (A) adds none.
	if len(m.undo) != 3 {
		t.Errorf("%d undo steps, want 3", len(m.undo))
	}
}

func TestMoveTaskInFileOrder(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two", "three")
	m = send(t, onTask(t, m, "one"), "J")
	wantLines(t, m.cfg.Files.Todo, "two", "one", "three")
	if m.selected().Text != "one" {
		t.Errorf("cursor on %q, want it to follow the moved task", m.selected().Text)
	}
	m = send(t, m, "J", "J") // second J is past the end: no change
	wantLines(t, m.cfg.Files.Todo, "two", "three", "one")
	m = send(t, m, "K")
	wantLines(t, m.cfg.Files.Todo, "two", "one", "three")
}

// Sorted by priority, a task only swaps with a neighbor of the same
// priority; anything else wouldn't move it in the list.
func TestMoveTaskSortedByPriority(t *testing.T) {
	m := fileModel(t, fileOpts{sort: "priority"}, "(B) bee", "(A) one", "(A) two")
	m = send(t, onTask(t, m, "two"), "J") // next row is (B)
	if !m.statusErr {
		t.Errorf("moved across priorities (status %q)", m.status)
	}
	wantLines(t, m.cfg.Files.Todo, "(B) bee", "(A) one", "(A) two")

	m = send(t, m, "K") // swap with (A) one
	wantLines(t, m.cfg.Files.Todo, "(B) bee", "(A) two", "(A) one")
}

func TestCompleteRecurringInsertsNextAfterIt(t *testing.T) {
	m := fileModel(t, fileOpts{}, "first", "water plants due:2026-03-01 rec:1w", "last")
	m = send(t, onTask(t, m, "water plants due:2026-03-01 rec:1w"), "x")
	got := lines(t, m.cfg.Files.Todo)
	if len(got) != 4 || got[0] != "first" || got[3] != "last" {
		t.Fatalf("todo.txt %q", got)
	}
	if !strings.HasPrefix(got[1], "x ") || !strings.Contains(got[1], "due:2026-03-01") {
		t.Errorf("completed line %q", got[1])
	}
	if strings.HasPrefix(got[2], "x ") || !strings.Contains(got[2], "rec:1w") || strings.Contains(got[2], "due:2026-03-01") {
		t.Errorf("next occurrence %q", got[2])
	}
	m = send(t, m, "u")
	wantLines(t, m.cfg.Files.Todo, "first", "water plants due:2026-03-01 rec:1w", "last")
}

func TestArchiveMovesDoneTasks(t *testing.T) {
	m := fileModel(t, fileOpts{}, "open", "x 2026-01-02 done one", "x 2026-01-03 done two")
	os.WriteFile(m.cfg.Files.Done, []byte("x 2025-12-31 archived before"), 0o644) // no trailing newline
	m = send(t, m, "A")
	if m.mode != modeConfirm || !strings.Contains(m.confirm, "2 completed") {
		t.Fatalf("A: mode %v prompt %q", m.mode, m.confirm)
	}
	m = send(t, m, "y")
	wantLines(t, m.cfg.Files.Todo, "open")
	wantLines(t, m.cfg.Files.Done, "x 2025-12-31 archived before", "x 2026-01-02 done one", "x 2026-01-03 done two")

	// Archiving touches two files, so it can't be undone.
	m = send(t, m, "u")
	if m.status != "nothing to undo" {
		t.Errorf("after archive, u: %q", m.status)
	}
	wantLines(t, m.cfg.Files.Todo, "open")
}

func TestArchiveDeclinedOrEmpty(t *testing.T) {
	m := fileModel(t, fileOpts{}, "open", "x 2026-01-02 done")
	m = send(t, m, "A", "n")
	wantLines(t, m.cfg.Files.Todo, "open", "x 2026-01-02 done")
	if lines(t, m.cfg.Files.Done) != nil {
		t.Error("done.txt written after answering no")
	}

	m = fileModel(t, fileOpts{}, "open")
	m = send(t, m, "A")
	if m.mode == modeConfirm || m.status != "no completed tasks to archive" {
		t.Errorf("mode %v status %q", m.mode, m.status)
	}
}

func TestAutoArchiveOnComplete(t *testing.T) {
	m := fileModel(t, fileOpts{autoArchive: true}, "one", "two")
	m = send(t, onTask(t, m, "one"), "x")
	wantLines(t, m.cfg.Files.Todo, "two")
	done := lines(t, m.cfg.Files.Done)
	if len(done) != 1 || !strings.HasPrefix(done[0], "x ") || !strings.HasSuffix(done[0], " one") {
		t.Errorf("done.txt %q", done)
	}
	if !strings.Contains(m.status, "archived") {
		t.Errorf("status %q", m.status)
	}
}

// If another program changed todo.txt, nothing that writes it may go
// ahead: the other program's edit must survive, and done.txt must not get
// tasks that are still in todo.txt.
func TestEditsRefusedWhenFileChangedOnDisk(t *testing.T) {
	const external = "edited elsewhere\nx 2026-01-02 done\n"
	for _, tc := range []struct {
		name string
		opts fileOpts
		keys []string
	}{
		{"delete", fileOpts{}, []string{"d", "y"}},
		{"visual delete", fileOpts{}, []string{"V", "d", "y"}},
		{"complete", fileOpts{}, []string{"x"}},
		{"set priority", fileOpts{}, []string{"p", "a"}},
		{"bump priority", fileOpts{}, []string{"+"}},
		{"move", fileOpts{}, []string{"J"}},
		{"start doing", fileOpts{}, []string{"i"}},
		{"progress", fileOpts{}, []string{">"}},
		{"archive", fileOpts{}, []string{"A", "y"}},
		{"auto-archive", fileOpts{autoArchive: true}, []string{"x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fileModel(t, tc.opts, "one", "two", "x 2026-01-01 old")
			m = onTask(t, m, "one")
			editExternally(t, m, external)
			m = send(t, m, tc.keys...)
			if !m.statusErr || !strings.Contains(m.status, "changed on disk") {
				t.Errorf("status %q (err %v), want the changed-on-disk error", m.status, m.statusErr)
			}
			if b, _ := os.ReadFile(m.cfg.Files.Todo); string(b) != external {
				t.Errorf("todo.txt overwritten:\n%s", b)
			}
			if lines(t, m.cfg.Files.Done) != nil {
				t.Errorf("done.txt written: %q", lines(t, m.cfg.Files.Done))
			}
		})
	}
}

// After r reloads the changed file, edits work again, on the new content.
func TestReloadAfterExternalChange(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one")
	editExternally(t, m, "one\nadded elsewhere\n")
	m = send(t, m, "r")
	m = send(t, onTask(t, m, "one"), "d", "y")
	wantLines(t, m.cfg.Files.Todo, "added elsewhere")
}
