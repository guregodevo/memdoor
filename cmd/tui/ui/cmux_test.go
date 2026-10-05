package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Inside cmux a new turn clears the last turn's notification and sets a
// status; the end clears it (Greg: "it keeps saying turn finished … it's a
// different turn and it's running"). A fake cmux records what it was told.
func TestCmuxFollowsTheTurn(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "cmux")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CMUX_BUNDLED_CLI_PATH", bin)
	t.Setenv("CMUX_WORKSPACE_ID", "ws-1")

	runAll(cmuxTurnStarted())
	m := Model{blurred: true}
	runAll((&m).turnEnded())

	raw, _ := os.ReadFile(log)
	got := string(raw)
	for _, want := range []string{"clear-notifications --workspace ws-1", "set-status memdoor Running", "clear-status memdoor --workspace ws-1", "notify --title Memdoor: turn finished"} {
		if !strings.Contains(got, want) {
			t.Errorf("cmux was not told %q; calls:\n%s", want, got)
		}
	}

	// Outside cmux nothing is run.
	t.Setenv("CMUX_WORKSPACE_ID", "")
	if cmuxTurnStarted() != nil && cmuxRun("x") != nil {
		t.Error("outside cmux there is nothing to run")
	}
}

// runAll runs a command and every command a batch holds, in order.
func runAll(c tea.Cmd) {
	if c == nil {
		return
	}
	if b, ok := c().(tea.BatchMsg); ok {
		for _, sub := range b {
			runAll(sub)
		}
	}
}
