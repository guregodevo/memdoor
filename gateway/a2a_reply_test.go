package gateway

import (
	"errors"
	"testing"
)

// The queue hands a job's ResponseWriter the turn's *AgentResponse, or the
// error when the turn failed. The agent-to-agent delivery writer read only
// a map, so every delivered answer became "invalid response format".
func TestA2AReplyOfReadsWhatTheQueueSends(t *testing.T) {
	text, err := a2aReplyOf(&AgentResponse{Text: "the answer"})
	if err != nil || text != "the answer" {
		t.Fatalf("agent response: got %q, %v", text, err)
	}
	if _, err := a2aReplyOf(&AgentResponse{Error: "model down"}); err == nil || err.Error() != "model down" {
		t.Fatalf("agent response with error: got %v", err)
	}
	if _, err := a2aReplyOf(errors.New("turn failed")); err == nil || err.Error() != "turn failed" {
		t.Fatalf("error: got %v", err)
	}
	if _, err := a2aReplyOf((*AgentResponse)(nil)); err == nil {
		t.Fatal("nil response: want an error")
	}
	text, err = a2aReplyOf(map[string]interface{}{"text": "legacy"})
	if err != nil || text != "legacy" {
		t.Fatalf("map: got %q, %v", text, err)
	}
	if _, err := a2aReplyOf(42); err == nil {
		t.Fatal("unknown type: want an error")
	}
}
