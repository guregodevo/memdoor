package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// CMUX KEEPS A NOTIFICATION UNTIL IT IS READ (the maintainer, 2026-09-27: "it keeps
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
	if bin, _ := cmuxCLI(); bin == "" {
		return nil
	}
	return func() tea.Msg {
		cmuxExec(30*time.Second, args...)
		return nil
	}
}

// cmuxExec runs one cmux command and waits for it. 2s was not enough to even
// start the process on a busy machine, so a status or a notification was
// dropped: the suite caught it as a flake (TestCmuxFollowsTheTurn,
// 2026-10-01), and a machine running a build is exactly when a turn ends
// unwatched. Nothing on the UI loop waits on this call.
func cmuxExec(limit time.Duration, args ...string) {
	bin, ws := cmuxCLI()
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	_ = exec.CommandContext(ctx, bin, append(args, "--workspace", ws)...).Run()
}

// cmuxTurnStarted drops the last turn's notification. The status beside the
// workspace is drawn from the window's status (cmuxStatusCmd).
func cmuxTurnStarted() tea.Cmd { return cmuxRun("clear-notifications") }

// THE SIDEBAR SAYS WHAT RUNS (#208). One pill under the key "memdoor", drawn
// from windowStatus: what runs in blue, a question or a gate in orange, a
// failure in red until the next turn. A workflow run adds cmux's progress bar.
// Done clears both.
const (
	cmuxStatusKey = "memdoor"
	cmuxBlue      = "#4C8DFF"
	cmuxOrange    = "#FF9500"
	cmuxRed       = "#FF3B30"
)

// cmuxSidebar is the two cmux commands that draw s: the pill, then the bar.
func cmuxSidebar(s windowStatus) [][]string {
	pill := func(value, icon, color string) []string {
		return []string{"set-status", cmuxStatusKey, value, "--icon", icon, "--color", color}
	}
	var status []string
	switch {
	case s.state == statusWorking:
		status = pill(firstNonEmpty(s.detail, "Running"), "bolt.fill", cmuxBlue)
	case s.state == statusBlocked && s.kind == blockedByQuestion:
		status = pill("Needs input", "bell.fill", cmuxOrange)
	case s.state == statusBlocked:
		status = pill(joinDetail("Waiting for approval", s.detail), "pause.circle.fill", cmuxOrange)
	case s.state == statusError:
		status = pill(joinDetail("Failed", s.detail), "exclamationmark.triangle.fill", cmuxRed)
	default:
		return [][]string{{"clear-status", cmuxStatusKey}, {"clear-progress"}}
	}
	progress := []string{"clear-progress"}
	if s.total > 0 && s.state != statusError {
		progress = []string{"set-progress", fmt.Sprintf("%.2f", float64(s.done)/float64(s.total)), "--label", fmt.Sprintf("%d/%d", s.done, s.total)}
	}
	return [][]string{status, progress}
}

func joinDetail(head, detail string) string {
	if detail == "" {
		return head
	}
	return head + " · " + detail
}

// cmuxWrites keeps the sidebar's writes in order: a "Running" made before a
// clear never lands after it (latest_writes.go). cmuxShown is what cmux
// shows now, the pill and the bar, touched only inside a write: each write
// carries the whole sidebar and sends the part that differs. Deciding that on
// the UI loop instead lost the bar when a newer write, which left the bar
// out, landed first and the older one that carried it was dropped.
var (
	cmuxWrites latestWrites
	cmuxShown  [2]string
)

// cmuxStatusCmd draws the window's status in cmux's sidebar when it has
// changed. A window that has run nothing draws nothing.
func (m *Model) cmuxStatusCmd() tea.Cmd {
	if bin, _ := cmuxCLI(); bin == "" {
		return nil
	}
	s := m.windowStatus()
	if s.state == "" && m.lastCmux == "" {
		return nil
	}
	calls := cmuxSidebar(s)
	key := fmt.Sprint(calls)
	if key == m.lastCmux {
		return nil
	}
	m.lastCmux = key
	return cmuxWrites.cmd(func() {
		for i, c := range calls {
			if k := strings.Join(c, "\x00"); k != cmuxShown[i] {
				cmuxExec(30*time.Second, c...)
				cmuxShown[i] = k
			}
		}
	})
}

// CmuxClear leaves no status behind when the window closes mid-turn, and
// drops every sidebar write still queued.
func CmuxClear() {
	if bin, _ := cmuxCLI(); bin == "" {
		return
	}
	cmuxWrites.close(func(bool) {
		cmuxExec(2*time.Second, "clear-status", cmuxStatusKey)
		cmuxExec(2*time.Second, "clear-progress")
	})
}
