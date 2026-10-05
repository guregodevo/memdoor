package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"memdoor/pkg/shared"
)

// setup_handlers: first-run setup: status and the setup wizard.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// Read settings from DB

// Stop existing runner

// Update config

// Start new runner if enabled

// handleSetupStatus handles GET /api/setup/status (no auth required)
// Returns whether the workspace has been initialized (has any users)
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
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

	// Check if any users exist (workspace is initialized)
	var userCount int
	err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&userCount)
	if err != nil {
		// Table may not exist yet
		userCount = 0
	}

	// Resolve both the display name AND the canonical slug. The slug is
	// what every workspace-scoped route keys on — slugifying the display
	// name client-side produces the wrong result for names that aren't a
	// straight word.
	wsSlug := ""
	if execCtx := shared.GetExecutionContext(r.Context()); execCtx != nil {
		wsSlug = execCtx.WorkspaceSlug
	}
	var workspaceName, workspaceSlug string
	if wsSlug != "" {
		_ = db.QueryRowContext(r.Context(), `SELECT name, slug FROM workspaces WHERE slug = ?`, wsSlug).Scan(&workspaceName, &workspaceSlug)
	} else {
		_ = db.QueryRowContext(r.Context(), `SELECT name, slug FROM workspaces ORDER BY created_at ASC LIMIT 1`).Scan(&workspaceName, &workspaceSlug)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"initialized":    userCount > 0,
		"workspace_name": workspaceName,
		"workspace_slug": workspaceSlug,
	})
}

// handleSetup handles POST /api/setup (no auth, one-time only)
// Creates the workspace and first admin user
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Check if already initialized (prevent re-setup)
	var userCount int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&userCount); err == nil && userCount > 0 {
		http.Error(w, `{"error": "Workspace already initialized"}`, http.StatusConflict)
		return
	}

	var req struct {
		WorkspaceName string `json:"workspace_name"`
		Language      string `json:"language"`
		Email         string `json:"email"`
		Username      string `json:"username"`
		Password      string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || req.WorkspaceName == "" {
		http.Error(w, `{"error": "workspace_name, email, and password are required"}`, http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, `{"error": "Password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	// Default username from email. The local part alone fails validation when
	// it is shorter than three characters, and setup then died on "username
	// must be 3-30 characters" — a field nobody typed (onboarding walk with
	// s@example.com, 2026-09-27). Borrow from the domain, and cap at 30.
	if req.Username == "" {
		req.Username = usernameFromEmail(req.Email)
	}

	// Default language
	if req.Language == "" {
		req.Language = "en"
	}

	// Workspace id == slug == slugify(WorkspaceName). This aligns the
	// freshly-created workspace with the id-equals-slug convention
	// every newer install (cyberlaw, hackernews, …) already uses, and
	// what the rest of the codebase queries against. The legacy UUID-
	// keyed sentinel (domain.DefaultWorkspaceID) is still referenced
	// by boot-time fallbacks and the invite handler for backward
	// compat; migration 014 rewrites any pre-existing UUID-keyed
	// default workspaces to slug-keyed in place.
	workspaceID := slugifyWorkspace(req.WorkspaceName)
	if workspaceID == "" {
		workspaceID = "default"
	}

	// Create or update workspace record (language is on the workspace row).
	_, err := db.ExecContext(r.Context(),
		`INSERT INTO workspaces (id, name, slug, owner_id, language, plan, status, created_at, updated_at)
		 VALUES (?, ?, ?, '', ?, 'free', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, language = excluded.language, updated_at = CURRENT_TIMESTAMP`,
		workspaceID, req.WorkspaceName, workspaceID, req.Language,
	)
	if err != nil {
		s.log.Error("Failed to create/update workspace", slog.String("error", err.Error()))
	}

	// Register first admin user via auth service
	if s.authAdapter == nil {
		http.Error(w, `{"error": "Auth service not available"}`, http.StatusInternalServerError)
		return
	}

	// Register the user
	_, registerErr := s.authAdapter.service.Register(r.Context(), req.Email, req.Username, req.Password, req.Username)
	if registerErr != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, registerErr.Error()), http.StatusBadRequest)
		return
	}

	// Promote to admin
	_, err = db.ExecContext(r.Context(),
		`UPDATE users SET role = 'admin' WHERE email = ?`, req.Email,
	)
	if err != nil {
		s.log.Error("Failed to promote user to admin", slog.String("error", err.Error()))
	}

	// Get admin user ID for channel membership and DM
	var adminUserID string
	_ = db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE email = ?`, req.Email).Scan(&adminUserID)

	// Create default #general channel with admin as member
	s.seedDefaultChannel(r.Context(), db, workspaceID, adminUserID)

	// Create Chief of Staff agent and DM with welcome message
	s.seedChiefAgent(r.Context(), db, workspaceID, adminUserID, req.WorkspaceName)

	// The coding team: the coder, its narrator, the planner, the verifier and
	// the runner.
	s.seedCoderAgent(r.Context(), db, workspaceID)
	s.seedNarratorAgent(r.Context(), db, workspaceID)
	s.seedPlannerAgent(r.Context(), db, workspaceID)
	s.seedVerifierAgent(r.Context(), db, workspaceID)
	s.seedRunnerAgent(r.Context(), db, workspaceID)

	// Auto-login
	user, token, err := s.authAdapter.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error": "Setup complete but auto-login failed"}`, http.StatusInternalServerError)
		return
	}

	s.log.Info("Workspace setup complete",
		slog.String("workspace", req.WorkspaceName),
		slog.String("workspace_slug", workspaceID),
		slog.String("admin", req.Email))

	// Return the resolved slug so the CLI can pass the right -w to
	// downstream subcommands without re-slugifying the workspace name
	// itself and risking drift from the server-side rule.
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"user":           user,
		"token":          token,
		"workspace_slug": workspaceID,
	})
}

// usernameFromEmail derives a username the user never has to think about:
// the local part, extended with the domain's first label when it is too short,
// and truncated to the 30 characters validation allows.
func usernameFromEmail(email string) string {
	local, domain, _ := strings.Cut(email, "@")
	name := local
	if len(name) < 3 {
		if label, _, _ := strings.Cut(domain, "."); label != "" {
			name = local + "-" + label
		}
	}
	for len(name) < 3 {
		name += "0"
	}
	if len(name) > 30 {
		name = name[:30]
	}
	return name
}

// slugifyWorkspace turns a workspace's display name into its id: letters and
// digits kept (in any script), everything else collapsed to single dashes,
// lower-cased. A name that reduces to nothing becomes "untitled", never an
// empty id. It transliterated to ASCII through a library that came in with a
// deleted feature and left with it (2026-09-27); a workspace id does not need
// to be ASCII to be a valid key.
var slugStripRe = regexp.MustCompile(`[^\p{L}\p{N}-]+`)
var slugDashRe = regexp.MustCompile(`-{2,}`)

func slugifyWorkspace(s string) string {
	s = strings.TrimSpace(s)
	s = slugStripRe.ReplaceAllString(s, "-")
	s = slugDashRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	return strings.ToLower(s)
}
