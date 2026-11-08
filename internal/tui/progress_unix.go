//go:build !windows

package tui

import (
	"os/exec"
	"syscall"
)

// inGroup starts c in a process group of its own, so killGroup stops what
// it started too (ssh, git...), not only the shell.
func inGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(c *exec.Cmd) {
	if c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
	}
}
