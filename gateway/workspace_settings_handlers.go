package gateway

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"memdoor/pkg/authorization"
	"memdoor/pkg/secrets"
	"memdoor/pkg/shared"
)

// workspace_settings_handlers: workspace settings read/update + DM-available users.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// handleWorkspaceSettings handles GET/PUT /api/workspace/settings
// GET: Returns workspace settings (any authenticated user can read)
// PUT: Updates workspace settings (admin only)
func (s *Server) handleWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		s.handleGetWorkspaceSettings(w, r)
	case http.MethodPut:
		s.handleUpdateWorkspaceSettings(w, r)
	default:
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	settings := map[string]string{}
	rows, err := db.QueryContext(r.Context(), `SELECT key, value FROM workspace_settings`)
	if err != nil {
		// Table might not exist yet on older databases
		json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		// Mask provider API keys — decrypt first so the mask shows
		// the plaintext key's first/last 4 (recognizable to the
		// user) rather than the ciphertext's. Legacy plaintext rows
		// from before encryption was wired fall through to the
		// raw mask via the decrypt-error branch.
		if strings.HasPrefix(key, "provider_key:") {
			plain := value
			if value != "" {
				if pt, err := secrets.Decrypt(value); err == nil {
					plain = pt
				}
			}
			if len(plain) > 8 {
				settings[key] = plain[:4] + "****" + plain[len(plain)-4:]
			} else {
				settings[key] = plain
			}
		} else {
			settings[key] = value
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
}

// handleUpdateWorkspaceSettings handles PUT /api/workspace/settings
// Updates workspace settings (admin only)
func (s *Server) handleUpdateWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Workspace slug from URL prefix
	wsSlug := ""
	if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
		wsSlug = execCtx.WorkspaceSlug
	}

	// Language settings are workspace-scoped; the rest are global
	workspaceScopedKeys := map[string]bool{
		"language": true,
	}
	globalKeys := globalSettingKeys

	// Provider/model routing is not a workspace setting: agent_provider:<name>
	// and agent_model:<name> writes are refused here (and named in the
	// response). Routing comes from the provider registry — the environment
	// and ~/.memdoor/providers.json — and a /model pin.
	var rejected []string
	for key, value := range body {
		scopeID := ""
		if workspaceScopedKeys[key] {
			scopeID = wsSlug
		} else if !globalKeys[key] {
			// Rejected keys are REPORTED, not silently dropped (live failure:
			// a tool_guards PUT returned {"ok":true} having written nothing,
			// and the absent guard was only discovered by an e2e test). The
			// provider/model lock-down keys stay unwritable — they are just
			// named in the response now.
			rejected = append(rejected, key)
			continue
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO workspace_settings (workspace_id, key, value, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(workspace_id, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
			scopeID, key, value,
		)
		if err != nil {
			slog.Error("Failed to update workspace setting", "key", key, "error", err)
			http.Error(w, `{"error": "Failed to update setting"}`, http.StatusInternalServerError)
			return
		}
	}

	if len(rejected) > 0 && len(rejected) == len(body) {
		// Nothing was written at all — that is an error, not an "ok".
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "rejected": rejected,
			"error": "no writable settings in request (unknown or locked keys)"})
		return
	}
	resp := map[string]interface{}{"ok": true}
	if len(rejected) > 0 {
		resp["rejected"] = rejected
	}
	json.NewEncoder(w).Encode(resp)
}

// handleGetWorkspaceSettingsPublic handles GET /api/workspace/settings/public
// Returns non-sensitive workspace settings without authentication (for language detection on login page)
func (s *Server) handleGetWorkspaceSettingsPublic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Workspace slug from URL prefix (set by workspaceMiddleware)
	wsSlug := ""
	if execCtx := shared.GetExecutionContext(r.Context()); execCtx != nil {
		wsSlug = execCtx.WorkspaceSlug
	}

	publicKeys := []string{"language"}
	settings := map[string]string{}

	for _, key := range publicKeys {
		var value string
		err := db.QueryRowContext(r.Context(),
			`SELECT value FROM workspace_settings WHERE workspace_id = ? AND key = ?`, wsSlug, key).Scan(&value)
		if err == nil {
			settings[key] = value
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
}

// handleAvailableUsersForDM handles GET /api/users/available-for-dm
// Lists users who share at least one channel with current user (authenticated users only)
func (s *Server) handleAvailableUsersForDM(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get authenticated user
	actorID := authorization.GetActorID(ctx)
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}

	// Get database from repository factory
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Query: Find all users and agents who share at least one channel with current user
	// Exclude the current user themselves, but include both humans and agents
	// Include avatar_emoji and icon for displaying agent/human avatars in UI
	query := `
		SELECT DISTINCT u.id, u.name, u.avatar_emoji, u.icon
		FROM users u
		INNER JOIN channel_memberships cm ON u.id = cm.actor_id
		WHERE cm.channel_id IN (
			SELECT channel_id
			FROM channel_memberships
			WHERE actor_id = ?
		)
		AND u.id != ?
		ORDER BY u.name ASC
	`

	rows, err := db.QueryContext(ctx, query, actorID.String(), actorID.String())
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch available users"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AvailableUser struct {
		ID          string `json:"id"`                     // Actor ID (with "human:" or "agent:" prefix)
		Name        string `json:"name"`                   // Display name
		AvatarEmoji string `json:"avatar_emoji,omitempty"` // Agent emoji avatar (if agent)
		Icon        string `json:"icon,omitempty"`         // Agent custom icon (if agent)
	}

	users := []AvailableUser{}
	for rows.Next() {
		var user AvailableUser
		var avatarEmoji, icon sql.NullString
		err := rows.Scan(&user.ID, &user.Name, &avatarEmoji, &icon)
		if err != nil {
			http.Error(w, `{"error": "Failed to scan user data"}`, http.StatusInternalServerError)
			return
		}
		if avatarEmoji.Valid {
			user.AvatarEmoji = avatarEmoji.String
		}
		if icon.Valid {
			user.Icon = icon.String
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, `{"error": "Failed to iterate user data"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// globalSettingKeys are the gateway-wide settings the PUT may write; any
// other key is rejected and named in the response. A decision setting
// missing here cannot be switched (2026-09-26: decision_turn_verdict,
// decision_result_acceptance and its threshold were read by the gateway
// and unwritable) — TestDecisionSettingsAreWritable holds them together.
var globalSettingKeys = map[string]bool{
	"tool_guards":     true, // admin-configured tool block rules (gateway/tool_guards.go)
	settingApprove:    true, // approval mode: "changes" asks before bash, writes, MCP tools (gateway/approval.go)
	settingEditFormat: true, // patch | hashline (docs/features/HASHLINE.md)
	// Decision model slot (gateway/decisions.go): which provider answers typed
	// questions, and each consumer's switch.
	settingDecisionGate:              true,
	settingDecisionThreshold:         true,
	settingToolRouting:               true,
	settingToolFamilies:              true,
	settingTurnVerdict:               true,
	settingTurnStop:                  true,
	settingResultAcceptance:          true,
	settingResultAcceptanceThreshold: true,
}
