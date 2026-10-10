package gateway

import "testing"

// A tool is retired for the turn after repeated failures, because the reason it
// fails ("no workspace available") does not become false mid-turn. But a tool
// that SUCCEEDS in between has proved the opposite about itself, and the
// counter ignored that: measured live 2026-08-30, todo_write failed, failed,
// SUCCEEDED, failed — and was retired on that last one, with the model's
// arguments perfectly valid every time.
func TestASuccessfulCallProvesTheToolIsNotBroken(t *testing.T) {
	const max = 3
	l := toolFailureLedger{}

	l.failed("todo_write", "no workspace available", "")
	l.failed("todo_write", "no workspace available", "")
	l.succeeded("todo_write")
	l.failed("todo_write", "no workspace available", "")

	if l.retired("todo_write", max, "") {
		t.Error("retired a tool that succeeded since its failures — one success proves it can work")
	}
}

// The case retirement exists for is untouched: a tool that never works.
func TestATooThatNeverWorksIsStillRetired(t *testing.T) {
	const max = 3
	l := toolFailureLedger{}
	for i := 0; i < max; i++ {
		if l.retired("legacy_tool", max, "") {
			t.Fatalf("retired after only %d failures, want %d", i, max)
		}
		l.failed("legacy_tool", "no workspace available", "")
	}
	if !l.retired("legacy_tool", max, "") {
		t.Errorf("a tool that failed %d times with no success was not retired", max)
	}
}

// Retirement is per tool: one broken tool must not disable the others.
func TestRetirementIsPerTool(t *testing.T) {
	const max = 3
	l := toolFailureLedger{}
	for i := 0; i < max; i++ {
		l.failed("legacy_tool", "no workspace available", "")
	}
	if !l.retired("legacy_tool", max, "") {
		t.Fatal("legacy_tool should be retired")
	}
	if l.retired("bash", max, "") {
		t.Error("bash was retired by another tool's failures")
	}
}

// A wall that moved is not the same wall: three failures against one state
// of the file retire the tool; a call against a changed file runs, and the
// count starts over (live 2026-10-10: apply_patch stayed retired on goal.py
// after a sed had changed it, and a different edit went in unguarded).
func TestARetirementLiftsWhenTheFileChanged(t *testing.T) {
	l := toolFailureLedger{}
	for range 3 {
		l.failed("apply_patch on goal.py", "changed NOTHING", "goal.py:100:1")
	}
	if !l.retired("apply_patch on goal.py", 3, "goal.py:100:1") {
		t.Fatal("three failures against the same file state must retire the tool")
	}
	if l.retired("apply_patch on goal.py", 3, "goal.py:96:2") {
		t.Fatal("the file changed since the last failure: the tool must run again")
	}
	if l.count("apply_patch on goal.py") != 0 {
		t.Fatalf("the count must start over, got %d", l.count("apply_patch on goal.py"))
	}
	// A tool with no state to measure (bash) stays retired.
	for range 3 {
		l.failed("bash", "no such command", "")
	}
	if !l.retired("bash", 3, "") {
		t.Fatal("a tool with no measured state is retired on the count alone")
	}
}
