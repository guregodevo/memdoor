package consumer

import (
	"testing"

	"memdoor/gateway/infra"
)

// The runtime's error frames say {"error": …}; the processed event must
// carry that text, or the screen has nothing to show for a failed turn.
func TestErrorStreamTextReachesTheProcessedEvent(t *testing.T) {
	var got []*ProcessedEvent
	c := NewConsumer(func(e *ProcessedEvent) { got = append(got, e) }, ConsumerOptions{})
	// Two runs: one that fails on the error stream, one that fails on the
	// lifecycle stream (the consumer finalizes a run on the first and
	// drops what follows for it).
	c.ProcessEvent(infra.AgentEvent{RunID: "r1", Seq: 1, Stream: infra.EventStreamError,
		Data: map[string]interface{}{"error": "no brain is serving yet"}})
	c.ProcessEvent(infra.AgentEvent{RunID: "r2", Seq: 1, Stream: infra.EventStreamLifecycle,
		Data: map[string]interface{}{"event": "error", "error": "no brain is serving yet"}})
	if len(got) != 2 {
		t.Fatalf("want 2 processed events, got %d", len(got))
	}
	for i, e := range got {
		if e.Error != "no brain is serving yet" {
			t.Fatalf("event %d: error text %q not carried", i, e.Error)
		}
	}
}
