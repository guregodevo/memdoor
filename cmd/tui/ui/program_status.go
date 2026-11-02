package ui

import (
	"io"
	"math"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// OSC 7501 (Program Status Protocol) reports the window's state to a terminal
// or multiplexer that tracks it (Herdr). The title glyphs (window_title.go)
// carry the same facts for a person reading the tab.
//
//	state=working:app=memdoor                 a turn or a workflow step runs
//	state=blocked:app=memdoor:kind=question   the agent waits for an answer
//	state=blocked:app=memdoor                 a workflow run waits at an approval gate
//	state=error:app=memdoor                   a workflow step failed
//	state=done:app=memdoor                    a turn or a workflow run finished
//
// One sequence is sent per change of state. A fresh window sends nothing until
// its first turn or workflow run. When the window closes, state=clear: Herdr
// keeps the last state until the program clears it.

const programStatusApp = "memdoor"

// programState is the state for this window now, "" before anything ran.
// A question, a workflow gate and a failed workflow step outrank a running
// turn: each needs the person to act.
func (m Model) programState() string {
	if m.pendingQuestion != nil {
		return "blocked:kind=question"
	}
	if run, ok := m.workflowLast[m.workflowRunning]; ok && m.workflowRunning != "" {
		switch {
		case runHasFailedTask(run):
			return "error"
		case run.State == "waiting":
			return "blocked"
		}
		return "working"
	}
	if m.busy() {
		return "working"
	}
	return m.lastOutcome
}

// workflowRunEnded records how a run finished, for the state that follows it.
func (m *Model) workflowRunEnded(run WorkflowRun) {
	if run.State == "failed" || runHasFailedTask(run) {
		m.lastOutcome = "error"
		return
	}
	m.lastOutcome = "done"
}

func runHasFailedTask(run WorkflowRun) bool {
	for _, t := range run.Tasks {
		if t.State == "failed" {
			return true
		}
	}
	return false
}

// programStatusCmd sends the state when it has changed since the last one sent.
func (m *Model) programStatusCmd() tea.Cmd {
	state := m.programState()
	if state == "" || state == m.lastStatus {
		return nil
	}
	m.lastStatus = state
	return writeProgramStatus(programStatusSeq(state))
}

// programStatusSeq turns a state such as "blocked:kind=question" into
// \x1b]7501;state=blocked:app=memdoor:kind=question\x07, wrapped for tmux
// when $TMUX is set.
func programStatusSeq(state string) string {
	name, extra, _ := strings.Cut(state, ":")
	body := "state=" + name + ":app=" + programStatusApp
	if extra != "" {
		body += ":" + extra
	}
	return tmuxPassthrough("\x1b]7501;" + body + "\x07")
}

// Bubble Tea runs each command on its own goroutine, so an earlier "working"
// can reach the terminal after a later "done". Each command gets a number when
// it is made (programStatusMade); a write older than programStatusWritten is
// dropped.
var (
	programStatusMu      sync.Mutex
	programStatusMade    uint64
	programStatusWritten uint64
)

func writeProgramStatus(seq string) tea.Cmd {
	programStatusMu.Lock()
	programStatusMade++
	n := programStatusMade
	programStatusMu.Unlock()
	return func() tea.Msg {
		programStatusMu.Lock()
		defer programStatusMu.Unlock()
		if n < programStatusWritten {
			return nil
		}
		programStatusWritten = n
		_, _ = os.Stdout.WriteString(seq)
		return nil
	}
}

// ProgramStatusClear sends state=clear when the window closes, so the terminal
// shows no state of a program that has exited. Called after the program ends
// however it ended: /exit, Ctrl-C, SIGTERM, an error or a panic.
func ProgramStatusClear() { programStatusClear(os.Stdout) }

// programStatusClear writes the clear only when a state was sent, and drops
// every status write still queued: none of them may land after it.
func programStatusClear(w io.Writer) {
	programStatusMu.Lock()
	defer programStatusMu.Unlock()
	sent := programStatusWritten > 0 && programStatusWritten < math.MaxUint64
	programStatusWritten = math.MaxUint64
	if sent {
		_, _ = io.WriteString(w, tmuxPassthrough("\x1b]7501;state=clear\x1b\\"))
	}
}
