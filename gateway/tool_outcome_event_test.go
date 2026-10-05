package gateway

import (
	"testing"
	"time"

	"memdoor/gateway/logs"
)

// EventToolFailed was declared in logs/event.go and emitted by nothing, so
// `memdoor logs query --regex tool_failed` could never answer "what is my tool
// failure rate". Production traces of 761M coding-agent calls (Liu et al.) put
// tool failures at 9% of turns and show them amplifying compute ~4x through
// retry loops, which makes it the largest cost line the harness itself
// controls — and it was the one number not being recorded.
//
// The failure path already populated ToolExecutionInfo.Error everywhere. Only
// the emit was missing.
func TestToolOutcomeEventTypesAreDistinct(t *testing.T) {
	if logs.EventToolFailed == logs.EventToolCompleted {
		t.Fatal("a failure and a success must be separable in the log store")
	}
	for _, ty := range []logs.EventType{logs.EventToolFailed, logs.EventToolCompleted} {
		if string(ty) == "" {
			t.Errorf("event type is empty — logs query has nothing to match on")
		}
	}
}

// A failure event must carry the tool name, the reason, and be queryable as a
// failure. A "something went wrong" line that names no tool cannot produce a
// per-tool failure rate, which is the whole point.
func TestToolFailureEventCarriesWhatARateNeeds(t *testing.T) {
	ev := toolOutcomeEvent(
		ToolExecutionInfo{Name: "bash", Error: "exit status 1"}, "s1", "r1", 250*time.Millisecond)

	if ev.Type != logs.EventToolFailed {
		t.Errorf("type = %v", ev.Type)
	}
	if ev.Success {
		t.Error("a failure must not be recorded as a success — it would vanish from any error filter")
	}
	if ev.Data["tool"] != "bash" {
		t.Errorf("the tool name is what a per-tool rate groups by, got %v", ev.Data["tool"])
	}
	if ev.Data["error"] == nil {
		t.Error("the reason must survive; a rate without causes cannot be acted on")
	}
	if ev.Session != "s1" || ev.RunID != "r1" {
		t.Errorf("session/run scope lost: %q %q", ev.Session, ev.RunID)
	}
}

// A SUCCESS must not be recorded as a failure, or the rate is 100% and
// meaningless. The two paths differ only by info.Error, so they are asserted
// together.
func TestToolSuccessIsNotCountedAsAFailure(t *testing.T) {
	ok := toolOutcomeEvent(ToolExecutionInfo{Name: "read_file"}, "s1", "r1", time.Second)
	if ok.Type != logs.EventToolCompleted {
		t.Errorf("a clean call must be EventToolCompleted, got %v", ok.Type)
	}
	if !ok.Success {
		t.Error("a clean call must record Success")
	}
	if ok.Level == logs.LevelWarn {
		t.Error("a clean call must not be a warning — it would pollute the error filter")
	}
	if ok.Data["tool"] != "read_file" {
		t.Errorf("the tool name must be present on successes too, or a RATE has no denominator")
	}
	// The denominator is the point: without completed events you can count
	// failures but never divide by anything.
	if ok.Data["tool"] == nil || toolOutcomeEvent(ToolExecutionInfo{Name: "bash", Error: "x"}, "s", "r", 0).Data["tool"] == nil {
		t.Error("both outcomes must carry the tool name to be groupable")
	}
}
