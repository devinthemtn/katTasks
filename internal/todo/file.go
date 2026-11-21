package todo

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Load reads every non-blank line of path. A missing file is an empty list.
// It also returns the file's modification time so callers can detect
// changes made by other programs.
func Load(path string) ([]*Task, time.Time, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()

	var tasks []*Task
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		tasks = append(tasks, Parse(sc.Text()))
	}
	if err := sc.Err(); err != nil {
		return nil, time.Time{}, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	return tasks, st.ModTime(), nil
}

// ModTime returns the file's modification time, or zero if it is missing.
func ModTime(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

// Save writes tasks to path atomically (temp file + rename) and returns the
// new modification time. The temp file is synced before the rename, so a
// crash or power cut leaves either the old file or the new one. If path is
// a symlink, the file it points to is replaced and the link is kept.
func Save(path string, tasks []*Task) (time.Time, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return time.Time{}, err
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".todo-*.tmp")
	if err != nil {
		return time.Time{}, err
	}
	defer os.Remove(tmp.Name())

	w := bufio.NewWriter(tmp)
	eol, withBOM := lineEnding(path)
	if withBOM {
		w.WriteString(bom)
	}
	for _, t := range tasks {
		w.WriteString(t.line())
		w.WriteString(eol)
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return time.Time{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return time.Time{}, err
	}
	if err := tmp.Close(); err != nil {
		return time.Time{}, err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return time.Time{}, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return time.Time{}, err
	}
	syncDir(filepath.Dir(path))
	return ModTime(path), nil
}

// syncDir makes a rename in dir durable. It's best effort: Windows can't
// sync a folder, and the rename is done either way.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync() //nolint:errcheck
		d.Close()
	}
}

// Archive moves done to donePath and saves keep, the tasks left, to
// todoPath. done.txt is written first, so a failure can't lose tasks. If
// writing either file fails, done.txt is put back as it was, so trying
// again doesn't add the tasks twice; todo.txt is then unchanged too, as
// Save replaces it whole or not at all.
func Archive(todoPath string, keep []*Task, donePath string, done []*Task) (time.Time, error) {
	size := int64(-1) // done.txt doesn't exist
	if st, err := os.Stat(donePath); err == nil {
		size = st.Size()
	}
	restore := func() {
		if size < 0 {
			os.Remove(donePath)
		} else {
			os.Truncate(donePath, size)
		}
	}
	if err := Append(donePath, done); err != nil {
		restore()
		return time.Time{}, err
	}
	mt, err := Save(todoPath, keep)
	if err != nil {
		restore()
		return time.Time{}, err
	}
	return mt, nil
}

// lineEnding is how the file at path is written, so saving it doesn't
// change that for other programs: with \r\n line endings if it has them
// now (as Windows editors write it), and with the byte order mark if it
// starts with one. A new file gets \n and no mark.
func lineEnding(path string) (eol string, withBOM bool) {
	eol = "\n"
	f, err := os.Open(path)
	if err != nil {
		return eol, false
	}
	defer f.Close()
	head := make([]byte, 64*1024)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if i := bytes.IndexByte(head, '\n'); i > 0 && head[i-1] == '\r' {
		eol = "\r\n"
	}
	return eol, bytes.HasPrefix(head, []byte(bom))
}

// Append adds tasks to the end of path (used for done.txt).
func Append(path string, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Make sure we start on a fresh line if the file lacks a trailing newline.
	prefix := ""
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 && b[len(b)-1] != '\n' {
		prefix, _ = lineEnding(path)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(prefix)
	eol, _ := lineEnding(path)
	for _, t := range tasks {
		b.WriteString(t.line())
		b.WriteString(eol)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
