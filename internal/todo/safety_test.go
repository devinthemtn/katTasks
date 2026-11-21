package todo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests hold the promises in the README's Safety section: a save
// replaces todo.txt whole or not at all, and archiving can neither lose
// tasks nor, when it fails, leave copies of them in done.txt.

// readOnly makes dir unwritable for the rest of the test, so creating the
// temp file for a save in it fails.
func readOnly(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores read-only folders")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only folder")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// tempFiles lists save's leftover temp files in dir.
func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, ".todo-*.tmp"))
	return m
}

func TestSaveFailureLeavesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "todo.txt")
	os.WriteFile(p, []byte("one\ntwo\n"), 0o644)
	readOnly(t, dir)
	if _, err := Save(p, []*Task{Parse("changed")}); err == nil {
		t.Fatal("save into a read-only folder succeeded")
	}
	if got := read(t, p); got != "one\ntwo\n" {
		t.Errorf("todo.txt is now %q", got)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "todo.txt")
	for i := 0; i < 3; i++ {
		if _, err := Save(p, []*Task{Parse("one")}); err != nil {
			t.Fatal(err)
		}
	}
	if left := tempFiles(t, dir); len(left) > 0 {
		t.Errorf("temp files left: %v", left)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("folder holds %d files, want just todo.txt", len(entries))
	}
}

// A private todo.txt stays private: the temp file that replaces it would
// otherwise get the default 0644.
func TestSaveKeepsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix permissions")
	}
	p := filepath.Join(t.TempDir(), "todo.txt")
	os.WriteFile(p, []byte("secret\n"), 0o600)
	if _, err := Save(p, []*Task{Parse("still secret")}); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", st.Mode().Perm())
	}
}

// A todo.txt that links to a synced folder must stay a link, or later
// saves would stop reaching the real file.
func TestSaveKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "synced", "todo.txt")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte("one\n"), 0o644)
	link := filepath.Join(dir, "todo.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("can't make symlinks here:", err)
	}
	if _, err := Save(link, []*Task{Parse("two")}); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("todo.txt is no longer a symlink (%v)", err)
	}
	if got := read(t, real); got != "two\n" {
		t.Errorf("linked file is %q", got)
	}
	if left := tempFiles(t, dir); len(left) > 0 {
		t.Errorf("temp files left beside the link: %v", left)
	}
}

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	todoPath, donePath := filepath.Join(dir, "todo.txt"), filepath.Join(dir, "done.txt")
	os.WriteFile(todoPath, []byte("open\nx 2026-01-02 done\n"), 0o644)
	os.WriteFile(donePath, []byte("x 2025-12-31 before\n"), 0o644)
	if _, err := Archive(todoPath, []*Task{Parse("open")}, donePath, []*Task{Parse("x 2026-01-02 done")}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, todoPath); got != "open\n" {
		t.Errorf("todo.txt %q", got)
	}
	if got := read(t, donePath); got != "x 2025-12-31 before\nx 2026-01-02 done\n" {
		t.Errorf("done.txt %q", got)
	}
}

// If todo.txt can't be saved, done.txt goes back to what it was, so the
// tasks aren't in both files, and archiving again doesn't copy them twice.
func TestArchiveFailureRestoresDone(t *testing.T) {
	for name, before := range map[string]*string{"existing done.txt": ptr("x 2025-12-31 before"), "no done.txt": nil} {
		t.Run(name, func(t *testing.T) {
			todoDir, doneDir := t.TempDir(), t.TempDir()
			todoPath, donePath := filepath.Join(todoDir, "todo.txt"), filepath.Join(doneDir, "done.txt")
			const todo = "open\nx 2026-01-02 done\n"
			os.WriteFile(todoPath, []byte(todo), 0o644)
			if before != nil {
				os.WriteFile(donePath, []byte(*before), 0o644) // no final newline: Append adds one
			}
			readOnly(t, todoDir)

			for try := 0; try < 2; try++ {
				if _, err := Archive(todoPath, []*Task{Parse("open")}, donePath, []*Task{Parse("x 2026-01-02 done")}); err == nil {
					t.Fatal("archive succeeded with todo.txt unwritable")
				}
			}
			if got := read(t, todoPath); got != todo {
				t.Errorf("todo.txt %q", got)
			}
			b, err := os.ReadFile(donePath)
			switch {
			case before == nil && !os.IsNotExist(err):
				t.Errorf("done.txt created: %q (%v)", b, err)
			case before != nil && string(b) != *before:
				t.Errorf("done.txt %q, want it back to %q", b, *before)
			}
		})
	}
}

// When done.txt can't be written, nothing is removed from todo.txt.
func TestArchiveDoneUnwritable(t *testing.T) {
	todoDir, doneDir := t.TempDir(), t.TempDir()
	todoPath := filepath.Join(todoDir, "todo.txt")
	donePath := filepath.Join(doneDir, "sub", "done.txt")
	const todo = "open\nx 2026-01-02 done\n"
	os.WriteFile(todoPath, []byte(todo), 0o644)
	readOnly(t, doneDir)
	_, err := Archive(todoPath, []*Task{Parse("open")}, donePath, []*Task{Parse("x 2026-01-02 done")})
	if err == nil {
		t.Fatal("archive succeeded with done.txt unwritable")
	}
	if got := read(t, todoPath); got != todo || !strings.Contains(got, "x 2026-01-02 done") {
		t.Errorf("todo.txt %q", got)
	}
}

func ptr(s string) *string { return &s }
