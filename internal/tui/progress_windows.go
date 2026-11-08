//go:build windows

package tui

import "os/exec"

func inGroup(*exec.Cmd) {}

func killGroup(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
	}
}
