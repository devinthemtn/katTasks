//go:build !windows

package alert

import "os/exec"

func hide(*exec.Cmd) {}
