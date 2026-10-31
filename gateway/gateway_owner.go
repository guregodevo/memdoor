package gateway

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/google/uuid"

	"memdoor/gateway/queue"
	"memdoor/pkg/authorization"
	"memdoor/pkg/repository"
	"memdoor/pkg/sandbox"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
)

// Who the gateway is. Every sign-up is admin of a workspace of its own
// (workspace_provision.go), so on a gateway that strangers reach — memdoor.ai,
// or any gateway on the network with open sign-up — an admin role is no line.
// The line is the first account's workspace, the one setup made from the
// machine itself (2026-10-30: a stranger's probe on memdoor.ai; the routes
// below answered it, some with no token at all).
//
//	gatewayMember   signed in to the first account's workspace: what that
//	                workspace's own coder could do anyway (files, workflows,
//	                MCP servers), from any window, a laptop's included.
//	gatewayOperator an admin of it, on the gateway's machine: what changes
//	                the gateway for everyone (its settings, its providers).

// gatewayMember reports whether the request is from the first account's
// workspace, and answers the refusal itself. system:internal (a tool call
// from inside this process) is a member.
func (s *Server) gatewayMember(w http.ResponseWriter, r *http.Request) bool {
	return requireGatewayMember(w, r, s.repoFactory)
}

// requireGatewayMember is gatewayMember for any holder of the repositories
// (ChatServer's handlers among them).
func requireGatewayMember(w http.ResponseWriter, r *http.Request, rf repository.RepositoryFactory) bool {
	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return false
	}
	if actorID == "system:internal" {
		return true
	}
	owners := firstAccountWorkspace(r.Context(), rf)
	ec := shared.GetExecutionContext(r.Context())
	if owners == "" || ec == nil || ec.WorkspaceID != owners {
		http.Error(w, `{"error": "Permission denied: this is for the gateway's own workspace"}`, http.StatusForbidden)
		return false
	}
	return true
}

// gatewayOperator reports whether the request is the gateway's operator's —
// signed in, from the gateway's machine (fromThisMachine: the same line the
// first account is drawn on), and an admin of the first account's workspace —
// and answers the refusal itself. The machine alone is no line on a gateway
// strangers sign up to: a /remote link a sign-up turns on is a program this
// gateway hosts, and it calls back from loopback with that sign-up's token.
func (s *Server) gatewayOperator(w http.ResponseWriter, r *http.Request) bool {
	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return false
	}
	if !fromThisMachine(r) {
		http.Error(w, `{"error": "this is the gateway operator's, on the gateway's machine itself: run memdoor there"}`, http.StatusForbidden)
		return false
	}
	owners := firstAccountWorkspace(r.Context(), s.repoFactory)
	if owners == "" || s.authzService == nil {
		http.Error(w, `{"error": "the gateway's operator could not be told"}`, http.StatusForbidden)
		return false
	}
	isOperator, err := s.authzService.IsWorkspaceAdminFor(r.Context(), actorID, owners)
	if err != nil {
		http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
		return false
	}
	if !isOperator {
		http.Error(w, `{"error": "Permission denied: this is the gateway operator's (an admin of the first account's workspace)"}`, http.StatusForbidden)
		return false
	}
	return true
}

// sessionIsCallers reports whether a conversation's key is the caller's to
// run turns on or follow: one that names a workspace must name the caller's
// own; any other shape (a subagent's run, a bare key) is the gateway's own
// workspace's. A sign-up started turns on any conversation by its key until
// 2026-10-30.
func sessionIsCallers(ctx context.Context, rf repository.RepositoryFactory, key string) bool {
	if authorization.GetActorID(ctx) == "system:internal" {
		return true
	}
	ec := shared.GetExecutionContext(ctx)
	if ec == nil {
		return false
	}
	if ws := shared.ParseSessionID(key).WorkspaceID; ws != "" {
		return ws == ec.WorkspaceID || (ec.WorkspaceSlug != "" && ws == ec.WorkspaceSlug)
	}
	owners := firstAccountWorkspace(ctx, rf)
	return owners != "" && ec.WorkspaceID == owners
}

// THE GATEWAY'S MODEL AND MACHINE ARE ITS OWN WORKSPACE'S (2026-10-30). Sign-up
// stays open (each sign-up a workspace of its own, its people kept apart), but
// a turn runs on the gateway's keys and as the gateway's user, so only the
// first account's workspace runs one: anyone who reached a gateway on the
// network could sign up and have its coder run bash on the owner's key.

// turnWorkspace is the workspace a queued turn runs for: its sandbox
// context's, else its conversation key's; "" when it names none (a spawned
// run, a scheduled check — started only from the gateway's own workspace).
func turnWorkspace(job *queue.AgentJob) string {
	if job.Context != nil {
		if sc, ok := job.Context.Value(sharedctx.SandboxContextKey).(sandbox.SandboxContext); ok {
			if sc.WorkspaceID != "" {
				return sc.WorkspaceID
			}
			if sc.WorkspaceSlug != "" {
				return sc.WorkspaceSlug
			}
		}
	}
	return shared.ParseSessionID(job.SessionKey).WorkspaceID
}

// isOwnWorkspace reports whether ws, an id or a slug, is the first account's.
// Before any account there is no one else's workspace.
func (s *Server) isOwnWorkspace(ctx context.Context, ws string) bool {
	owner := firstAccountWorkspace(ctx, s.repoFactory)
	if owner == "" || ws == owner {
		return true
	}
	if db, ok := s.repoFactory.DB().(*sql.DB); ok && db != nil {
		if slug := (&dbSlugResolver{db: db}).ResolveSlug(owner); slug != "" && ws == slug {
			return true
		}
	}
	return false
}

// isOwnChannel reports whether a channel belongs to the gateway's own
// workspace (a channel that cannot be found is not).
func (s *Server) isOwnChannel(ctx context.Context, channelID string) bool {
	id, err := uuid.Parse(channelID)
	if err != nil || s.repoFactory == nil {
		return false
	}
	ch, err := s.repoFactory.Channels().GetByID(ctx, id)
	return err == nil && ch != nil && s.isOwnWorkspace(ctx, ch.WorkspaceID)
}

// conversationAllowed is sessionIsCallers for a request: signed in, and the
// conversation the key names is the caller's. It answers the refusal itself.
func (s *Server) conversationAllowed(w http.ResponseWriter, r *http.Request, key string) bool {
	if authorization.GetActorID(r.Context()) == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return false
	}
	if !sessionIsCallers(r.Context(), s.repoFactory, key) {
		http.Error(w, `{"error": "Permission denied: that conversation is another workspace's"}`, http.StatusForbidden)
		return false
	}
	return true
}

// firstAccountWorkspace is the workspace of the account setup made from the
// machine itself: the gateway's own. Empty when there is none.
func firstAccountWorkspace(ctx context.Context, rf repository.RepositoryFactory) string {
	if rf == nil {
		return ""
	}
	db, ok := rf.DB().(*sql.DB)
	if !ok || db == nil {
		return ""
	}
	var ws string
	_ = db.QueryRowContext(ctx, `SELECT workspace_id FROM users ORDER BY created_at, rowid LIMIT 1`).Scan(&ws)
	return ws
}
