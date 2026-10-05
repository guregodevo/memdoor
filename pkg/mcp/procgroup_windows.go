//go:build windows

package mcp

import (
	"errors"
	"os/exec"
)

// errNoProcessGroups is returned instead of pretending a group kill happened.
var errNoProcessGroups = errors.New("process groups are not available on windows")

// Windows has no process groups in the POSIX sense and no signals to send
// them. Killing the process is what is available here; a child the MCP server
// spawned may survive it, which is a real difference in behaviour rather than
// a hidden one — the caller's fallback path already handles a failed group
// kill by killing the process alone.

func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup reports that no group kill is available, so the caller
// falls back to killing the process itself.
func killProcessGroup(cmd *exec.Cmd, pid int) error {
	return errNoProcessGroups
}
