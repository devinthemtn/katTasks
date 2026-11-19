package alert

import (
	"os"
	"path/filepath"
)

// Autostart is how katTasks remind starts at login on this OS.
//
// Install sets it up to run exe with args (normally "remind" plus any
// -config or -file flags) and returns a description of what it did;
// Uninstall undoes it.

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
