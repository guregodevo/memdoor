package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"time"

	"memdoor/gateway/email"
	"memdoor/pkg/domain"
	"memdoor/pkg/shared"
)

// invite_email_handlers: invite validation, transactional email, invites, demo requests.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// handleValidateInvite handles GET /api/invite/validate?token=xxx (no auth required)
// Validates an invite token and returns the associated email
func (s *Server) handleValidateInvite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	inviteToken := r.URL.Query().Get("token")
	if inviteToken == "" {
		http.Error(w, `{"error": "Token is required"}`, http.StatusBadRequest)
		return
	}

	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	var inviteEmail, channelName string
	var expiresAt time.Time
	var usedAt sql.NullTime
	err := db.QueryRowContext(r.Context(),
		`SELECT email, channel_name, expires_at, used_at FROM invites WHERE token = ?`,
		inviteToken,
	).Scan(&inviteEmail, &channelName, &expiresAt, &usedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "Invalid invite token"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error": "Failed to validate invite"}`, http.StatusInternalServerError)
		return
	}

	if usedAt.Valid {
		http.Error(w, `{"error": "Invite already used"}`, http.StatusGone)
		return
	}

	if time.Now().After(expiresAt) {
		http.Error(w, `{"error": "Invite expired"}`, http.StatusGone)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":        true,
		"email":        inviteEmail,
		"channel_name": channelName,
	})
}

// handleSendEmail handles POST /api/email/send
// Sends an arbitrary email via the configured email provider
func (s *Server) handleSendEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only allow internal service calls (from agent tools). isInternalActor
	// is set by the auth middleware ONLY for loopback X-Internal-Service
	// requests, so a remote attacker forging the header cannot reach the
	// mailer (previously this read the raw header and was an open relay).
	if !isInternalActor(r) {
		http.Error(w, `{"error": "Internal service access required"}`, http.StatusForbidden)
		return
	}

	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
		IsHTML  bool   `json:"is_html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.To == "" || req.Subject == "" || req.Body == "" {
		http.Error(w, `{"error": "to, subject, and body are required"}`, http.StatusBadRequest)
		return
	}

	if s.emailService == nil {
		http.Error(w, `{"error": "Email service not configured"}`, http.StatusServiceUnavailable)
		return
	}

	// Wrap body with Memdoor footer
	body := req.Body
	if req.IsHTML {
		body += `<br><hr style="border:none;border-top:1px solid #e5e7eb;margin:30px 0"><p style="color:#9ca3af;font-size:12px;text-align:center;">Sent via <a href="https://memdoor.ai" style="color:#667eea;text-decoration:none;">Memdoor</a></p>`
	} else {
		body += "\n\n---\nSent via Memdoor (https://memdoor.ai)"
	}

	if err := s.emailService.Send(email.Email{
		To:      req.To,
		Subject: req.Subject,
		Body:    body,
		IsHTML:  req.IsHTML,
	}); err != nil {
		slog.Error("Failed to send email", slog.String("to", req.To), slog.String("error", err.Error()))
		http.Error(w, `{"error": "Failed to send email"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("Email sent via agent tool", slog.String("to", req.To), slog.String("subject", req.Subject))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Email sent to %s", req.To),
	})
}

// handleSendInvite handles POST /api/invite
// Sends an invite email to join the workspace
func (s *Server) handleSendInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Allow internal service calls (from agent tools) or authenticated users.
	// isInternalActor is set by the auth middleware only for loopback
	// X-Internal-Service calls — do not trust the raw header from a remote.
	inviterName := "Memdoor"
	isInternal := isInternalActor(r)

	if !isInternal {
		// Require auth for non-internal calls
		token := r.Header.Get("Authorization")
		if token == "" || s.authAdapter == nil {
			http.Error(w, `{"error": "Authorization required"}`, http.StatusUnauthorized)
			return
		}
		token = strings.TrimPrefix(token, "Bearer ")
		user, err := s.authAdapter.VerifyToken(token)
		if err != nil {
			http.Error(w, `{"error": "Invalid token"}`, http.StatusUnauthorized)
			return
		}
		inviterName = user.Name
	}

	var req struct {
		Email       string `json:"email"`
		ChannelName string `json:"channel_name"`
		Message     string `json:"message"`
		InviterName string `json:"inviter_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, `{"error": "Email is required"}`, http.StatusBadRequest)
		return
	}

	// Internal calls can override inviter name (e.g. agent sending on behalf of user)
	if isInternal && req.InviterName != "" {
		inviterName = req.InviterName
	}

	if s.emailService == nil {
		http.Error(w, `{"error": "Email service not configured"}`, http.StatusServiceUnavailable)
		return
	}

	// Create invite token in database
	inviteToken := generateUUID()
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok {
		http.Error(w, `{"error": "Database not available"}`, http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days
	_, err := db.ExecContext(r.Context(),
		`INSERT INTO invites (token, email, workspace_id, invited_by, channel_name, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		inviteToken, req.Email, domain.DefaultWorkspaceID, inviterName, req.ChannelName, expiresAt,
	)
	if err != nil {
		slog.Error("Failed to create invite token", slog.String("error", err.Error()))
		http.Error(w, `{"error": "Failed to create invite"}`, http.StatusInternalServerError)
		return
	}

	inviteWS := ""
	if execCtx := shared.GetExecutionContext(r.Context()); execCtx != nil {
		inviteWS = execCtx.WorkspaceSlug
	}
	if err := s.emailService.SendInviteWithToken(req.Email, inviterName, req.ChannelName, req.Message, s.baseURL, inviteToken, inviteWS); err != nil {
		slog.Error("Failed to send invite email", slog.String("to", req.Email), slog.String("error", err.Error()))
		http.Error(w, `{"error": "Failed to send invite"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("Invite email sent", slog.String("to", req.Email), slog.String("inviter", inviterName))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Invite sent to %s", req.Email),
	})
}

// handleDemoRequest handles POST /api/demo-request (no auth required)
// Sends a notification email to the support address with the demo request details
func (s *Server) handleDemoRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email          string `json:"email"`
		YouTubeChannel string `json:"youtube_channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	notifyEmail := os.Getenv("DEMO_NOTIFY_EMAIL")
	if notifyEmail == "" {
		slog.Warn("DEMO_NOTIFY_EMAIL not set, demo request logged only",
			slog.String("from", req.Email),
			slog.String("youtube", req.YouTubeChannel))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	err := s.emailService.SendDemoRequest(notifyEmail, req.Email, req.YouTubeChannel)
	if err != nil {
		slog.Error("Failed to send demo request notification",
			slog.String("error", err.Error()),
			slog.String("from", req.Email))
	}

	slog.Info("Demo request received",
		slog.String("email", req.Email),
		slog.String("youtube", req.YouTubeChannel))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
