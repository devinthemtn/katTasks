package todo

import (
	"bufio"
	"errors"
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
// new modification time.
func Save(path string, tasks []*Task) (time.Time, error) {
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
	for _, t := range tasks {
		w.WriteString(t.String())
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
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
	return ModTime(path), nil
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
		prefix = "\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(prefix)
	for _, t := range tasks {
		b.WriteString(t.String())
		b.WriteByte('\n')
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
