// Package atomicfile replaces files whole or not at all.
package atomicfile

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Write replaces the file at path with data. It writes a temporary file
// beside it, syncs it to disk and renames it over path, so a crash or
// power cut leaves either the old file or the new one, never part of
// either. An existing file keeps its permissions; a new one gets perm. If
// path is a symlink, the file it points to is replaced and the link kept.
func Write(path string, data []byte, perm fs.FileMode) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

// syncDir makes a rename in dir durable. It's best effort: Windows can't
// sync a folder, and the rename is done either way.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync() //nolint:errcheck
		d.Close()
	}
}
