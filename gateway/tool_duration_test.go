package gateway

import "testing"

// agentResponseToExecutionResult hardcoded DurationMs: 0 for every tool, so the
// Published-Interface ExecutionResult always reported zero-duration tool calls.
// executeTool now stamps the real wall time onto ToolExecutionInfo; this proves
// the adapter carries it through instead of zeroing it.
func TestExecutionResultCarriesToolDuration(t *testing.T) {
	a := &AgentExecutorAdapter{}
	resp := &AgentResponse{
		Text: "done",
		ToolsExecuted: []ToolExecutionInfo{
			{Name: "bash", DurationMs: 42},
			{Name: "apply_patch", DurationMs: 0}, // a blocked/instant call stays 0 — honest
		},
	}

	res := a.agentResponseToExecutionResult(resp)

	if len(res.ToolsExecuted) != 2 {
		t.Fatalf("expected 2 tool executions, got %d", len(res.ToolsExecuted))
	}
	if res.ToolsExecuted[0].DurationMs != 42 {
		t.Errorf("tool duration not propagated: got %d, want 42", res.ToolsExecuted[0].DurationMs)
	}
	if res.ToolsExecuted[0].Name != "bash" {
		t.Errorf("name mismatch: %q", res.ToolsExecuted[0].Name)
	}
}
