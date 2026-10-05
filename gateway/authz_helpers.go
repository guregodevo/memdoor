package gateway

import (
	"net/http"

	"memdoor/pkg/authorization"
)

// Centralized HTTP authorization gates. These exist because authz was
// previously copy-pasted per handler — and several handlers simply forgot
// the check (system secrets, agent update/delete, cron, memory, log prune,
// the X-Internal-Service email/invite endpoints). Routing every handler
// through one of these helpers makes "deny by default" the easy path and
// keeps the policy in one place.
//
// A nil authzService disables the admin check (the test harness wires it
// nil), matching the long-standing `if authz != nil` convention.

// requireAdmin allows the request only if the actor is a workspace admin.
// On denial it writes the response and returns false, so callers do:
//
//	if !requireAdmin(w, r, cs.authzService) { return }
//
// The internal actor ("system:internal", set by AuthMiddleware only for
// loopback X-Internal-Service calls) is treated as admin by
// IsWorkspaceAdmin, so in-process agent tools keep working.
func requireAdmin(w http.ResponseWriter, r *http.Request, authz *authorization.Service) bool {
	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return false
	}
	if authz != nil {
		isAdmin, err := authz.IsWorkspaceAdmin(r.Context(), actorID)
		if err != nil || !isAdmin {
			http.Error(w, `{"error":"permission denied: admin only"}`, http.StatusForbidden)
			return false
		}
	}
	return true
}

// (requireAuth — "any authenticated actor" — already exists in
// chat_server.go and returns (actorID, ok); use that for auth-only gates.)

// isInternalActor reports whether the request is an in-process call from an
// agent tool. AuthMiddleware sets this actor ONLY for an X-Internal-Service
// request originating from a loopback address, so — unlike reading the raw
// header — a remote attacker cannot forge it.
func isInternalActor(r *http.Request) bool {
	return authorization.GetActorID(r.Context()) == "system:internal"
}
