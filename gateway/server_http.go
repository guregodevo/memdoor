package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"memdoor/gateway/health"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
)

// HTTP endpoint handlers
// Pattern: OpenClaw REST API endpoints for external integrations

// handleHealth handles health check requests
// Pattern: OpenClaw health snapshot with caching
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check for probe parameter
	probe := r.URL.Query().Get("probe") == "true"

	// Create snapshot options
	opts := health.DefaultSnapshotOptions()
	opts.Probe = probe

	// Try to get cached snapshot if not probing
	var snapshot *health.HealthSummary
	if !probe && opts.IncludeCache {
		cached := s.healthCache.Get(time.Duration(opts.CacheMaxAge) * time.Millisecond)
		if cached != nil {
			snapshot = cached
		}
	}

	// Generate fresh snapshot if needed
	if snapshot == nil {
		var err error
		snapshot, err = health.GenerateSnapshot(s.cfg, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate health snapshot: %v", err), http.StatusInternalServerError)
			return
		}

		// Cache the fresh snapshot
		s.healthCache.Set(snapshot)
	}

	// Whether a model can answer here (modelConnected). Set outside the cache.
	available := modelConnected()
	hint := ""
	if !available {
		hint = "no model configured: set a provider key or run memdoor connect"
	}
	snapshot.LLM = &health.LLMHealth{Available: available, Hint: hint}

	// Return snapshot
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode health snapshot: %v", err), http.StatusInternalServerError)
	}
}

// handleSessions handles session listing requests
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sessions := s.sessions.ListSessions()

	// Build summary list with metadata
	type sessionSummary struct {
		SessionKey   string `json:"session_key"`
		Kind         string `json:"kind"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
		MessageCount int    `json:"message_count"`
	}

	summaries := make([]sessionSummary, 0, len(sessions))
	for _, sess := range sessions {
		sess.mu.RLock()
		summaries = append(summaries, sessionSummary{
			SessionKey:   sess.ID,
			Kind:         sess.Type,
			CreatedAt:    sess.CreatedAt.Unix(),
			UpdatedAt:    sess.UpdatedAt.Unix(),
			MessageCount: len(sess.Messages),
		})
		sess.mu.RUnlock()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":    len(summaries),
		"sessions": summaries,
	})
}

// handleSessionDetail handles GET /sessions/detail?id=xxx
func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		http.Error(w, `{"error":"id parameter required"}`, http.StatusBadRequest)
		return
	}

	session, err := s.sessions.GetSession(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"session not found: %s"}`, sessionID), http.StatusNotFound)
		return
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_key":   session.ID,
		"kind":          session.Type,
		"created_at":    session.CreatedAt.Unix(),
		"updated_at":    session.UpdatedAt.Unix(),
		"message_count": len(session.Messages),
	})
}

