//go:build windows

package tools

import "os/exec"

// Windows has no process groups in this sense; the marketplace client that
// builds here never runs the coder's bash. Kill the process itself.
func ownProcessGroup(*exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// OwnProcessGroup and KillProcessGroup are the same for callers outside the
// package (the gateway's streamed bash).
func OwnProcessGroup(cmd *exec.Cmd)        { ownProcessGroup(cmd) }
func KillProcessGroup(cmd *exec.Cmd) error { return killProcessGroup(cmd) }
