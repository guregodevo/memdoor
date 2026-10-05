package telemetry

import (
	"context"

	"memdoor/gateway/logs"
)

// WrapStorage returns a logs.Storage decorator that forwards every
// WriteEvents call to the inner storage AND enqueues a copy of the
// events to the telemetry sink. Read paths (QueryEvents, TraceChain,
// etc.) pass through unchanged — telemetry never affects local
// observability.
//
// Wrapping rather than tee'ing at a higher layer keeps the wiring
// invisible to callers: any code path that writes events through
// gateway/logs is automatically telemetered. New emit sites need
// no opt-in; if telemetry is enabled at boot, they're covered.
//
// When telemetry is disabled, the gateway boot path skips this
// wrap entirely and uses the bare inner storage — no per-event
// branch, no per-event cost.
func WrapStorage(inner logs.Storage, sink Sink) logs.Storage {
	if sink == nil {
		sink = NewNullSink()
	}
	return &storageWrapper{Storage: inner, sink: sink}
}

// storageWrapper decorates a logs.Storage with side-effects on
// WriteEvents (sink enqueue + loop defense) and forwards every other
// method to the inner storage unchanged. We embed the logs.Storage
// interface so all read-side methods (QueryEvents, TraceChain,
// ReconstructSession, FindSimilar, GetStats, Close) come for free
// via the promotion rules — only WriteEvents needs an override.
//
// Pre-fix this struct had six pass-through methods that just called
// inner.X(...). Each new method on logs.Storage required updating
// the wrapper; each was a one-line that added zero behavior. The
// embedded interface kills the duplication: the wrapper is the
// override list, nothing more.
type storageWrapper struct {
	logs.Storage // embedded — promotes all methods of the inner storage
	sink         Sink
}

// WriteEvents is the ONE method this wrapper overrides. It writes to
// the inner storage first (the local sqlite is the source of truth
// for `memdoor logs query`), then enqueues a copy onto the telemetry
// sink. Sink enqueueing happens regardless of the inner write outcome
// — telemetry that captures a failed-to-write event is more useful
// than one that silently drops it on the floor.
//
// Loop defense: events that arrived via the server-side receive
// handler carry telemetry_host_id in Data; the receive path stamps
// it. The wrapper drops those before enqueueing so the memdoor.ai
// monitoring gateway can't accidentally telemetry-loop the same
// events back into circulation if it has telemetry enabled pointed
// at itself.
func (w *storageWrapper) WriteEvents(ctx context.Context, events []*logs.Event) error {
	err := w.Storage.WriteEvents(ctx, events)
	if local := filterLocallyOriginated(events); len(local) > 0 {
		w.sink.Enqueue(local)
	}
	return err
}

// filterLocallyOriginated returns events that were emitted on this
// host (not received from a remote install). The receive handler
// stamps Data["telemetry_host_id"] on every received event; locally-
// originated events don't have that field.
func filterLocallyOriginated(events []*logs.Event) []*logs.Event {
	out := events[:0:0]
	for _, e := range events {
		if e.Data != nil {
			if _, received := e.Data["telemetry_host_id"]; received {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}