// handleSessionHistory handles GET /sessions/history?id=xxx&limit=N&include_tools=true
func (s *Server) handleSessionHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		http.Error(w, `{"error":"id parameter required"}`, http.StatusBadRequest)
		return
	}

	session, err := s.sessions.GetSession(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"session not found: %s"}`, sessionID), http.StatusNotFound)
		return
	}

	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	includeTools := r.URL.Query().Get("include_tools") == "true"

	session.mu.RLock()
	messages := make([]map[string]interface{}, 0)
	for _, msg := range session.Messages {
		if !includeTools && (msg.Role == "tool" || msg.Role == "toolResult") {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"role":      msg.Role,
			"content":   msg.Content,
			"timestamp": msg.Timestamp.Unix(),
		})
	}
	session.mu.RUnlock()

	// Apply limit (take last N)
	truncated := false
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
		truncated = true
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_key": sessionID,
		"messages":    messages,
		"count":       len(messages),
		"truncated":   truncated,
	})
}

// handleSessionClear clears conversation history for a session
// Use this when an agent's context has grown too large (e.g., researcher with 478K tokens)
func (s *Server) handleSessionClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	// Two layers to wipe:
	//   1) In-memory Session.Messages (cleared if the session is loaded).
	//   2) On-disk persistence file (LoadRecentMessages re-reads this
	//      on the next turn, so without wiping it the agent keeps
	//      seeing the same poisoned conversation).
	//
	// "Session not found" in memory is NOT an error — chat sessions
	// are loaded on demand and may not be in the SessionManager's
	// LRU cache when a CLI test calls clear before the next turn.
	// What matters is that the persisted file is gone.
	memoryCleared := false
	if session, err := s.sessions.GetSession(req.SessionID); err == nil {
		session.Clear()
		memoryCleared = true
	}
	persistErr := s.agent.DeletePersistedSession(req.SessionID)

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"status":         "success",
		"message":        "Session conversation cleared",
		"session_id":     req.SessionID,
		"memory_cleared": memoryCleared,
	}
	if persistErr != nil {
		resp["persist_error"] = persistErr.Error()
	}
	json.NewEncoder(w).Encode(resp)
}

// handleQueue handles queue status requests
// Pattern: OpenClaw queue monitoring
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Get queue depth
	depth := s.queueManager.GetQueueDepth()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Queue status retrieved successfully",
		"depth":   depth,
	})
}

// handleAPIMessage handles REST API message sending
func (s *Server) handleAPIMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// MVP: Not yet implemented - WebSocket is the primary interface
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "received",
		"message": "Message processing not yet implemented",
	})
}

// handleAgentExecution handles agent execution requests
// Pattern: OpenClaw's gateway.agent() method
func (s *Server) handleAgentExecution(w http.ResponseWriter, r *http.Request) {
	log := logs.New("HTTP")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message          string `json:"message"`
		SessionKey       string `json:"session_key"`
		IdempotencyKey   string `json:"idempotency_key,omitempty"`
		AnnounceBack     bool   `json:"announce_back,omitempty"`
		RequesterSession string `json:"requester_session,omitempty"`
		Lane             string `json:"lane,omitempty"`
		SpawnedBy        string `json:"spawned_by,omitempty"`
		Label            string `json:"label,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	if req.SessionKey == "" {
		http.Error(w, "session_key is required", http.StatusBadRequest)
		return
	}

	// Generate run ID
	runID := req.IdempotencyKey
	if runID == "" {
		runID = generateID("run")
	}

	// Check for duplicate (idempotency)
	if existingRun, err := s.runTracker.GetRun(runID); err == nil {
		// Run already exists
		log.Debug("Idempotent run detected",
			slog.String("run_id", runID),
			slog.String("status", string(existingRun.Status)))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"run_id":      runID,
			"status":      "accepted",
			"session_key": req.SessionKey,
		})
		return
	}

	log.Debug("Creating agent run",
		slog.String("run_id", runID),
		slog.String("session", req.SessionKey),
		slog.Bool("announce_back", req.AnnounceBack),
		slog.String("lane", req.Lane))

	// Determine global lane from request
	// Pattern: OpenClaw lane selection
	globalLane := queue.LaneMain
	if req.Lane == "cron" {
		globalLane = queue.LaneCron
	} else if req.Lane == "subagent" {
		globalLane = queue.LaneSubagent
	} else if req.Lane == "nested" {
		globalLane = queue.LaneNested
	}

	// Enqueue job for execution
	// Pattern: OpenClaw's enqueueCommandInLane()
	job := &queue.AgentJob{
		SessionKey:  req.SessionKey,
		Message:     req.Message,
		GlobalLane:  globalLane,
		EnqueueTime: time.Now(),
		Context:     context.Background(),
	}

	err := s.queueManager.EnqueueJob(job)
	if err != nil {
		log.WithError(err).Warn("Failed to enqueue job")
		http.Error(w, fmt.Sprintf("Failed to enqueue job: %v", err), http.StatusInternalServerError)
		return
	}

	// Return immediately with run ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":      runID,
		"status":      "accepted",
		"session_key": req.SessionKey,
	})
}

// handleSessionRewind handles POST /sessions/rewind {"session_id","turns"} —
// undo the last N turns of a session instead of wiping it (handleSessionClear).
// Same two layers as clear: truncate the persisted file, then clear the
// in-memory copy so the next turn reloads the truncated transcript.
func (s *Server) handleSessionRewind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Turns     int    `json:"turns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if req.Turns < 1 {
		req.Turns = 1
	}
	dropped, err := s.agent.RewindPersistedSession(req.SessionID, req.Turns)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadRequest)
		return
	}
	if session, err := s.sessions.GetSession(req.SessionID); err == nil {
		session.Clear()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success", "turns": req.Turns, "entries_dropped": dropped,
	})
}
