package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeCmux records every call it gets, one line each, and returns the reader.
func fakeCmux(t *testing.T) func() string {
	t.Helper()
	if runtime.GOOS == "windows" {
		// cmux is a macOS app, and its stand-in here is a /bin/sh script:
		// Windows has neither the app nor a shell that runs it.
		t.Skip("cmux is macOS only")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "cmux")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CMUX_BUNDLED_CLI_PATH", bin)
	t.Setenv("CMUX_WORKSPACE_ID", "ws-1")
	cmuxShown = [2]string{}
	return func() string {
		raw, _ := os.ReadFile(log)
		_ = os.Remove(log)
		return string(raw)
	}
}

func wantCalls(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("cmux was not told %q; calls:\n%s", w, got)
		}
	}
}

// The pill says what runs, a question, a gate and a failure, each in its
// own colour; a workflow run adds the progress bar (#208).
func TestCmuxSidebarSaysWhatRuns(t *testing.T) {
	cases := []struct {
		name   string
		status windowStatus
		want   []string
	}{
		{"a tool", windowStatus{state: statusWorking, detail: "Running Bash"},
			[]string{"set-status memdoor Running Bash --icon bolt.fill --color " + cmuxBlue, "clear-progress"}},
		{"nothing named", windowStatus{state: statusWorking},
			[]string{"set-status memdoor Running --icon bolt.fill"}},
		{"a workflow step", windowStatus{state: statusWorking, detail: "team-solve · plan", done: 3, total: 18},
			[]string{"set-status memdoor team-solve · plan", "set-progress 0.17 --label 3/18"}},
		{"a question", windowStatus{state: statusBlocked, kind: blockedByQuestion},
			[]string{"set-status memdoor Needs input --icon bell.fill --color " + cmuxOrange}},
		{"a gate", windowStatus{state: statusBlocked, detail: "approve", done: 2, total: 3},
			[]string{"set-status memdoor Waiting for approval · approve --icon pause.circle.fill --color " + cmuxOrange, "set-progress 0.67 --label 2/3"}},
		{"a failure", windowStatus{state: statusError, detail: "build", done: 1, total: 3},
			[]string{"set-status memdoor Failed · build --icon exclamationmark.triangle.fill --color " + cmuxRed, "clear-progress"}},
		{"done", windowStatus{state: statusDone},
			[]string{"clear-status memdoor", "clear-progress"}},
	}
	for _, c := range cases {
		var lines []string
		for _, call := range cmuxSidebar(c.status) {
			lines = append(lines, strings.Join(call, " "))
		}
		got := strings.Join(lines, "\n")
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: want %q in\n%s", c.name, w, got)
			}
		}
	}
}

// A new turn clears the last turn's notification; the pill follows the turn
// through a tool and a question, and the end clears it and rings when the
// person is away (the maintainer: "it keeps saying turn finished … it's a
// different turn and it's running").
func TestCmuxFollowsTheTurn(t *testing.T) {
	calls := fakeCmux(t)

	fresh := Model{}
	if (&fresh).cmuxStatusCmd() != nil {
		t.Error("a window that has run nothing draws nothing")
	}

	runAll(cmuxTurnStarted())
	m := Model{turnRunning: true, blurred: true, activeTools: map[string]time.Time{"bash": time.Now()}}
	runAll((&m).cmuxStatusCmd())
	if (&m).cmuxStatusCmd() != nil {
		t.Error("an unchanged status is not sent again")
	}
	wantCalls(t, calls(), "clear-notifications --workspace ws-1", "set-status memdoor Running "+displayName("bash"))

	m.activeTools, m.isThinking = nil, true
	runAll((&m).cmuxStatusCmd())
	wantCalls(t, calls(), "set-status memdoor Thinking")

	m.pendingQuestion = &pendingQuestion{}
	runAll((&m).cmuxStatusCmd())
	wantCalls(t, calls(), "set-status memdoor Needs input")

	m.pendingQuestion, m.isThinking, m.turnRunning = nil, false, false
	runAll((&m).turnEnded())
	runAll((&m).cmuxStatusCmd())
	end := calls()
	wantCalls(t, end, "notify --title Memdoor: turn finished", "clear-status memdoor --workspace ws-1")
	if strings.Contains(end, "progress") {
		t.Errorf("a turn with no run never drew a bar: only the pill changes, got\n%s", end)
	}

	// Outside cmux nothing is run.
	t.Setenv("CMUX_WORKSPACE_ID", "")
	if cmuxTurnStarted() != nil || cmuxRun("x") != nil {
		t.Error("outside cmux there is nothing to run")
	}
	m.turnRunning = true
	if (&m).cmuxStatusCmd() != nil {
		t.Error("outside cmux the sidebar is not drawn")
	}
}

// A question rings for a person who is away: the turn waits on them.
func TestAQuestionRings(t *testing.T) {
	calls := fakeCmux(t)
	m := Model{turnRunning: true, blurred: true}
	_, cmd := m.Update(questionMsg{id: "q1", question: "Which file?"})
	runAll(cmd)
	wantCalls(t, calls(), "notify --title Memdoor: "+titleDirName()+" needs your answer")
}

