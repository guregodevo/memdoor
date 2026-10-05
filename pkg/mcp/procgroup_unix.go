//go:build !windows

package mcp

import (
	"os/exec"
	"syscall"
)

// An MCP server often spawns children that outlive it — a chrome-devtools
// server launches an actual browser. Killing only the server leaves those
// running, so the server gets its own process group and the group is what we
// kill.

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup signals the whole group: a negative pid means "the group
// led by pid" on Unix.
func killProcessGroup(cmd *exec.Cmd, pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
