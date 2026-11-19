package alert

import (
	"os/exec"
	"syscall"
)

// hide keeps a console program (PowerShell) from flashing a window when
// katTasks-gui or the background reminder starts it.
func hide(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
