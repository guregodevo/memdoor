package telemetry

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// GatewayVersion is the running memdoor version string the flusher
// stamps into every batch envelope. Populated by the CLI entry point
// (cmd/cli/cmd) before the gateway boots — keeping the version's
// canonical home in cmd/cli/cmd (where -ldflags injects it) and the
// gateway side just reading. Empty string is fine; batches without
// a version are still useful on the monitoring side, just less
// filterable.
var GatewayVersion string

// Stack is the bundle of moving parts the gateway boot path needs
// to keep alive for telemetry: the storage wrapper to install, the
// flusher whose lifecycle matches the gateway, and the sink a status
// view would read (planned, not built: docs/roadmap/SHOULD.md). Returned as a
// single value so the boot code can wire it with one call site.
type Stack struct {
	// Storage is the wrapped logs.Storage the gateway should use in
	// place of the bare inner storage. Equal to the input inner
	// when Enabled is false (zero-cost passthrough).
	Storage logs.Storage

	// Flusher runs the periodic POST loop. nil when telemetry is
	// disabled — the gateway shutdown path tolerates nil.
	Flusher Flusher

	// Sink is the buffer the storage wrapper enqueues into. nil when
	// disabled. No command reads it yet (a status/tail view is planned).
	Sink Sink

	// Enabled is true exactly when the telemetry path is active.
	// Operator-facing surfaces check this to decide whether to
	// print "ON" or "OFF".
	Enabled bool
}

// Install wires the telemetry stack on top of an existing
// logs.Storage. Reads the operator config from env (LoadConfig)
// and either returns a wrapped + flushing stack (Enabled=true) or
// a passthrough stack (Enabled=false) — the gateway can use
// Stack.Storage unconditionally.
//
// version is the running memdoor version string (the `memdoor
// version` value); included in every batch envelope for monitoring-
// side filtering. Passed in rather than read here so the
// linker-injected version lives in one place (cmd/cli/cmd.Version).
//
// logger is used for transient operational logs from the flusher.
// nil installs slog.Default.
func Install(inner logs.Storage, version string, logger *slog.Logger) *Stack {
	cfg := LoadConfig()
	if !cfg.Enabled {
		return &Stack{Storage: inner, Enabled: false}
	}
	ring := NewRing(cfg.RingCapacity)
	var sink Sink = &LevelFilter{Inner: ring, Min: cfg.MinLevel}
	wrapped := WrapStorage(inner, sink)
	client := NewHTTPClient(cfg.Endpoint, cfg.BearerToken, nil)
	flusher := NewFlusher(sink, client, FlusherConfig{
		HostID:  loadOrCreateHostID(),
		Version: version,
	}, logger)
	flusher.Start(context.Background())
	return &Stack{
		Storage: wrapped,
		Flusher: flusher,
		Sink:    sink,
		Enabled: true,
	}
}

// loadOrCreateHostID returns an install-stable anonymous UUID. First
// call creates and persists; subsequent calls read. The ID is
// per-install (not per-user, not per-host) — sufficient to group
// events by source in the monitoring inbox without leaking
// identifying info.
//
// Stored at ~/.memdoor/install-id alongside config.json. A missing
// or unreadable file is treated as "first run" — a fresh UUID is
// generated and an attempt is made to persist; failure is non-fatal
// (an ephemeral UUID is used and the next run gets a new one).
func loadOrCreateHostID() string {
	path := shared.MemdoorHome("install-id")
	if data, err := os.ReadFile(path); err == nil {
		// TrimSpace strips trailing newline / whitespace before
		// length check. Pre-fix this was len(id) >= 32 followed by
		// id[:36], which panics on any 32-35 char persisted ID
		// (e.g. a 32-char UUID variant with trailing \n). Standard
		// UUIDs are exactly 36 chars; we require that length and
		// accept anything else as "corrupted, regenerate."
		id := strings.TrimSpace(string(data))
		if len(id) == 36 {
			return id
		}
	}
	id := uuid.New().String()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(id), 0o644)
	return id
}
