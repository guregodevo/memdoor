package consumer

import (
	"fmt"
	"testing"

	"memdoor/gateway/infra"
)

// Streamed chunks must reach the handler in EVENT ORDER.
//
// ProcessEvent fired a goroutine per event. Events carry monotonic sequence
// numbers, but a goroutine each let seq N+1 reach the handler before seq N —
// and the handler appends the chunk to the transcript, so the text came out
// scrambled: "program" rendered as "ograprm", "</anthropic>" as "</anthrpioc>",
// adjacent chunks swapping across a whole reply (2026-08-30).
//
// Invisible until streamed deltas arrived: with a handful of coarse events per
// turn the race almost never lost; at 45 tokens a second it always does.
//
// Run with -race to catch the concurrent handler calls as well as the order.
func TestDeltasReachTheHandlerInOrder(t *testing.T) {
	var got string
	c := NewConsumer(func(e *ProcessedEvent) { got += e.TextDelta }, ConsumerOptions{})

	// One character per event, which is what a token stream looks like.
	want := "the quick brown fox jumps over the lazy dog"
	for i, ch := range want {
		ev := infra.AgentEvent{
			RunID:     "run-1",
			SessionID: "sess-1",
			Seq:       i + 1,
			Stream:    infra.EventStreamAssistant,
			Data: map[string]interface{}{
				"event": "text_delta",
				"delta": string(ch),
			},
		}
		if err := c.ProcessEvent(ev); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}

	if got != want {
		t.Errorf("the transcript must assemble in order.\n got: %q\nwant: %q\n"+
			"scrambled output here is the goroutine-per-event race", got, want)
	}
}

// The handler must not be able to take down the read loop.
func TestHandlerPanicIsContained(t *testing.T) {
	c := NewConsumer(func(e *ProcessedEvent) { panic("handler exploded") }, ConsumerOptions{})

	err := c.ProcessEvent(infra.AgentEvent{
		RunID: "run-2", SessionID: "s", Seq: 1,
		Stream: infra.EventStreamAssistant,
		Data:   map[string]interface{}{"event": "text_delta", "delta": "x"},
	})
	if err != nil {
		t.Fatalf("a panicking handler must not surface as an error: %v", err)
	}
	_ = fmt.Sprint(err)
}
