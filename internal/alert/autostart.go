package alert

import (
	"os"
	"path/filepath"
)

// Each OS's autostart file has Install, which sets katTasks remind up to
// start at login with the given arguments (normally "remind" plus any
// -config or -file flags) and says what it did, and Uninstall, which undoes
// it.

// executable is the running program's path, symlinks resolved.
func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe, nil
}
