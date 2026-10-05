package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// user_handlers_admin: per-user admin: get, role update, delete, admin password reset.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// handleGetUser handles GET /api/users/:id for fetching single user information
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request, userID string) {
	// Reading a user (incl. their email) requires authentication. The
	// route is authMiddleware-wrapped, so anonymous callers fail here.
	if _, ok := requireAuth(w, r); !ok {
		return
	}

	// Get database from repository factory
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Query database for user information
	// Note: We use a simplified User struct here since this is just for API response
	// Not using pkg/auth User to avoid coupling
	type UserResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		AvatarURL   string `json:"avatar_url,omitempty"`
		AvatarEmoji string `json:"avatar_emoji,omitempty"`
		Icon        string `json:"icon,omitempty"`
	}

	var user UserResponse
	var avatarURL, avatarEmoji, icon sql.NullString

	query := `SELECT id, name, email, avatar_url, avatar_emoji, icon FROM users WHERE id = ?`
	err := db.QueryRowContext(r.Context(), query, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&avatarURL,
		&avatarEmoji,
		&icon,
	)

	if err == sql.ErrNoRows {
		s.log.Warn("User not found", slog.String("user_id", userID))
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	if err != nil {
		s.log.Error("Failed to fetch user", slog.String("user_id", userID), slog.String("error", err.Error()))
		http.Error(w, `{"error": "Failed to fetch user"}`, http.StatusInternalServerError)
		return
	}

	// Handle nullable fields
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if avatarEmoji.Valid {
		user.AvatarEmoji = avatarEmoji.String
	}
	if icon.Valid {
		user.Icon = icon.String
	}

	// Return user data
	json.NewEncoder(w).Encode(user)
}

// sameWorkspaceAsActor reports whether the target user belongs to the same
// workspace as the acting admin. This blocks a cross-tenant IDOR: an admin of
// workspace A mutating (role/password) a user of workspace B. The internal
// actor ("system:internal") is exempt — it is a global, trusted caller.
//
// On any failure (auth service missing, actor or target not found) it returns
// false + an explanatory message and writes nothing, leaving HTTP handling to
// the caller. In single-workspace installs both users share one workspace_id,
// so this always passes and behavior is unchanged.
func (s *Server) sameWorkspaceAsActor(ctx context.Context, actorID shared.ActorID, targetUserID string) (bool, string) {
	if actorID == "system:internal" {
		return true, ""
	}
	if s.authAdapter == nil {
		return false, "Auth service not available"
	}

	actor, err := s.authAdapter.service.GetUserByID(ctx, string(actorID))
	if err != nil {
		return false, "Failed to load acting user"
	}
	target, err := s.authAdapter.service.GetUserByID(ctx, targetUserID)
	if err != nil {
		return false, "User not found"
	}
	if actor.WorkspaceID != target.WorkspaceID {
		return false, "Permission denied: user is in another workspace"
	}
	return true, ""
}

// handleUpdateUserRole handles the /api/admin/users/{id}* subtree (admin only):
//   - PUT  /api/admin/users/{id}          → update user role (existing behavior)
//   - POST /api/admin/users/{id}/password → set the user's password (rotation/offboarding)
//
// Both paths land here because the route is registered as the catch-all
// "/api/admin/users/" prefix; we dispatch on the subpath + method. The admin
// check is shared by both branches.
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()

	// Get authenticated user
	actorID := authorization.GetActorID(ctx)
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}

	// AUTHORIZATION CHECK: Only admins can update user roles or set passwords
	if s.authzService != nil {
		isAdmin, err := s.authzService.IsWorkspaceAdmin(ctx, actorID)
		if err != nil {
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			http.Error(w, `{"error": "Permission denied: admin access required"}`, http.StatusForbidden)
			return
		}
	}

	// Extract path after the route prefix: /api/admin/users/{id}[/password]
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	path = strings.TrimSuffix(path, "/")

	// Subpath dispatch: the set-password lever lives at {id}/password (POST).
	if strings.HasSuffix(path, "/password") {
		userID := strings.TrimSuffix(path, "/password")
		s.handleAdminSetPassword(w, r, userID)
		return
	}

	// DELETE /api/admin/users/{id} — remove a user and all of their rows.
	if r.Method == http.MethodDelete {
		s.handleDeleteUser(w, r, path, actorID)
		return
	}

	if r.Method != http.MethodPut {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := path
	if userID == "" {
		http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, `{"error": "Invalid role: must be 'admin' or 'user'"}`, http.StatusBadRequest)
		return
	}

	// Cross-tenant guard: the target must be in the acting admin's workspace.
	if ok, msg := s.sameWorkspaceAsActor(ctx, actorID, userID); !ok {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, msg), http.StatusForbidden)
		return
	}

	// Get database from repository factory
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Update user role in database
	result, err := db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, req.Role, userID)
	if err != nil {
		http.Error(w, `{"error": "Failed to update user role"}`, http.StatusInternalServerError)
		return
	}

	// Check if user was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, `{"error": "Failed to verify update"}`, http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user_id": userID,
		"role":    req.Role,
	})
}

