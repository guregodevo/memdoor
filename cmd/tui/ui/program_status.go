package ui

import (
	"io"
	"os"
	"strings"

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

// programState is the window's status in OSC 7501's words (window_status.go),
// "" before anything ran.
func (m Model) programState() string {
	s := m.windowStatus()
	if s.kind != "" {
		return s.state + ":kind=" + s.kind
	}
	return s.state
}

// workflowRunEnded records how a run finished, for the state that follows it.
func (m *Model) workflowRunEnded(run WorkflowRun) {
	if run.State == "failed" || runHasFailedTask(run) {
		m.lastOutcome = windowStatus{state: statusError, detail: failedStep(run)}
		return
	}
	m.lastOutcome = windowStatus{state: statusDone}
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

// programStatusWrites keeps the escapes in order (latest_writes.go).
var programStatusWrites latestWrites

func writeProgramStatus(seq string) tea.Cmd {
	return programStatusWrites.cmd(func() { _, _ = os.Stdout.WriteString(seq) })
}

// ProgramStatusClear sends state=clear when the window closes, so the terminal
// shows no state of a program that has exited. Called after the program ends
// however it ended: /exit, Ctrl-C, SIGTERM, an error or a panic.
func ProgramStatusClear() { programStatusClear(os.Stdout) }

// programStatusClear writes the clear only when a state was sent, and drops
// every status write still queued: none of them may land after it.
func programStatusClear(w io.Writer) {
	programStatusWrites.close(func(sent bool) {
		if sent {
			_, _ = io.WriteString(w, tmuxPassthrough("\x1b]7501;state=clear\x1b\\"))
		}
	})
}
