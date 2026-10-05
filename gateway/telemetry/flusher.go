package telemetry

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Flusher periodically drains a Sink and ships batches via a Client.
// Runs as a background goroutine started by Start(); stops cleanly
// via Stop() so gateway shutdown can wait for the last batch in
// flight. Interface so the gateway boot path can take it as a
// dependency and Stop() it from the shutdown hook.
type Flusher interface {
	// Start begins the periodic flush loop. Safe to call once;
	// double-Start is a programmer error and panics.
	Start(ctx context.Context)

	// Stop signals the flush loop to exit and waits for it to
	// drain any in-flight batch. Safe to call after Start; safe
	// to call before Start (no-op).
	Stop()
}

// FlusherConfig tunes the periodic flush loop. Zero values mean
// "use the default" — picked for the 10s typical-WARN-rate workload
// without bursting the monitoring endpoint.
type FlusherConfig struct {
	// Interval is the period between flush attempts. Default 30s.
	// Smaller = lower latency from event to monitoring inbox; larger
	// = fewer HTTP requests under steady-state.
	Interval time.Duration

	// BatchMax caps the number of events shipped per HTTP request.
	// Default 50. Bigger = fewer requests under bursts; smaller =
	// less work to discard if a single request fails.
	BatchMax int

	// HostID identifies this install in the batch envelope. Required
	// for the server to group events by source.
	HostID string

	// Version is the running memdoor version string. Useful on the
	// monitoring side for "this bug only happens on v1.10.0+"
	// queries.
	Version string
}

// NewFlusher returns a Flusher that drains sink and ships via client
// on the configured interval. Logger is used for transient operational
// warnings (send failures, etc.) — passing nil installs slog.Default.
func NewFlusher(sink Sink, client Client, cfg FlusherConfig, logger *slog.Logger) Flusher {
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.BatchMax == 0 {
		cfg.BatchMax = 50
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &flusher{
		sink:    sink,
		client:  client,
		cfg:     cfg,
		logger:  logger,
		stopped: make(chan struct{}),
	}
}

type flusher struct {
	sink   Sink
	client Client
	cfg    FlusherConfig
	logger *slog.Logger

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	stopped chan struct{}
}

func (f *flusher) Start(ctx context.Context) {
	f.mu.Lock()
	if f.started {
		f.mu.Unlock()
		panic("telemetry.Flusher: Start called twice")
	}
	f.started = true
	loopCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.mu.Unlock()

	go f.loop(loopCtx)
}

func (f *flusher) Stop() {
	f.mu.Lock()
	if !f.started {
		f.mu.Unlock()
		return
	}
	cancel := f.cancel
	stopped := f.stopped
	f.mu.Unlock()
	cancel()
	<-stopped
}

func (f *flusher) loop(ctx context.Context) {
	defer close(f.stopped)
	ticker := time.NewTicker(f.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// One last drain so events buffered just before shutdown
			// still make it to the server.
			f.flushOnce(context.Background())
			return
		case <-ticker.C:
			f.flushOnce(ctx)
		}
	}
}

// flushOnce drains a single batch and posts it. On send failure the
// drained events are NOT re-enqueued — by the time we know the send
// failed, the ring may have moved on. v0 trades that loss for
// simpler code; if it shows up as a real gap in the monitoring
// inbox, a "requeue on failure" pass is straightforward.
func (f *flusher) flushOnce(ctx context.Context) {
	events := f.sink.Drain(f.cfg.BatchMax)
	if len(events) == 0 {
		return
	}
	batch := Batch{
		HostID:  f.cfg.HostID,
		Version: f.cfg.Version,
		Events:  events,
	}
	if err := f.client.Send(ctx, batch); err != nil {
		f.logger.Warn("telemetry send failed",
			"err", err,
			"events_dropped", len(events))
	}
}
