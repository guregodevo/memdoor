package infra

import (
	"sync"
	"time"
)

// AgentEventStream represents the type of event stream
// Pattern: OpenClaw's event streaming (src/infra/agent-events.ts)
type AgentEventStream string

const (
	EventStreamLifecycle AgentEventStream = "lifecycle"
	EventStreamTool      AgentEventStream = "tool"
	EventStreamAssistant AgentEventStream = "assistant"
	EventStreamError     AgentEventStream = "error"
	EventStreamContext   AgentEventStream = "context"
)

// AgentEvent represents a single event in the agent execution stream
// Pattern: OpenClaw's AgentEventPayload
type AgentEvent struct {
	RunID     string                 `json:"run_id"`
	Seq       int                    `json:"seq"`
	Stream    AgentEventStream       `json:"stream"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	SessionID string                 `json:"session_id,omitempty"`
}

// EventListener is a function that receives agent events
type EventListener func(event AgentEvent)

// EventEmitter manages event listeners and emits events in order
// Pattern: OpenClaw's event emitter with monotonic sequence numbers
type EventEmitter struct {
	listeners []EventListener
	seqByRun  map[string]int
	mu        sync.RWMutex
	verbose   bool
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter(verbose bool) *EventEmitter {
	return &EventEmitter{
		listeners: make([]EventListener, 0),
		seqByRun:  make(map[string]int),
		verbose:   verbose,
	}
}

// OnEvent registers a new event listener
// Returns a function to unregister the listener
// Pattern: OpenClaw's onAgentEvent
func (ee *EventEmitter) OnEvent(listener EventListener) func() {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	ee.listeners = append(ee.listeners, listener)

	// Return unregister function
	return func() {
		ee.mu.Lock()
		defer ee.mu.Unlock()

		for i, l := range ee.listeners {
			// Compare function pointers (this is a simple approach)
			if &l == &listener {
				ee.listeners = append(ee.listeners[:i], ee.listeners[i+1:]...)
				break
			}
		}
	}
}

// EmitEvent emits an event to all registered listeners
// Pattern: OpenClaw's emitAgentEvent with sequence numbers
func (ee *EventEmitter) EmitEvent(runID string, stream AgentEventStream, sessionID string, data map[string]interface{}) {
	ee.mu.Lock()

	// Get next sequence number for this run
	nextSeq := ee.seqByRun[runID] + 1
	ee.seqByRun[runID] = nextSeq

	// Create enriched event
	event := AgentEvent{
		RunID:     runID,
		Seq:       nextSeq,
		Stream:    stream,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
		SessionID: sessionID,
	}

	// Copy listeners to avoid holding the lock during callbacks.
	listenersCopy := make([]EventListener, len(ee.listeners))
	copy(listenersCopy, ee.listeners)

	ee.mu.Unlock()

	// Notify listeners SYNCHRONOUSLY, in emit order. The previous code fired a
	// goroutine PER EVENT ("fire-and-forget"): a run's events get monotonic seq
	// numbers assigned under the lock above, but a goroutine per event let seq
	// N+1 be delivered before seq N — the broadcaster's marshal/send then ran
	// concurrently and the client reassembled an out-of-order, corrupted stream
	// (and it was a data race on the shared broadcast path). Ordered, serial
	// delivery is required. It can't stall the run: the WS broadcaster's Send is
	// non-blocking (it drops when the per-client buffer is full), so one slow
	// client can't block the emitter.
	for _, listener := range listenersCopy {
		func(l EventListener) {
			// One bad listener must not break the others or the run.
			defer func() { _ = recover() }()
			l(event)
		}(listener)
	}
}

// ClearRun removes sequence tracking for a completed run
// Should be called when a run completes to avoid memory leaks
func (ee *EventEmitter) ClearRun(runID string) {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	delete(ee.seqByRun, runID)
}

// ListenerCount returns the number of registered listeners
func (ee *EventEmitter) ListenerCount() int {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	return len(ee.listeners)
}
