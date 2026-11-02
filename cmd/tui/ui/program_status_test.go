package ui

import (
	"math"
	"strings"
	"testing"
)

// OSC 7501: one escape per change of state, for turns, questions and workflow runs.
func TestProgramStatusFollowsTurnsAndQuestions(t *testing.T) {
	t.Setenv("TMUX", "")

	fresh := Model{}
	if cmd := (&fresh).programStatusCmd(); cmd != nil {
		t.Error("a window that has run nothing reports no state")
	}

	m := Model{turnRunning: true}
	if got := m.programState(); got != "working" {
		t.Errorf("a running turn is working, got %q", got)
	}
	if (&m).programStatusCmd() == nil {
		t.Fatal("the first state must be sent")
	}
	if (&m).programStatusCmd() != nil {
		t.Error("an unchanged state must not be sent again")
	}

	m.pendingQuestion = &pendingQuestion{}
	if got := programStatusSeq(m.programState()); got != "\x1b]7501;state=blocked:app=memdoor:kind=question\x07" {
		t.Errorf("a question blocks with kind=question, got %q", got)
	}

	m.pendingQuestion, m.turnRunning = nil, false
	(&m).turnEnded()
	if got := programStatusSeq(m.programState()); got != "\x1b]7501;state=done:app=memdoor\x07" {
		t.Errorf("a finished turn is done, got %q", got)
	}
}

func TestProgramStatusFollowsAWorkflowRun(t *testing.T) {
	m := Model{}
	event := func(run WorkflowRun) {
		(&m).updateWorkflowPanel(workflowEventMsg{runID: run.ID, state: "started", status: &run})
	}
	step := func(state string) []WorkflowTask { return []WorkflowTask{{Name: "build", State: state}} }

	event(WorkflowRun{ID: "r1", State: "running", Tasks: step("running")})
	if got := m.programState(); got != "working" {
		t.Errorf("a running step is working, got %q", got)
	}
	event(WorkflowRun{ID: "r1", State: "waiting", Tasks: step("done")})
	if got := m.programState(); got != "blocked" {
		t.Errorf("a gate blocks, got %q", got)
	}
	event(WorkflowRun{ID: "r1", State: "running", Tasks: step("failed")})
	if got := m.programState(); got != "error" {
		t.Errorf("a failed step is an error while the run goes on, got %q", got)
	}
	event(WorkflowRun{ID: "r1", State: "failed", Tasks: step("failed")})
	if got := m.programState(); got != "error" {
		t.Errorf("a failed run stays an error, got %q", got)
	}

	event(WorkflowRun{ID: "r2", State: "running", Tasks: step("running")})
	event(WorkflowRun{ID: "r2", State: "done", Tasks: step("done")})
	if got := m.programState(); got != "done" {
		t.Errorf("a finished run is done, got %q", got)
	}
}

func TestProgramStatusGoesThroughTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	want := "\x1bPtmux;\x1b\x1b]7501;state=working:app=memdoor\x07\x1b\\"
	if got := programStatusSeq("working"); got != want {
		t.Errorf("inside tmux the escape goes through DCS passthrough, got %q", got)
	}
}

func TestProgramStatusIsClearedWhenTheWindowCloses(t *testing.T) {
	t.Setenv("TMUX", "")
	defer func() { programStatusWritten = 0 }()

	var out strings.Builder
	programStatusWritten = 0
	programStatusClear(&out)
	if out.Len() != 0 {
		t.Errorf("a window that sent no state clears nothing, got %q", out.String())
	}

	programStatusWritten = 3
	programStatusClear(&out)
	if want := "\x1b]7501;state=clear\x1b\\"; out.String() != want {
		t.Errorf("closing clears the state, got %q, want %q", out.String(), want)
	}
	if writeProgramStatus(programStatusSeq("working"))(); programStatusWritten != math.MaxUint64 {
		t.Error("a status queued before the close must not land after the clear")
	}
}
