package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"log/slog"

	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// handleDeleteWorkspace cascades a full workspace delete:
//   - resolves slug → workspace_id from the workspaces table
//   - within a single transaction, deletes every workspace-scoped row
//     across the SQL schema (channels, messages, sessions, buddies,
//     cron_*, agent_*, etc.)
//   - after the SQL commit, removes the filesystem workspace dir
//     (~/.memdoor/workspaces/<slug>/) and the RAG store
//     (~/.memdoor/rag/<workspace_id>.db)
//
// Admin-only. Idempotent in the sense that a missing workspace returns
// 404, but partial failure (SQL ok, filesystem rm fails) is reported
// with 200 + a `warnings` field rather than rolled back — the row is
// gone so the workspace is functionally unusable; the leftover files
// are flagged for the admin to clean up by hand.
func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodDelete {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Auth: admin only.
	actorID := authorization.GetActorID(ctx)
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}
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

	// Parse slug from /api/workspaces/{slug}
	slug := strings.TrimPrefix(r.URL.Path, "/api/workspaces/")
	slug = strings.Trim(slug, "/")
	if slug == "" || strings.Contains(slug, "/") {
		http.Error(w, `{"error": "workspace slug required in URL"}`, http.StatusBadRequest)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Resolve slug → workspace_id.
	var workspaceID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE slug = ?`, slug).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "workspace not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("workspace delete: lookup failed", slog.String("slug", slug), slog.String("error", err.Error()))
		http.Error(w, `{"error": "lookup failed"}`, http.StatusInternalServerError)
		return
	}

	// Scoped re-check now that we know the concrete target workspace: an admin
	// of workspace A must not be able to delete workspace B (cross-tenant IDOR).
	// The early generic admin check above stays as a cheap first gate.
	if s.authzService != nil {
		ok, err := s.authzService.IsWorkspaceAdminFor(ctx, actorID, workspaceID)
		if err != nil {
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, `{"error":"Permission denied: not an admin of this workspace"}`, http.StatusForbidden)
			return
		}
	}

	// Transactional cascade.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error": "tx begin failed"}`, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Order: leaf-first by FK direction (transitively-scoped rows
	// before their parents). SQLite doesn't enforce FKs by default
	// so the order is for clarity, not correctness.
	cascadeStmts := []string{
		// Reactions hang off messages, which hang off channels.
		`DELETE FROM reactions WHERE message_id IN (
			SELECT id FROM messages WHERE channel_id IN (
				SELECT id FROM channels WHERE workspace_id = ?
			)
		)`,
		`DELETE FROM messages WHERE channel_id IN (
			SELECT id FROM channels WHERE workspace_id = ?
		)`,
		`DELETE FROM channel_memberships WHERE channel_id IN (
			SELECT id FROM channels WHERE workspace_id = ?
		)`,
		`DELETE FROM channels WHERE workspace_id = ?`,
		// Cron history hangs off cron_jobs.
		`DELETE FROM cron_history WHERE job_id IN (
			SELECT id FROM cron_jobs WHERE workspace_id = ?
		)`,
		`DELETE FROM cron_jobs WHERE workspace_id = ?`,
		// subagent_runs has no workspace_id column and is keyed by
		// opaque session keys (child_session_key, requester_session_key)
		// that don't expose the workspace. Skip it — orphan rows are
		// harmless and they age out via cleanup_completed_at.
		`DELETE FROM sessions WHERE workspace_id = ?`,
		// Agent / buddy state with workspace_id.
		// NOTE: agent_memories (per-agent, no workspace_id) and
		// agent_secrets (FK CASCADE from buddies) and files (FK
		// CASCADE from channels) are intentionally not in this list —
		// they either cascade via FK or are workspace-decoupled.
		`DELETE FROM agent_events WHERE workspace_id = ?`,
		`DELETE FROM buddy_memory WHERE workspace_id = ?`,
		`DELETE FROM buddies WHERE workspace_id = ?`,
		// Workspace-level tables.
		`DELETE FROM workspace_settings WHERE workspace_id = ?`,
		`DELETE FROM invites WHERE workspace_id = ?`,
		`DELETE FROM users WHERE workspace_id = ?`,
		// Finally the workspace row itself (by id, not workspace_id).
		`DELETE FROM workspaces WHERE id = ?`,
	}

	rowsDeleted := map[string]int64{}
	for i, stmt := range cascadeStmts {
		// All cascade statements take exactly one parameter — the
		// workspace_id (or the workspaces.id for the last stmt).
		res, err := tx.ExecContext(ctx, stmt, workspaceID)
		if err != nil {
			// Tables may not exist on older deployments; tolerate
			// "no such table" but fail other errors.
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			slog.Error("workspace delete: cascade failed",
				slog.Int("step", i),
				slog.String("workspace_id", workspaceID),
				slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf(`{"error": "cascade failed at step %d: %s"}`, i, err.Error()), http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			// Use the table name (first word after DELETE FROM) as
			// the key so the response is human-readable.
			parts := strings.Fields(stmt)
			if len(parts) >= 3 {
				rowsDeleted[parts[2]] += n
			}
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error": "commit failed"}`, http.StatusInternalServerError)
		return
	}
	committed = true

	// Filesystem + RAG cleanup. Best-effort: if these fail, the SQL
	// is already gone, so report a warning rather than rolling back.
	warnings := []string{}
	wsDir := shared.MemdoorHome("workspaces", slug)
	if err := os.RemoveAll(wsDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("filesystem rm %s: %s", wsDir, err.Error()))
	}
	ragDB := shared.MemdoorHome("rag", workspaceID+".db")
	if err := os.Remove(ragDB); err != nil && !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("rag rm %s: %s", ragDB, err.Error()))
	}

	slog.Info("workspace deleted",
		slog.String("slug", slug),
		slog.String("workspace_id", workspaceID),
		slog.Any("rows_deleted", rowsDeleted),
		slog.Any("warnings", warnings))

	resp := map[string]interface{}{
		"slug":         slug,
		"workspace_id": workspaceID,
		"rows_deleted": rowsDeleted,
	}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	json.NewEncoder(w).Encode(resp)
}
