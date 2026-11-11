//go:build !windows

package tools

import (
	"os/exec"
	"syscall"

	"memdoor/pkg/shell"
)

// ownProcessGroup starts the command in its own process group, so a kill
// reaches the children (a find or sleep bash spawned), not only bash. The
// person's own shell (MEMDOOR_SHELL) gets a session of its own instead: a new
// session is also a new process group, with no controlling terminal for an
// interactive zsh to take, and Setpgid on top of Setsid fails (EPERM).
func ownProcessGroup(cmd *exec.Cmd) {
	if shell.Detached() {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the command and everything it spawned.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// OwnProcessGroup and KillProcessGroup are the same for callers outside the
// package (the gateway's streamed bash).
func OwnProcessGroup(cmd *exec.Cmd)        { ownProcessGroup(cmd) }
func KillProcessGroup(cmd *exec.Cmd) error { return killProcessGroup(cmd) }
