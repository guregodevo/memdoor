//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal starts the gateway in a session of its own. In the
// terminal's process group, the Ctrl+C that cancels a first-run question (the
// OpenRouter sign-in says "Ctrl+C cancels") stopped the gateway as well, and the
// window that should have opened next failed with "gateway unreachable"
// (2026-11-19, a fresh home on v1.833.0).
func detachFromTerminal(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
