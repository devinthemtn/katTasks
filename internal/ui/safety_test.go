package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The README's Safety section, from the app's side; internal/todo tests
// the file operations themselves.

// unwritable makes path's folder read-only until the returned func is
// called (or the test ends).
func unwritable(t *testing.T, path string) (restore func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores read-only folders")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only folder")
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	restore = func() { os.Chmod(dir, 0o755) }
	t.Cleanup(restore)
	return restore
}

// moveDone puts done.txt in a folder of its own, so todo.txt's folder can
// be made read-only while done.txt stays writable.
func moveDone(t *testing.T, m Model) Model {
	m.cfg.Files.Done = filepath.Join(t.TempDir(), "done.txt")
	return m
}

// If todo.txt can't be saved, archiving changes neither file nor the list,
// and once it can be saved, archiving again copies each task once.
func TestArchiveFailureChangesNothing(t *testing.T) {
	m := moveDone(t, fileModel(t, fileOpts{showDone: true}, "open", "x 2026-01-02 done"))
	restore := unwritable(t, m.cfg.Files.Todo)

	m = send(t, m, "A", "y")
	if !m.statusErr || !strings.Contains(m.status, "archive failed") {
		t.Errorf("status %q (err %v)", m.status, m.statusErr)
	}
	wantLines(t, m.cfg.Files.Todo, "open", "x 2026-01-02 done")
	wantLines(t, m.cfg.Files.Done)
	if len(m.tasks) != 2 || len(m.visible) != 2 {
		t.Errorf("list has %d tasks (%d shown), want both still there", len(m.tasks), len(m.visible))
	}

	restore()
	m = send(t, m, "A", "y")
	wantLines(t, m.cfg.Files.Todo, "open")
	wantLines(t, m.cfg.Files.Done, "x 2026-01-02 done")
}

// With auto_archive, a failed archive must not leave the task shown as
// done when that was never saved.
func TestAutoArchiveFailureKeepsTaskOpen(t *testing.T) {
	m := moveDone(t, fileModel(t, fileOpts{autoArchive: true}, "one", "two"))
	unwritable(t, m.cfg.Files.Todo)
	m = send(t, onTask(t, m, "one"), "x")
	if !m.statusErr {
		t.Errorf("status %q, want an error", m.status)
	}
	wantLines(t, m.cfg.Files.Todo, "one", "two")
	wantLines(t, m.cfg.Files.Done)
	for _, task := range m.tasks {
		if task.Done {
			t.Errorf("%q shows as done, but that wasn't saved", task.Text)
		}
	}
	if len(m.tasks) != 2 {
		t.Errorf("%d tasks in the list, want 2", len(m.tasks))
	}
}

// Adding and editing are refused too when another program changed
// todo.txt (bulk_test covers the other edits).
func TestAddAndEditRefusedWhenFileChangedOnDisk(t *testing.T) {
	const external = "edited elsewhere\n"
	for name, keys := range map[string][]string{
		"add":  {"a", "n", "e", "w", "enter"},
		"edit": {"e", "!", "enter"},
	} {
		t.Run(name, func(t *testing.T) {
			m := onTask(t, fileModel(t, fileOpts{}, "one"), "one")
			editExternally(t, m, external)
			m = send(t, m, keys...)
			if !m.statusErr || !strings.Contains(m.status, "changed on disk") {
				t.Errorf("status %q (err %v), want the changed-on-disk error", m.status, m.statusErr)
			}
			if b, _ := os.ReadFile(m.cfg.Files.Todo); string(b) != external {
				t.Errorf("todo.txt overwritten: %q", b)
			}
		})
	}
}