// handleDeleteUser handles DELETE /api/admin/users/{id} — remove a user and
// every row keyed to them (sessions, SSH keys, channel memberships, tokens) in
// a single transaction. The admin check is done by the caller. Guards: an admin
// can't delete themselves (lockout), can't delete across workspaces, and can't
// remove the last admin of a workspace (which would orphan it).
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, userID string, actorID shared.ActorID) {
	ctx := r.Context()
	if userID == "" {
		http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
		return
	}
	if userID == string(actorID) {
		http.Error(w, `{"error": "You cannot delete your own account"}`, http.StatusBadRequest)
		return
	}
	// Cross-tenant guard: target must be in the acting admin's workspace.
	if ok, msg := s.sameWorkspaceAsActor(ctx, actorID, userID); !ok {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, msg), http.StatusForbidden)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Refuse to remove the last admin of the target's workspace — that would
	// leave it with no one able to administer it.
	var wsID, role string
	if err := db.QueryRowContext(ctx, `SELECT workspace_id, role FROM users WHERE id = ?`, userID).Scan(&wsID, &role); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error": "Failed to load user"}`, http.StatusInternalServerError)
		return
	}
	if role == "admin" {
		var admins int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE workspace_id = ? AND role = 'admin'`, wsID).Scan(&admins); err == nil && admins <= 1 {
			http.Error(w, `{"error": "Cannot delete the last admin of the workspace"}`, http.StatusBadRequest)
			return
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error": "Failed to start transaction"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Order doesn't matter (no FK cascade enforced), but delete dependents first
	// for clarity. Every row keyed to the user is removed.
	stmts := []struct {
		sql string
		arg string
	}{
		{`DELETE FROM sessions WHERE user_id = ?`, userID},
		{`DELETE FROM user_ssh_keys WHERE user_id = ?`, userID},
		{`DELETE FROM channel_memberships WHERE actor_id = ?`, userID},
		{`DELETE FROM email_verification_tokens WHERE user_id = ?`, userID},
		{`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID},
		{`DELETE FROM users WHERE id = ?`, userID},
	}
	for _, st := range stmts {
		if _, err := tx.ExecContext(ctx, st.sql, st.arg); err != nil {
			// A missing optional table (older schema) shouldn't block the delete.
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			http.Error(w, `{"error": "Failed to delete user"}`, http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error": "Failed to commit deletion"}`, http.StatusInternalServerError)
		return
	}

	logs.New("Users").Info("user deleted", slog.String("user_id", userID), slog.String("by", string(actorID)))
	json.NewEncoder(w).Encode(map[string]any{"success": true, "deleted": userID})
}

// handleAdminSetPassword handles POST /api/admin/users/{id}/password — admin
// sets any user's password (rotation/offboarding). The admin check is already
// performed by the caller (handleUpdateUserRole). All of the target's sessions
// are invalidated so they must re-login with the new password.
func (s *Server) handleAdminSetPassword(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if userID == "" {
		http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
		return
	}

	if s.authAdapter == nil {
		http.Error(w, `{"error": "Auth service not available"}`, http.StatusInternalServerError)
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Cross-tenant guard: the target must be in the acting admin's workspace.
	actorID := authorization.GetActorID(r.Context())
	if ok, msg := s.sameWorkspaceAsActor(r.Context(), actorID, userID); !ok {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, msg), http.StatusForbidden)
		return
	}

	if err := s.authAdapter.service.AdminSetPassword(r.Context(), userID, req.NewPassword); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user_id": userID,
	})
}
