//go:build !windows

package ui

import "os/exec"

// editorCommand runs the editor command line ed (which may carry flags,
// like "emacsclient -t") on path, through the shell as git does.
func editorCommand(ed, path string) *exec.Cmd {
	return exec.Command("sh", "-c", ed+` "$1"`, "sh", path)
}
