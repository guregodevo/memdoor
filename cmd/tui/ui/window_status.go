package ui

import (
	"fmt"
	"strings"
)

// ONE STATUS FOR THE WINDOW, DRAWN BY EACH DOOR (#208). The title glyph
// (window_title.go), OSC 7501 (program_status.go) and cmux's sidebar (cmux.go)
// read the same value, so they cannot disagree about whether the window runs,
// waits for the person, or failed.
//
// The detail names what runs, in words that change when the activity changes
// and never on a clock: each change starts a cmux process, so a label with
// seconds in it would start one a second.

const (
	statusWorking = "working"
	statusBlocked = "blocked"
	statusError   = "error"
	statusDone    = "done"

	blockedByQuestion = "question"
)

type windowStatus struct {
	state  string // "" before anything ran, else one of the status* names (OSC 7501's words)
	kind   string // blockedByQuestion when a question blocks, "" at a workflow gate
	detail string // what runs, the gate's step, or the failed step
	done   int    // a workflow run's progress; total 0 means no run
	total  int
}

// windowStatus is this window's status now. A question, a workflow gate and a
// failed step outrank a running turn: each needs the person to act.
func (m Model) windowStatus() windowStatus {
	if m.pendingQuestion != nil {
		return windowStatus{state: statusBlocked, kind: blockedByQuestion}
	}
	if run, ok := m.workflowLast[m.workflowRunning]; ok && m.workflowRunning != "" {
		s := windowStatus{done: run.Done, total: run.Total}
		switch {
		case runHasFailedTask(run):
			s.state, s.detail = statusError, failedStep(run)
		case run.State == "waiting":
			_, gate := m.windowGate()
			s.state, s.detail = statusBlocked, gate
		default:
			s.state, s.detail = statusWorking, runningSteps(run)
		}
		return s
	}
	if m.busy() {
		return windowStatus{state: statusWorking, detail: m.activityLabel()}
	}
	return m.lastOutcome
}

// activityLabel is the footer's label (model_view.go renderThinkingBar)
// without its clock: no elapsed time, no byte count, no rotating verb.
func (m Model) activityLabel() string {
	switch {
	case m.compacting:
		return "Compacting context"
	case len(m.activeTools) == 1:
		for name := range m.activeTools {
			return "Running " + displayName(name)
		}
	case len(m.activeTools) > 1:
		return fmt.Sprintf("Running %d tools", len(m.activeTools))
	case len(m.subagentWork) == 1:
		for _, w := range m.subagentWork {
			if w.agent != "" && w.tool != "" {
				return w.agent + " · " + displayName(w.tool)
			}
		}
		return "A subagent is working"
	case len(m.subagentWork) > 1:
		return fmt.Sprintf("%d agents working", len(m.subagentWork))
	case len(m.activeSubagents) > 0:
		return "Delegating"
	case m.pendingCall != "" && m.pendingCall != pendingCallUnnamed:
		return "Writing a " + displayName(m.pendingCall) + " call"
	case m.pendingCall != "":
		return "Writing a tool call"
	case m.isThinking:
		return "Thinking"
	}
	return ""
}

// runningSteps is the run's name and the steps running in it now.
func runningSteps(r WorkflowRun) string {
	var names []string
	for _, t := range r.Tasks {
		if t.State == "running" || t.State == "retrying" {
			names = append(names, t.Name)
		}
	}
	switch len(names) {
	case 0:
		return r.Workflow
	case 1:
		return r.Workflow + " · " + names[0]
	}
	return fmt.Sprintf("%s · %d steps", r.Workflow, len(names))
}

// failedStep names the first failed step, or the run when none failed.
func failedStep(r WorkflowRun) string {
	for _, t := range r.Tasks {
		if t.State == "failed" {
			return t.Name
		}
	}
	return r.Workflow
}

// endNote is what the ring says when the window stops running and the
// person is not looking: a gate and a failure are not "finished".
func (s windowStatus) endNote(dir string) string {
	switch {
	case s.state == statusBlocked && s.kind == blockedByQuestion:
		return "Memdoor: " + dir + " needs your answer"
	case s.state == statusBlocked:
		return "Memdoor: " + dir + " waits for approval at " + s.detail
	case s.state == statusError:
		return "Memdoor: " + strings.TrimSpace(s.detail+" failed in "+dir)
	}
	return "Memdoor: turn finished in " + dir
}
