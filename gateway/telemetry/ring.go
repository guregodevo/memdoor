package telemetry

import (
	"sync/atomic"

	"memdoor/gateway/logs"
)

// Ring is the bounded in-memory event buffer the storage wrapper
// enqueues into. When the buffer is full, new events overwrite the
// OLDEST pending events — the rationale is that recent failures are
// more useful for incident response than ancient ones, and the
// alternative (drop-newest) would mean the symptom that just
// occurred is the one we lose.
//
// Capacity is fixed at construction. Operators tune it via
// the config at gateway boot. The 200-event
// default sizes for ~24 hours of WARN+ERROR on a typical user
// (single-digit anomalies/day) with margin for incident bursts.
//
// All operations are O(1); the underlying storage is a circular
// slice. Thread-safe via the embedded sinkLocker.
type Ring struct {
	sinkLocker
	buf     []*logs.Event
	head    int   // next write position
	count   int   // current pending count
	dropped int64 // atomic monotonic counter — see Stats
}

// NewRing returns a Ring sized to capacity. Capacity must be > 0;
// callers should validate before calling (the constructor panics
// on zero/negative to surface programmer errors loudly rather than
// silently producing a useless 0-cap ring).
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		panic("telemetry.NewRing: capacity must be > 0")
	}
	return &Ring{buf: make([]*logs.Event, capacity)}
}

// Enqueue appends events to the ring. When the ring is full, the
// oldest pending events are overwritten and the dropped counter
// increments. Never blocks the caller — telemetry must never become
// a hot-path bottleneck on the logger.
func (r *Ring) Enqueue(events []*logs.Event) {
	if len(events) == 0 {
		return
	}
	r.Lock()
	defer r.Unlock()
	for _, e := range events {
		if r.count == len(r.buf) {
			// Overwriting the oldest. The slot at r.head IS the
			// oldest (write-head wraps; count stayed at capacity).
			atomic.AddInt64(&r.dropped, 1)
		} else {
			r.count++
		}
		r.buf[r.head] = e
		r.head = (r.head + 1) % len(r.buf)
	}
}

// Drain removes up to max events from the ring and returns them in
// oldest-first order. Returns an empty slice when the ring is empty.
// Atomic — the caller holds the events exclusively after return,
// the ring no longer references them.
func (r *Ring) Drain(max int) []*logs.Event {
	r.Lock()
	defer r.Unlock()
	if r.count == 0 || max <= 0 {
		return nil
	}
	take := r.count
	if take > max {
		take = max
	}
	out := make([]*logs.Event, 0, take)
	// Oldest is at (head - count + capacity) mod capacity. Walk forward.
	start := (r.head - r.count + len(r.buf)) % len(r.buf)
	for i := 0; i < take; i++ {
		idx := (start + i) % len(r.buf)
		out = append(out, r.buf[idx])
		r.buf[idx] = nil // release reference so GC can reclaim
	}
	r.count -= take
	return out
}

// Peek returns up to max recent events WITHOUT removing them from
// the ring: what is about to be sent before it leaves the machine.
// Tests read it; a `tail` command is planned, not built.
func (r *Ring) Peek(max int) []*logs.Event {
	r.Lock()
	defer r.Unlock()
	if r.count == 0 || max <= 0 {
		return nil
	}
	take := r.count
	if take > max {
		take = max
	}
	out := make([]*logs.Event, 0, take)
	start := (r.head - r.count + len(r.buf)) % len(r.buf)
	for i := 0; i < take; i++ {
		idx := (start + i) % len(r.buf)
		out = append(out, r.buf[idx])
	}
	return out
}

// Stats reports the ring's current state. Pending and Dropped are
// snapshots; a concurrent Enqueue may change them immediately after
// the call returns.
func (r *Ring) Stats() SinkStats {
	r.Lock()
	pending := r.count
	cap := len(r.buf)
	r.Unlock()
	return SinkStats{
		Pending:  pending,
		Dropped:  atomic.LoadInt64(&r.dropped),
		Capacity: cap,
	}
}