// A run at a gate waits for the person: the pill says so and the ring does
// not call it finished.
func TestAGateIsNotAFinishedTurn(t *testing.T) {
	tasks := []WorkflowTask{{Name: "build", State: "done"}, {Name: "approve", External: true, State: "waiting", Requires: []string{"build"}}}
	m := Model{blurred: true, workflowRunning: "r1", workflowLast: map[string]WorkflowRun{
		"r1": {ID: "r1", Workflow: "release", State: "waiting", Tasks: tasks, Done: 1, Total: 2},
	}}
	s := m.windowStatus()
	if s.state != statusBlocked || s.kind != "" || s.detail != "approve" {
		t.Fatalf("a gate blocks on its step, got %+v", s)
	}
	if got := s.endNote("proj"); got != "Memdoor: proj waits for approval at approve" {
		t.Errorf("the ring at a gate, got %q", got)
	}
	if got := m.programState(); got != "blocked" {
		t.Errorf("OSC 7501 at a gate stays blocked, got %q", got)
	}
}

// A run that failed stays failed after the turn's end, and the next turn
// starts clean. Before, turnEnded wrote "done" over the run's "error".
func TestAFailedRunStaysFailed(t *testing.T) {
	calls := fakeCmux(t)
	m := Model{blurred: true}
	event := func(run WorkflowRun) {
		(&m).updateWorkflowPanel(workflowEventMsg{runID: run.ID, state: "started", status: &run})
	}
	event(WorkflowRun{ID: "r1", Workflow: "release", State: "running", Tasks: []WorkflowTask{{Name: "build", State: "running"}}, Total: 1})
	if s := m.windowStatus(); s.state != statusWorking || s.detail != "release · build" {
		t.Fatalf("a running step is named with its run, got %+v", s)
	}
	event(WorkflowRun{ID: "r1", Workflow: "release", State: "failed", Tasks: []WorkflowTask{{Name: "build", State: "failed"}}, Total: 1})
	runAll((&m).turnEnded())
	if got := m.programState(); got != "error" {
		t.Errorf("the turn's end keeps the run's failure, got %q", got)
	}
	runAll((&m).cmuxStatusCmd())
	wantCalls(t, calls(), "notify --title Memdoor: build failed in", "set-status memdoor Failed · build")

	next, _ := m.Update(workflowEventMsg{runID: "r2", state: "started", status: &WorkflowRun{ID: "r2", Workflow: "release", State: "running"}})
	m = next.(Model)
	m.workflowRunning, m.workflowLast = "", nil
	if got := m.programState(); got != "" {
		t.Errorf("a new run clears the last failure, got %q", got)
	}
}

// The pill has no clock: a long thought is one cmux call, not one a second.
func TestThePillHasNoClock(t *testing.T) {
	m := Model{turnRunning: true, isThinking: true, thinkingStartTime: time.Now()}
	first := m.windowStatus()
	m.thinkingStartTime = time.Now().Add(-time.Minute)
	if m.windowStatus() != first {
		t.Errorf("the status changed with the clock: %+v then %+v", first, m.windowStatus())
	}
	m.isThinking = false
	m.subagentWork = map[string]subagentWorkState{"s1": {agent: "coder", tool: "bash", seconds: 10}}
	a := m.windowStatus()
	m.subagentWork["s1"] = subagentWorkState{agent: "coder", tool: "bash", seconds: 70}
	if m.windowStatus() != a || a.detail != "coder · "+displayName("bash") {
		t.Errorf("a subagent's seconds are not in the pill: %+v then %+v", a, m.windowStatus())
	}
}

// A newer write that leaves the bar as it was, landing before an older one
// that drew it, still leaves the bar drawn: the older one is dropped, so the
// newer one carries the whole sidebar (found driving the TUI: a run's 0/2
// bar never showed).
func TestTheBarSurvivesWritesOutOfOrder(t *testing.T) {
	calls := fakeCmux(t)
	m := Model{lastCmux: "x", workflowRunning: "r1", workflowLast: map[string]WorkflowRun{"r1": {ID: "r1", Workflow: "pass", State: "running", Total: 2}}}
	older := (&m).cmuxStatusCmd()
	m.workflowLast["r1"] = WorkflowRun{ID: "r1", Workflow: "pass", State: "running", Total: 2, Tasks: []WorkflowTask{{Name: "one", State: "running"}}}
	newer := (&m).cmuxStatusCmd()
	newer()
	older()
	wantCalls(t, calls(), "set-status memdoor pass · one", "set-progress 0.00 --label 0/2")
}

// An older write never lands after a newer one, and none after the close.
func TestLatestWritesKeepsOrder(t *testing.T) {
	var l latestWrites
	var got []string
	older := l.cmd(func() { got = append(got, "working") })
	newer := l.cmd(func() { got = append(got, "done") })
	newer()
	older()
	if strings.Join(got, ",") != "done" {
		t.Errorf("an older write landed after a newer one: %v", got)
	}
	late := l.cmd(func() { got = append(got, "late") })
	sent := false
	l.close(func(s bool) { sent = s })
	late()
	if !sent || len(got) != 1 {
		t.Errorf("close: sent %v, writes %v", sent, got)
	}
}

// runAll runs a command and every command a batch or a sequence holds, in
// order. tea's sequence message is unexported: it is read as a slice of Cmd.
func runAll(c tea.Cmd) {
	if c == nil {
		return
	}
	v := reflect.ValueOf(c())
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeOf(tea.Cmd(nil)) {
		return
	}
	for i := 0; i < v.Len(); i++ {
		runAll(v.Index(i).Interface().(tea.Cmd))
	}
}
