package gateway

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/telemetry"
)

// maxTelemetryPostBytes caps the POST body size the receive handler
// will read. Picked at 10 MiB — comfortably above the largest plausible
// honest batch (50 events × ~10 KB each ≈ 500 KB, with margin for
// future schema growth) but well below the threshold where a single
// request could exhaust server memory. A malicious sender posting
// gigabyte JSON gets a clean 413 instead of OOMing the gateway.
const maxTelemetryPostBytes = 10 * 1024 * 1024

// handleTelemetry receives event batches POSTed by remote memdoor
// installs that have opted in to telemetry, and writes them to the
// LOCAL logs.Storage so the `memdoor logs query` UX works against
// the monitoring inbox the same way it works against a local install.
//
// This is what makes memdoor.ai "the monitoring layer" — the server-
// side gateway runs the same logs.Storage as every other install,
// and incoming batches just become entries in that storage. the maintainer
// (or whoever monitors) queries with `memdoor logs query --regex
// 'reference catalogue' --since 1h` against the memdoor.ai gateway and sees
// every remote install's stale-catalogue warnings.
//
// Auth: optional bearer token (MEMDOOR_TELEMETRY_RECEIVE_TOKEN env
// var on the server). Empty means accept all — fine for v0 while
// only the maintainer's installs are configured to send. Phase 2 needs a real
// per-tenant auth story.
//
// Anti-loop: this handler does NOT re-emit the received events
// through the gateway's own EventLogger — that would create an
// infinite cycle on the memdoor.ai monitoring gateway if it ever
// had telemetry enabled pointing at itself. We write straight to
// the underlying Storage via the global handle.
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// The receiver takes a bearer token, MEMDOOR_TELEMETRY_RECEIVE_TOKEN, and
	// nothing without one: what lands here is read by the operator and by
	// the coder (`logs query`), so an anonymous write is a line in their
	// input. Constant-time comparison so the token cannot be read off timing.
	expected := os.Getenv("MEMDOOR_TELEMETRY_RECEIVE_TOKEN")
	if expected == "" {
		http.Error(w, "telemetry receiver not configured", http.StatusServiceUnavailable)
		return
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Cap the POST body size before decoding. Without this, a single
	// malicious POST of arbitrary size can exhaust server memory on
	// the JSON decode. maxTelemetryPostBytes (10 MiB) is well above
	// the largest plausible honest batch.
	r.Body = http.MaxBytesReader(w, r.Body, maxTelemetryPostBytes)
	var batch telemetry.Batch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "bad batch json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(batch.Events) == 0 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":0}`))
		return
	}

	// Stamp the receive-side fields onto each event so monitoring
	// queries can group/filter by source. host_id and version come
	// from the batch envelope; we project them into Event.Data
	// so the existing `--regex` and per-field query paths work
	// without schema changes.
	for _, e := range batch.Events {
		if e.Data == nil {
			e.Data = make(map[string]any)
		}
		e.Data["telemetry_host_id"] = batch.HostID
		if batch.Version != "" {
			e.Data["telemetry_sender_version"] = batch.Version
		}
		e.Data["telemetry_received_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	// Write straight to the underlying Storage (NOT through the
	// EventLogger) — defends against the infinite-loop case where
	// a monitoring gateway has telemetry enabled pointing at itself.
	store := logs.GetGlobalStorage()
	if store == nil {
		http.Error(w, "no storage", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := store.WriteEvents(ctx, batch.Events); err != nil {
		http.Error(w, "write events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"received": len(batch.Events)})
}
