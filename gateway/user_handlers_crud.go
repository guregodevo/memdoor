package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"memdoor/pkg/authorization"
)

// user_handlers_crud: user CRUD + avatar: create, list, get, list-workspace-users.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// handleAvatarUpload handles PUT /api/users/avatar - Upload profile picture
func (s *Server) handleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, `{"error": "File too large (max 5MB)"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, `{"error": "No file provided"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(mimeType, "image/") {
		http.Error(w, `{"error": "Only image files are allowed"}`, http.StatusBadRequest)
		return
	}

	if s.fileService == nil {
		http.Error(w, `{"error": "File sharing not configured"}`, http.StatusServiceUnavailable)
		return
	}

	attachment, err := s.fileService.Upload(r.Context(), "__avatars__", actorID, header.Filename, mimeType, file)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	avatarURL := "/api/files/" + attachment.ID

	// Update user's avatar_url in DB
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	_, err = db.ExecContext(r.Context(),
		`UPDATE users SET avatar_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		avatarURL, actorID.String(),
	)
	if err != nil {
		http.Error(w, `{"error": "Failed to update profile"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"avatar_url": avatarURL,
	})
}

// handleGetUserByID handles GET /api/users/:id - Get single user by ID
func (s *Server) handleGetUserByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from /api/users/:id
	userID := strings.TrimPrefix(r.URL.Path, "/api/users/")
	userID = strings.TrimSuffix(userID, "/")

	if userID == "" {
		http.Error(w, `{"error": "User ID required"}`, http.StatusBadRequest)
		return
	}

	s.handleGetUser(w, r, userID)
}

// handleListUsers handles GET /api/admin/users (list) + POST /api/admin/users
// (create — admin-provisioned user, bypasses the invite gate on the public
// /api/auth/register endpoint).
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreateUser(w, r)
		return
	}

	ctx := r.Context()

	// Get authenticated user
	actorID := authorization.GetActorID(ctx)
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}

	// AUTHORIZATION CHECK: Only admins can list users
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

	// Get database from repository factory
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	// Cross-tenant scope: a workspace admin may only list users in their own
	// workspace (prevents a cross-tenant email leak). system:internal is a
	// global trusted caller and sees all users.
	var workspaceFilter string
	if actorID != "system:internal" {
		if s.authAdapter == nil {
			http.Error(w, `{"error": "Auth service not available"}`, http.StatusInternalServerError)
			return
		}
		actor, err := s.authAdapter.service.GetUserByID(ctx, string(actorID))
		if err != nil {
			http.Error(w, `{"error": "Failed to load acting user"}`, http.StatusInternalServerError)
			return
		}
		workspaceFilter = actor.WorkspaceID
	}

	// Query database for users (scoped to the actor's workspace unless internal)
	type UserListItem struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		AvatarURL     string `json:"avatar_url,omitempty"`
		Role          string `json:"role"`
		EmailVerified bool   `json:"email_verified"`
		CreatedAt     string `json:"created_at"`
	}

	var rows *sql.Rows
	var err error
	if workspaceFilter != "" {
		query := `SELECT id, name, email, avatar_url, role, email_verified, created_at FROM users WHERE workspace_id = ? ORDER BY created_at DESC`
		rows, err = db.QueryContext(ctx, query, workspaceFilter)
	} else {
		query := `SELECT id, name, email, avatar_url, role, email_verified, created_at FROM users ORDER BY created_at DESC`
		rows, err = db.QueryContext(ctx, query)
	}
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch users"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []UserListItem{}
	for rows.Next() {
		var user UserListItem
		var avatarURL sql.NullString
		err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&avatarURL,
			&user.Role,
			&user.EmailVerified,
			&user.CreatedAt,
		)
		if err != nil {
			http.Error(w, `{"error": "Failed to scan user data"}`, http.StatusInternalServerError)
			return
		}

		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
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

// handleCreateUser handles POST /api/admin/users — admin-provisioned user
// creation. Bypasses the invite-only gate on /api/auth/register because the
// caller is already an authenticated admin. Optional `admin: true` promotes
// the new user to workspace admin in the same call. The new account is
// marked email_verified=true since the admin is vouching for it.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

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

	if s.authAdapter == nil {
		http.Error(w, `{"error": "Auth service not available"}`, http.StatusInternalServerError)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Admin    bool   `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error": "email and password are required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error": "Password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		req.Username = strings.Split(req.Email, "@")[0]
	}
	if req.Name == "" {
		req.Name = req.Username
	}

	user, err := s.authAdapter.service.Register(ctx, req.Email, req.Username, req.Password, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	if db, ok := s.repoFactory.DB().(*sql.DB); ok {
		if _, err := db.ExecContext(ctx, `UPDATE users SET email_verified = 1 WHERE email = ?`, req.Email); err != nil {
			s.log.Warn("Failed to mark admin-created user as verified", slog.String("error", err.Error()), slog.String("email", req.Email))
		}
		if req.Admin {
			if _, err := db.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE email = ?`, req.Email); err != nil {
				s.log.Warn("Failed to promote new user to admin", slog.String("error", err.Error()), slog.String("email", req.Email))
			}
		}
	}

	s.log.Info("User created via admin endpoint", slog.String("email", req.Email), slog.Bool("admin", req.Admin), slog.String("created_by", string(actorID)))

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}

// handleListWorkspaceUsers handles GET /api/workspace/users
// Lists all users in workspace for mention autocomplete (authenticated users only)
func (s *Server) handleListWorkspaceUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()

	// Get authenticated user - any authenticated user can list workspace users
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

	// Query database for all users (minimal info for mention autocomplete + avatars)
	type WorkspaceUser struct {
		ID        string `json:"id"`                   // Actor ID (with "human:" prefix)
		Name      string `json:"name"`                 // Display name
		AvatarURL string `json:"avatar_url,omitempty"` // Profile picture URL
	}

	query := `SELECT id, name, avatar_url FROM users ORDER BY name ASC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch users"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []WorkspaceUser{}
	for rows.Next() {
		var user WorkspaceUser
		var avatarURL sql.NullString
		err := rows.Scan(&user.ID, &user.Name, &avatarURL)
		if err != nil {
			http.Error(w, `{"error": "Failed to scan user data"}`, http.StatusInternalServerError)
			return
		}
		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
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
