//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal starts the gateway in a process group of its own: Windows
// sends Ctrl+C to no process of a new group, so the Ctrl+C that cancels a
// first-run question no longer stops the gateway with it.
func detachFromTerminal(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
