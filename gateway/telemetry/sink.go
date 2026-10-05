package telemetry

import (
	"sync"

	"memdoor/gateway/logs"
)

// Sink is the buffer the storage wrapper enqueues events into. The
// flusher drains the sink periodically and ships batches to the
// monitoring server. Interface so tests can substitute a no-op or
// counter-only impl and the wrapper doesn't need to know about the
// disk/network details.
type Sink interface {
	// Enqueue accepts a batch of events for eventual transmission.
	// Must be safe to call from any goroutine. Implementations are
	// expected to silently drop events when the buffer is full
	// rather than blocking the caller — the wrapped logger never
	// becomes a hot path.
	Enqueue(events []*logs.Event)

	// Drain atomically returns up to max pending events and removes
	// them from the buffer. Called by the flusher when it's about
	// to POST a batch. Returns an empty slice when nothing is
	// pending.
	Drain(max int) []*logs.Event

	// Stats reports the sink's current state (tests read it; a status
	// command is planned, not built).
	Stats() SinkStats
}

// SinkStats is the operator-facing view of the sink's state.
type SinkStats struct {
	// Pending counts events buffered but not yet sent.
	Pending int

	// Dropped counts events the sink had to discard because the
	// buffer was full at Enqueue time. Non-zero indicates the
	// flusher isn't keeping up with emission (network down,
	// endpoint failing, etc.).
	Dropped int64

	// Capacity is the configured upper bound on Pending. Operators
	// can compare Pending / Capacity to see how close to the
	// drop-cliff they are.
	Capacity int
}

// LevelFilter wraps a Sink and only enqueues events at or above the
// configured minimum level. The standard v0 config (WARN+) installs
// LevelFilter{Inner: ring, Min: logs.LevelWarn} so DEBUG/INFO traffic
// doesn't fill the bounded buffer.
//
// Implemented as a wrapping Sink rather than a property on Ring so
// the filter rule can evolve (per-component, per-type) without
// touching the ring's bounded-buffer semantics.
//
// The Sink interface is embedded (via the `Inner` field, with the
// method promotion rule taking care of Drain and Stats) so this
// wrapper only has to override Enqueue. Adding a method to Sink in
// the future doesn't require touching LevelFilter.
type LevelFilter struct {
	// Inner is named because callers construct LevelFilter{Inner: ring, Min: ...}
	// as a literal — the field name is part of the public API.
	Inner Sink
	Min   logs.Level
}

// Enqueue forwards only events whose Level is at or above the
// configured minimum. Filtering on the way IN means dropped events
// don't even count against the ring's capacity — they're invisible
// to Stats.
func (f *LevelFilter) Enqueue(events []*logs.Event) {
	if len(events) == 0 {
		return
	}
	keep := events[:0:0]
	for _, e := range events {
		if levelAtLeast(e.Level, f.Min) {
			keep = append(keep, e)
		}
	}
	if len(keep) > 0 {
		f.Inner.Enqueue(keep)
	}
}

// Drain and Stats are not defined here — they delegate to Inner via
// the Inner field's Sink methods, which the LevelFilter exposes
// through its own Drain/Stats methods below. Promotion via embedding
// would be cleaner, but the named-field form is required so callers
// can write LevelFilter{Inner: ring, Min: ...} as a literal. Two
// one-line delegating methods is the minimum tax for that API shape.
func (f *LevelFilter) Drain(max int) []*logs.Event { return f.Inner.Drain(max) }
func (f *LevelFilter) Stats() SinkStats            { return f.Inner.Stats() }

// levelAtLeast reports whether `have` is at least as severe as `min`.
// Implemented as an ordered map rather than relying on the string
// values' lexicographic order (which would put "WARN" < "ERROR" but
// also "DEBUG" < "ERROR" in surprising ways).
var levelOrder = map[logs.Level]int{
	logs.LevelDebug: 0,
	logs.LevelInfo:  1,
	logs.LevelWarn:  2,
	logs.LevelError: 3,
}

func levelAtLeast(have, min logs.Level) bool {
	return levelOrder[have] >= levelOrder[min]
}

// nullSink is the no-op Sink used when telemetry is disabled. Lets
// the storage wrapper unconditionally call Enqueue without checking
// a flag on every event.
type nullSink struct{}

func (nullSink) Enqueue([]*logs.Event)   {}
func (nullSink) Drain(int) []*logs.Event { return nil }
func (nullSink) Stats() SinkStats        { return SinkStats{} }

// NewNullSink returns a Sink that silently discards everything. Used
// when telemetry is configured OFF — keeps the wrapper code branchless.
func NewNullSink() Sink { return nullSink{} }

// ensure compile-time interface conformance.
var (
	_ Sink = (*LevelFilter)(nil)
	_ Sink = nullSink{}
	_ Sink = (*Ring)(nil) // forward reference to Ring in ring.go
)

// sinkLocker is a small helper for Sinks that need a mutex around
// their internal buffer. Embedded by Ring (and would-be embedded by
// any future disk-backed sink).
type sinkLocker struct{ sync.Mutex }
