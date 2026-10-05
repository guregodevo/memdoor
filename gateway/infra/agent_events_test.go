package infra

import "testing"

// TestEmitEventDeliversInOrder pins the ordering guarantee that the WS stream
// depends on: events carry monotonic per-run seq numbers and must reach a
// listener in that order. The previous implementation fired a goroutine per
// event, so delivery raced — seq N+1 could land before seq N, and the listener
// append below was a data race (run with -race). Synchronous, ordered delivery
// makes both deterministic.
func TestEmitEventDeliversInOrder(t *testing.T) {
	ee := NewEventEmitter(false)

	var got []int
	ee.OnEvent(func(e AgentEvent) { got = append(got, e.Seq) })

	const n = 500
	for i := 0; i < n; i++ {
		ee.EmitEvent("run1", EventStreamAssistant, "sess", map[string]interface{}{"i": i})
	}

	if len(got) != n {
		t.Fatalf("received %d events, want %d", len(got), n)
	}
	for i := 0; i < n; i++ {
		if got[i] != i+1 {
			t.Fatalf("event %d delivered seq %d, want %d (out-of-order delivery)", i, got[i], i+1)
		}
	}
}

// TestEmitEventListenerPanicIsolated verifies one panicking listener doesn't
// break delivery to the others (or the emitting run).
func TestEmitEventListenerPanicIsolated(t *testing.T) {
	ee := NewEventEmitter(false)

	ee.OnEvent(func(e AgentEvent) { panic("boom") })
	delivered := 0
	ee.OnEvent(func(e AgentEvent) { delivered++ })

	ee.EmitEvent("run1", EventStreamLifecycle, "sess", nil)

	if delivered != 1 {
		t.Fatalf("good listener got %d events, want 1 (panic in another listener leaked)", delivered)
	}
}
