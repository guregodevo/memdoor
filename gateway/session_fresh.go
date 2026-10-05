package gateway

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/notes"
	"memdoor/pkg/shared"
	"memdoor/tools"
)

// FRESH SESSION, KEEP THE TASK (roadmap MUST item 6, 2026-09-29): a long
// conversation full of old reads and big listings degrades every later turn.
// POST /api/sessions/fresh wipes an agent's memory of one conversation — what
// the model is sent, not the messages people read — so the TUI's /fresh can
// send the same request again to an agent that starts from nothing, and /clear
// can wipe it without a task.
//
// ORDER MATTERS. A turn saves its new messages when it ENDS, cancelled or not
// (updateSession), and deregisters after it has saved. Wiping while a turn is
// still running — right after Esc — would be undone a moment later by that
// turn's save. So: cancel the running turn, wait until it has deregistered,
// and only then delete.

// freshWait bounds how long a cancelled turn may take to wind down.
const freshWait = 15 * time.Second

var errFreshTurnStillRunning = errors.New("the running turn did not stop in time; try again")

// freshSteps are what a fresh start does, injected so the order can be tested
// against a turn that saves late.
type freshSteps struct {
	cancel  func(key string) bool // cancel the conversation's running turn
	running func(key string) bool // is a turn still registered for it
	forget  func(key string)      // drop the in-memory session
	remove  func(key string) error
	wait    time.Duration
}

// startFresh cancels, waits for the turn to be gone, then forgets and deletes
// the agent's transcript. It reports whether a turn was cancelled.
func startFresh(steps freshSteps, sessionKey, transcript string) (bool, error) {
	cancelled := steps.cancel(sessionKey)
	deadline := time.Now().Add(steps.wait)
	for steps.running(sessionKey) {
		if time.Now().After(deadline) {
			return cancelled, errFreshTurnStillRunning
		}
		time.Sleep(25 * time.Millisecond)
	}
	steps.forget(sessionKey)
	return cancelled, steps.remove(transcript)
}

// runActive reports whether a turn is registered for sessionKey.
func (s *Server) runActive(sessionKey string) bool {
	s.runCancelsMu.Lock()
	defer s.runCancelsMu.Unlock()
	_, ok := s.runCancels[sessionKey]
	return ok
}

// handleSessionFresh is POST /api/sessions/fresh {workspace, channel_id, agent}.
func (s *Server) handleSessionFresh(w http.ResponseWriter, r *http.Request) {
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
	cancelled, err := startFresh(s.freshSteps(key), key, transcriptKey(key, in.Agent))
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errFreshTurnStillRunning) {
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	logs.New("Session").Info("fresh session: the agent's memory of this conversation was wiped",
		slog.String("session", key), slog.String("agent", in.Agent), slog.Bool("cancelled_a_turn", cancelled))
	_ = json.NewEncoder(w).Encode(map[string]any{"cancelled": cancelled, "cleared": true})
}

// freshSteps are the steps of wiping the conversation key: /fresh, /clear,
// and /handoff after its summary.
func (s *Server) freshSteps(key string) freshSteps {
	return freshSteps{
		cancel:  s.cancelRun,
		running: s.runActive,
		forget: func(k string) {
			if sess, err := s.sessions.GetSession(k); err == nil {
				sess.Clear()
			}
		},
		remove: func(transcript string) error {
			if err := s.agent.DeletePersistedSession(transcript); err != nil {
				return err
			}
			return tools.DeleteNotes(notes.ConversationKey(key))
		},
		wait: freshWait,
	}
}
