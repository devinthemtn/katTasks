package ui

import (
	"os/exec"
	"syscall"
)

// editorCommand runs the editor command line ed (which may carry flags) on
// path, through cmd.exe. The command line is passed as written: given as an
// argument, Go would escape the quotes around path as \", which cmd.exe
// doesn't understand. /s with the outer quotes makes cmd.exe run what's
// inside them as is.
func editorCommand(ed, path string) *exec.Cmd {
	c := exec.Command("cmd.exe")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /s /c "` + ed + ` "` + path + `""`}
	return c
}
