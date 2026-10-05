package consumer

import "testing"

// The context event the gateway emits (agent_runtime_session.go
// emitContextUpdate) decodes field for field: a renamed key decodes to zero
// and /context would say nothing is in the window.
func TestContextEventCarriesTheBreakdown(t *testing.T) {
	d := &Discriminator{}
	got, err := d.parseContextData(map[string]interface{}{
		"event": "context_update", "tokens": 30_000, "limit": 114_688, "percent": 26.2,
		"system": 2_500, "tools": 4_000, "conversation": 23_500, "tool_output": 18_000, "compact_at": 68_812, "compact_rule": "thresholdTokens", "keep_recent": 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := ContextEventData{Event: "context_update", Tokens: 30_000, Limit: 114_688, Percent: 26.2,
		System: 2_500, Tools: 4_000, Conversation: 23_500, ToolOutput: 18_000, CompactAt: 68_812,
		CompactRule: "thresholdTokens", KeepRecent: 12_000}
	if got != want {
		t.Fatalf("got %+v", got)
	}
}
