package billingsvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"memdoor/pkg/plan"
)

// handleSetPlan — POST /v1/plan {"workspace":"acme","plan":"enterprise"}.
// Operator only.
//
// This is the whole enterprise upgrade path, and it is deliberately a command
// an operator runs rather than a button a customer presses. Committed capacity
// carries contractual terms; a self-serve button would imply a price somebody
// can accept alone, and this one has a signature on it.
func (s *Server) handleSetPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	op := os.Getenv("MEMDOOR_BILLING_TOKEN")
	if op == "" || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != op {
		writeErr(w, http.StatusForbidden, fmt.Errorf("operator token required"))
		return
	}
	var req struct {
		Workspace string `json:"workspace"`
		Plan      string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid body"))
		return
	}
	if req.Workspace == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("workspace is required"))
		return
	}
	p, err := plan.Parse(req.Plan)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.auth.SetPlan(req.Workspace, p)
	writeJSON(w, map[string]interface{}{
		"workspace": req.Workspace,
		"plan":      p.String(),
		"label":     p.Label(),
	})
}
