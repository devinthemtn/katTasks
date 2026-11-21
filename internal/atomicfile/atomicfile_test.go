package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteNewAndReplace(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "f.txt")
	if err := Write(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(p, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "two" {
		t.Errorf("content %q", b)
	}
	if runtime.GOOS != "windows" {
		// The existing file's mode wins over perm.
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Errorf("mode %v, want 0600", st.Mode().Perm())
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "sub", ".*.tmp")); len(left) > 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestWriteFailureLeavesFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user can't write")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	os.WriteFile(p, []byte("old"), 0o644)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if err := Write(p, []byte("new"), 0o644); err == nil {
		t.Fatal("write into a read-only folder succeeded")
	}
	if b, _ := os.ReadFile(p); string(b) != "old" {
		t.Errorf("content %q", b)
	}
}

func TestWriteKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "f.txt")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte("old"), 0o644)
	link := filepath.Join(dir, "f.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("can't make symlinks here:", err)
	}
	if err := Write(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
	if b, _ := os.ReadFile(real); string(b) != "new" {
		t.Errorf("linked file %q", b)
	}
}
