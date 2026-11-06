package gateway

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
)

// A PERSON WHO NEVER HAD A PASSWORD MUST NOT BE LOCKED OUT (2026-11-06, the
// maintainer: "users dont know their password like me and cannot connect
// anymore"). The generated login (~/.memdoor/local-login) renews a lapsed
// session, but when the gateway refuses it — the file overwritten, the user
// made by hand — nothing did, and the only advice was a password nobody had.
//
// The gateway writes ~/.memdoor/machine-key (0600) at boot. A request from
// its own machine that shows the key gets a session for the gateway's
// operator. Whoever can read that file can already read the database and
// master.key beside it, so the key grants nothing they could not take.

// machineKeyFile is the key the gateway writes and the CLI on its machine reads.
const machineKeyFile = "machine-key"

// ensureMachineKey writes the key once; an existing key is kept, so the
// gateways one person runs on one machine answer the same key.
func ensureMachineKey() {
	p := shared.MemdoorHome(machineKeyFile)
	if b, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(b)) != "" {
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		logs.New("MachineKey").Warn("machine key not written", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		logs.New("MachineKey").Warn("machine key not written", "error", err)
		return
	}
	if err := os.WriteFile(p, []byte(hex.EncodeToString(raw)), 0o600); err != nil {
		logs.New("MachineKey").Warn("machine key not written", "error", err)
	}
}

func readMachineKey() string {
	b, err := os.ReadFile(shared.MemdoorHome(machineKeyFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

type machineLoginRequest struct {
	Key string `json:"key"`
	// Email is the account the CLI was signed in as: honoured when it is an
	// admin of the gateway's own workspace, else the first account is.
	Email string `json:"email"`
}

// handleMachineLogin is POST /api/auth/machine-login.
func (s *Server) handleMachineLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !fromThisMachine(r) {
		http.Error(w, `{"error": "the machine key signs in on the gateway's machine itself only"}`, http.StatusForbidden)
		return
	}
	if ip := guestClientIP(r); !loginRateLimiter.Allow(ip) {
		http.Error(w, `{"error":"too many attempts, slow down"}`, http.StatusTooManyRequests)
		return
	}
	var req machineLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}
	want := readMachineKey()
	if want == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Key)), []byte(want)) != 1 {
		http.Error(w, `{"error": "that is not this gateway's machine key"}`, http.StatusUnauthorized)
		return
	}
	userID := s.machineLoginUser(r.Context(), req.Email)
	if userID == "" || s.authAdapter == nil {
		http.Error(w, `{"error": "this gateway has no account yet"}`, http.StatusNotFound)
		return
	}
	user, token, err := s.authAdapter.service.OpenSessionFor(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error": "could not open a session"}`, http.StatusInternalServerError)
		return
	}
	s.log.Info("machine login", "email", user.Email)
	_ = json.NewEncoder(w).Encode(AuthResponse{User: *user, Token: token})
}

// machineLoginUser is the account the machine key signs in: the one named,
// when it is the operator's (an admin of the first account's workspace),
// else the first account.
func (s *Server) machineLoginUser(ctx context.Context, email string) string {
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok || db == nil {
		return ""
	}
	if email = strings.TrimSpace(email); email != "" && s.authzService != nil {
		var id string
		if db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email) = lower(?)`, email).Scan(&id) == nil {
			if owners := firstAccountWorkspace(ctx, s.repoFactory); owners != "" {
				if ok, err := s.authzService.IsWorkspaceAdminFor(ctx, shared.ActorID(id), owners); err == nil && ok {
					return id
				}
			}
		}
	}
	var first string
	_ = db.QueryRowContext(ctx, `SELECT id FROM users ORDER BY created_at, rowid LIMIT 1`).Scan(&first)
	return first
}
