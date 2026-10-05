package gateway

import (
	"encoding/json"
	"memdoor/gateway/providers"
	"memdoor/pkg/authorization"
	"net/http"
)

// handleLLMBurst serves GET /api/llm/burst: the brain's status and which model
// answers — the read behind the footer, the turn gate and `account status`.
// (The path keeps its old name; the POST that switched engines is gone,
// 2026-10-03.)
func (s *Server) handleLLMBurst(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if authorization.GetActorID(r.Context()) == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "GET only"}`, http.StatusMethodNotAllowed)
		return
	}
	out := map[string]interface{}{"brain": s.BrainStatus()}
	// decisions says whether a decision model judges this gateway's turns;
	// off, the window hints how to connect one (ui/suggest.go).
	avail, ok := s.decisions.(interface{ Available(agentID string) bool })
	out["decisions"] = ok && avail.Available("coder")
	if re := providers.ActiveRemoteEngine(); re != nil {
		out["model"], out["agent_models"], out["agent_ladders"] = re.Model, re.AgentModels, re.AgentLadders
	} else if m := providers.ConnectedModel(r.Context()); m != "" {
		// No engine chosen at start: the connected provider's model answers.
		out["model"] = m
	}
	_ = json.NewEncoder(w).Encode(out)
}
