package ui

import (
	"context"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// CMUX KEEPS A NOTIFICATION UNTIL IT IS READ (Greg, 2026-09-27: "it keeps
// saying turn finished … right now it's a different turn and it's running").
// An OSC 9 cannot be taken back, so inside cmux the window speaks cmux's own
// language instead, the way its agent integrations do: a status beside the
// workspace while a turn runs, the notification cleared when the next turn
// starts, and the end of a turn sent through `cmux notify`.
//
// Outside cmux none of this runs and the title plus BEL/OSC 9 do the work.

// cmuxCLI is cmux's command and this window's workspace, or "" outside cmux.
func cmuxCLI() (bin, workspace string) {
	bin, workspace = os.Getenv("CMUX_BUNDLED_CLI_PATH"), os.Getenv("CMUX_WORKSPACE_ID")
	if bin == "" || workspace == "" {
		return "", ""
	}
	return bin, workspace
}

// cmuxRun runs one cmux command for this workspace, off the UI loop, and
// ignores failure: the tab's title still tells the truth without it.
func cmuxRun(args ...string) tea.Cmd {
	bin, ws := cmuxCLI()
	if bin == "" {
		return nil
	}
	return func() tea.Msg {
		// 2s was not enough to even start the process on a busy machine, so a
		// status or a notification was dropped: the suite caught it as a flake
		// (TestCmuxFollowsTheTurn, 2026-10-01), and a machine running a build
		// is exactly when a turn ends unwatched. Nothing waits on this call.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, bin, append(args, "--workspace", ws)...).Run()
		return nil
	}
}

// cmuxTurnStarted drops the last turn's notification and marks the workspace.
func cmuxTurnStarted() tea.Cmd {
	return tea.Batch(
		cmuxRun("clear-notifications"),
		cmuxRun("set-status", "memdoor", "Running", "--icon", "bolt.fill", "--color", "#4C8DFF"),
	)
}

// cmuxTurnEnded clears the status; the notification is the caller's choice.
func cmuxTurnEnded() tea.Cmd { return cmuxRun("clear-status", "memdoor") }

// CmuxClear leaves no status behind when the window closes mid-turn.
func CmuxClear() {
	bin, ws := cmuxCLI()
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, bin, "clear-status", "memdoor", "--workspace", ws).Run()
}
