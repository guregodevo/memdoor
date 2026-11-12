//go:build windows

package tools

import (
	"os/exec"
	"strconv"
)

// Windows has no process groups in this sense: the tree is killed by taskkill.
func ownProcessGroup(*exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Killing bash alone leaves its child (sleep, find) holding the output
	// pipe, and Run waits for it: taskkill /T takes the whole tree.
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// OwnProcessGroup and KillProcessGroup are the same for callers outside the
// package (the gateway's streamed bash).
func OwnProcessGroup(cmd *exec.Cmd)        { ownProcessGroup(cmd) }
func KillProcessGroup(cmd *exec.Cmd) error { return killProcessGroup(cmd) }
