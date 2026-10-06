package gateway

import (
	"encoding/json"
	"net/http"

	"memdoor/gateway/telemetry"
)

// handleStatus answers /api/status: the gateway is up, and which version. It
// is what `make up`, the installer's restart check, the CI installers
// workflow and the TUI skill poll for readiness. Until 2026-10-06 the path had
// no handler and the catch-all served the home page with 200, so the polls
// passed by accident; the 404 for unknown paths made them honest, and this
// makes them true.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": telemetry.GatewayVersion})
}
