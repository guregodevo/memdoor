package tools

import (
	"strings"
	"testing"
)

// TestFormatTodosFollowUp: the formatter always names the next action, so a turn
// can't end with work silently pending.
func TestFormatTodosFollowUp(t *testing.T) {
	todos := []TodoItem{
		{Content: "write hello.go", Status: "completed"},
		{Content: "run it", Status: "pending"},
	}
	out := FormatTodos(todos)
	if !strings.Contains(out, "Next: run it") {
		t.Errorf("expected follow-up to point at the next pending step:\n%s", out)
	}
	if !strings.Contains(out, "(1/2 done)") {
		t.Errorf("expected progress count 1/2:\n%s", out)
	}

	// In-progress takes priority over pending.
	todos[1].Status = "in_progress"
	if out := FormatTodos(todos); !strings.Contains(out, "In progress: run it") {
		t.Errorf("in-progress should be the follow-up:\n%s", out)
	}

	// All completed → verify+done directive, not a next step. The wording must
	// stay AGENT-NEUTRAL: this line reaches every agent with a todo list, and
	// when it prescribed the coder's ending ("build/run", "append a note") the
	// next agent went hunting for a build to run and a tool it does not
	// have (live 2026-09-02).
	for i := range todos {
		todos[i].Status = "completed"
	}
	done := FormatTodos(todos)
	// The all-done line must be TERMINAL: an instruction to "verify" sent an
	// agent back into its last tool in a loop (2026-09-02).
	if !strings.Contains(done, "STOP") || strings.Contains(strings.ToLower(done), "verify the result") {
		t.Errorf("all-done must tell the agent to report and stop, not to act again:\n%s", done)
	}
	for _, coderOnly := range []string{"build/run", "append a note"} {
		if strings.Contains(done, coderOnly) {
			t.Errorf("follow-up must not prescribe the coder's tools (%q):\n%s", coderOnly, done)
		}
	}
}

// TestTaskStoreSharedByKey: two agents keyed by the same channel see one list.
func TestTaskStoreSharedByKey(t *testing.T) {
	const ch = "ws:1:channel:demo"
	SetTasks(ch, []TodoItem{{Content: "step 1", Status: "pending"}})
	if got := GetTasks(ch); len(got) != 1 || got[0].Content != "step 1" {
		t.Fatalf("shared read failed: %+v", got)
	}
	// A different key is isolated.
	if got := GetTasks("ws:1:channel:other"); got != nil {
		t.Fatalf("expected isolation, got %+v", got)
	}
}
