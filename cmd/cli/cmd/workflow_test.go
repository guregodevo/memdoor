package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// The shell's view of a run: one line per task with its glyph, its time,
// what it requires, and the reason it failed.
func TestPrintWorkflowTasks(t *testing.T) {
	r := workflowRunStatus{Tasks: []workflowTaskStatus{
		{Name: "tests", State: "skipped"},
		{Name: "notes", State: "done", Took: "11s", Requires: []string{"tests"}},
		{Name: "approve", State: "waiting", External: true},
		{Name: "announce", State: "failed", Error: "exit 1", Requires: []string{"notes", "approve"}},
	}}
	old := os.Stdout
	rd, w, _ := os.Pipe()
	os.Stdout = w
	printWorkflowTasks(r)
	w.Close()
	os.Stdout = old
	var b bytes.Buffer
	io.Copy(&b, rd)
	out := b.String()
	for _, want := range []string{"✓ tests     skipped", "✓ notes     done 11s  ← tests", "○ approve   waiting  (external)", "✗ announce  failed  ← notes, approve", "      exit 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// run --wait ends on the run's outcome; a gate ends it only with
// --stop-at-gate (a CI job has nobody to approve), as exit 3.
func TestWorkflowWaitEnds(t *testing.T) {
	gate := workflowRunStatus{State: "waiting", Tasks: []workflowTaskStatus{
		{Name: "build", State: "done"},
		{Name: "approve", State: "waiting", External: true},
	}}
	cases := []struct {
		st         workflowRunStatus
		stopAtGate bool
		code       int
		end        bool
	}{
		{workflowRunStatus{State: "running"}, true, 0, false},
		{workflowRunStatus{State: "done"}, false, workflowExitDone, true},
		{workflowRunStatus{State: "failed"}, false, workflowExitFailed, true},
		{workflowRunStatus{State: "stopped"}, false, workflowExitStopped, true},
		{gate, false, workflowExitGate, false},
		{gate, true, workflowExitGate, true},
	}
	for _, c := range cases {
		code, end := workflowWaitEnds(c.st, c.stopAtGate)
		if code != c.code || end != c.end {
			t.Errorf("%s stopAtGate=%v: got (%d, %v), want (%d, %v)", c.st.State, c.stopAtGate, code, end, c.code, c.end)
		}
	}
	if g := waitingGate(gate); g != "approve" {
		t.Errorf("waitingGate = %q, want approve", g)
	}
	if g := waitingGate(workflowRunStatus{State: "running", Tasks: gate.Tasks}); g != "" {
		t.Errorf("a running run stands at no gate, got %q", g)
	}
}

// A command task's error shows its exit status and what it said, not the
// command it repeats (mario: "task <name>: <command>: exit status 1 — …").
func TestTaskErrorText(t *testing.T) {
	got := taskErrorText("task w.steps.verdict: f=x\n[ -f \"$f\" ] || exit 0\n: exit status 1 — 1 finding(s) that must not merge:\n- **P0** `a.go:6` — wrong")
	want := "exit status 1 — 1 finding(s) that must not merge:\n- **P0** `a.go:6` — wrong"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := taskErrorText("no model is configured"); got != "no model is configured" {
		t.Fatalf("an error with no exit status changed: %q", got)
	}
}
