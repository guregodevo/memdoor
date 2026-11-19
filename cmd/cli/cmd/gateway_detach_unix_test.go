//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
	"testing"
)

// Ctrl+C at a first-run question goes to the terminal's foreground process
// group. The gateway the first run starts is in a group of its own, so it never
// gets that signal.
func TestTheGatewayTheFirstRunStartsIsOutOfReachOfCtrlC(t *testing.T) {
	c := exec.Command("sleep", "30")
	detachFromTerminal(c)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Process.Kill(); _ = c.Wait() })

	pgid, err := syscall.Getpgid(c.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if pgid == syscall.Getpgrp() {
		t.Fatalf("the gateway is in this terminal's process group %d: Ctrl+C here stops it", pgid)
	}
}
