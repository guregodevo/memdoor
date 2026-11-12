package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"memdoor/gateway/logs"
)

// The event log is the whole gateway's (every workspace's tool inputs,
// emails, session ids), so every route here is an admin of the gateway's own
// workspace's: any sign-up, admin of its own, read and pruned it until
// 2026-10-30.

// handleLogsQuery handles GET /api/logs/query
func (s *Server) handleLogsQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}

	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}

	s.log.Debug("Logs query request", slog.String("query", r.URL.RawQuery))

	query, err := logs.QueryFromParams(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := storage.QueryEvents(r.Context(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleLogsPrune handles POST /api/logs/prune — delete old log events
// and VACUUM. Cutoff via query param: ?before=<dur> (e.g. 720h) |
// ?days=<n> | ?all=true.
func (s *Server) handleLogsPrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Pruning/VACUUMing the audit log is destructive — admin only.
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}
	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	var cutoff time.Time
	switch {
	case q.Get("all") == "true":
		cutoff = time.Now()
	case q.Get("before") != "":
		dur, err := time.ParseDuration(q.Get("before"))
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid before: %v", err), http.StatusBadRequest)
			return
		}
		cutoff = time.Now().Add(-dur)
	case q.Get("days") != "":
		days, err := strconv.Atoi(q.Get("days"))
		if err != nil || days < 0 {
			http.Error(w, "invalid days", http.StatusBadRequest)
			return
		}
		cutoff = time.Now().AddDate(0, 0, -days)
	default:
		http.Error(w, "specify before=<dur>, days=<n>, or all=true", http.StatusBadRequest)
		return
	}
	deleted, err := storage.PruneBefore(r.Context(), cutoff)
	if err != nil {
		http.Error(w, fmt.Sprintf("prune failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deleted": deleted,
		"cutoff":  cutoff.Format(time.RFC3339),
	})
}

// handleLogsErrors handles GET /api/logs/errors
func (s *Server) handleLogsErrors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}

	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}

	s.log.Debug("Logs errors request")

	query, err := logs.ErrorsQueryFromParams(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := storage.QueryEvents(r.Context(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleLogsStats handles GET /api/logs/stats
func (s *Server) handleLogsStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}

	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}

	s.log.Debug("Logs stats request")

	stats, err := storage.GetStats(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("stats failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleLogsTrace handles GET /api/logs/trace?event_id=xxx
func (s *Server) handleLogsTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}

	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}

	eventID := r.URL.Query().Get("event_id")
	if eventID == "" {
		http.Error(w, "event_id is required", http.StatusBadRequest)
		return
	}

	s.log.Debug("Logs trace request", slog.String("event_id", eventID))

	chain, err := storage.TraceChain(r.Context(), eventID)
	if err != nil {
		http.Error(w, fmt.Sprintf("trace failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chain)
}

// handleLogsSession handles GET /api/logs/session?session_id=xxx
func (s *Server) handleLogsSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdmin(w, r, s.authzService) || !s.gatewayMember(w, r) {
		return
	}

	storage := logs.GetGlobalStorage()
	if storage == nil {
		http.Error(w, "log storage not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	s.log.Debug("Logs session request", slog.String("session_id", sessionID))

	events, err := storage.ReconstructSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("session reconstruction failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id": sessionID,
		"events":     events,
		"count":      len(events),
	})
}
