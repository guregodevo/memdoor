package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
)

// handleSessionCompact is POST /api/sessions/compact {workspace, channel_id,
// agent, focus}: /compact in the TUI (AgentRuntime.CompactNow). A turn that
// is running owns the conversation, so it is refused, not cancelled.
func (s *Server) handleSessionCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "POST"}`, http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Workspace string `json:"workspace"`
		ChannelID string `json:"channel_id"`
		Agent     string `json:"agent"`
		Focus     string `json:"focus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error": "bad json"}`, http.StatusBadRequest)
		return
	}
	in.Agent = strings.ToLower(strings.TrimSpace(in.Agent))
	scope := strings.TrimSpace(in.Workspace)
	if scope == "" || in.ChannelID == "" || in.Agent == "" {
		http.Error(w, `{"error": "workspace, channel_id and agent are required"}`, http.StatusBadRequest)
		return
	}
	key := shared.NewChannelSessionID(scope, in.ChannelID)
	w.Header().Set("Content-Type", "application/json")
	if s.runActive(key) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "a turn is running — /compact when it ends, or Esc first"})
		return
	}
	res, err := s.agent.CompactNow(r.Context(), transcriptKey(key, in.Agent), in.Agent, in.Focus)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	logs.New("Session").Info(fmt.Sprintf("compacted on request: %d → %d tokens in %d → %d messages (summary written by the model: %v)",
		res.BeforeTokens, res.AfterTokens, res.BeforeMessages, res.AfterMessages, res.Written),
		slog.String("session", key), slog.String("agent", in.Agent), slog.Bool("focus", strings.TrimSpace(in.Focus) != ""))
	_ = json.NewEncoder(w).Encode(res)
}

// handleSessionHandoff is POST /api/sessions/handoff {workspace, channel_id,
// agent}: /handoff in the TUI. The model writes a handoff of the
// conversation; the conversation is then wiped, as /fresh wipes it, and the
// next one starts from the handoff. Refused while a turn runs.
func (s *Server) handleSessionHandoff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "POST"}`, http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Workspace string `json:"workspace"`
		ChannelID string `json:"channel_id"`
		Agent     string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error": "bad json"}`, http.StatusBadRequest)
		return
	}
	in.Agent = strings.ToLower(strings.TrimSpace(in.Agent))
	scope := strings.TrimSpace(in.Workspace)
	if scope == "" || in.ChannelID == "" || in.Agent == "" {
		http.Error(w, `{"error": "workspace, channel_id and agent are required"}`, http.StatusBadRequest)
		return
	}
	key := shared.NewChannelSessionID(scope, in.ChannelID)
	transcript := transcriptKey(key, in.Agent)
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, err error) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
	}
	if s.runActive(key) {
		fail(http.StatusConflict, fmt.Errorf("a turn is running — /handoff when it ends, or Esc first"))
		return
	}
	handoff, err := s.agent.Handoff(r.Context(), transcript, in.Agent)
	if err != nil {
		fail(http.StatusBadGateway, fmt.Errorf("the handoff could not be written: %w", err))
		return
	}
	// The wipe /fresh does, then the handoff as the new conversation's start.
	if _, err := startFresh(s.freshSteps(key), key, transcript); err != nil {
		fail(http.StatusInternalServerError, err)
		return
	}
	if err := s.agent.StartFrom(transcript, handoffHead+handoff); err != nil {
		fail(http.StatusInternalServerError, err)
		return
	}
	logs.New("Session").Info("handoff: the conversation was summarized and started over from it",
		slog.String("session", key), slog.String("agent", in.Agent), slog.Int("bytes", len(handoff)))
	_ = json.NewEncoder(w).Encode(map[string]any{"handoff": handoff})
}
