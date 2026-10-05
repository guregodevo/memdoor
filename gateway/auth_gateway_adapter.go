package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"memdoor/gateway/ratelimit"
	"memdoor/pkg/auth"
)

// loginRateLimiter throttles POST /api/auth/login by client IP to slow
// credential-stuffing / brute-force attempts. In-memory; resets on
// gateway restart, which is fine for an abuse-prevention layer.
var loginRateLimiter = ratelimit.NewPerClientLimiter(10, 1*time.Minute)

// forgotPasswordRateLimiter throttles POST /api/auth/forgot-password by
// client IP to prevent password-reset-email flooding / enumeration.
var forgotPasswordRateLimiter = ratelimit.NewPerClientLimiter(5, 1*time.Hour)

// AuthGatewayAdapter adapts pkg/auth.Service for gateway HTTP handlers
// Pattern: Adapter pattern to bridge domain service to HTTP layer
type AuthGatewayAdapter struct {
	service auth.Service
	db      *sql.DB // For invite token validation
}

// NewAuthGatewayAdapter creates a new auth gateway adapter
func NewAuthGatewayAdapter(service auth.Service) *AuthGatewayAdapter {
	return &AuthGatewayAdapter{service: service}
}

// SetDB wires the database connection for invite token validation
func (a *AuthGatewayAdapter) SetDB(db *sql.DB) {
	a.db = db
}

// RegisterRequest is the request body for user registration
type RegisterRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	InviteToken string `json:"invite_token"`
}

// LoginRequest is the request body for user login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is the response for successful authentication
type AuthResponse struct {
	User  auth.User `json:"user"`
	Token string    `json:"token"`
}

// HandleRegister handles POST /api/auth/register
func (a *AuthGatewayAdapter) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid request: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Enforce invite-only registration. The campaign signup bypass was
	// removed 2026-04-11 along with the rest of the campaign system.
	if a.db != nil {
		if req.InviteToken == "" {
			http.Error(w, `{"error": "Registration requires an invite. Please ask an admin for an invite link."}`, http.StatusForbidden)
			return
		}
		// Validate invite token
		var inviteEmail string
		var expiresAt time.Time
		var usedAt sql.NullTime
		err := a.db.QueryRowContext(r.Context(),
			`SELECT email, expires_at, used_at FROM invites WHERE token = ?`,
			req.InviteToken,
		).Scan(&inviteEmail, &expiresAt, &usedAt)

		if err == sql.ErrNoRows {
			http.Error(w, `{"error": "Invalid invite token"}`, http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, `{"error": "Failed to validate invite"}`, http.StatusInternalServerError)
			return
		}
		if usedAt.Valid {
			http.Error(w, `{"error": "This invite has already been used"}`, http.StatusGone)
			return
		}
		if time.Now().After(expiresAt) {
			http.Error(w, `{"error": "This invite has expired"}`, http.StatusGone)
			return
		}
		// Verify email matches invite
		if req.Email != inviteEmail {
			http.Error(w, fmt.Sprintf(`{"error": "This invite was sent to %s. Please register with that email."}`, inviteEmail), http.StatusForbidden)
			return
		}
	}

	_, err := a.service.Register(r.Context(), req.Email, req.Username, req.Password, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Mark invite as used
	if a.db != nil && req.InviteToken != "" {
		_, _ = a.db.ExecContext(r.Context(),
			`UPDATE invites SET used_at = CURRENT_TIMESTAMP WHERE token = ?`,
			req.InviteToken,
		)
	}

	// Auto-login after registration
	loginUser, token, err := a.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error": "Failed to auto-login after registration"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		User:  *loginUser,
		Token: token,
	})
}

// HandleLogin handles POST /api/auth/login
func (a *AuthGatewayAdapter) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ip := guestClientIP(r); !loginRateLimiter.Allow(ip) {
		http.Error(w, `{"error":"too many attempts, slow down"}`, http.StatusTooManyRequests)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid request: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	user, token, err := a.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		User:  *user,
		Token: token,
	})
}

// HandleWhoami handles GET /api/auth/whoami
func (a *AuthGatewayAdapter) HandleWhoami(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from Authorization header
	token := extractBearerToken(r)
	if token == "" {
		http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
		return
	}

	// Verify token
	user, err := a.service.VerifyToken(r.Context(), token)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}

// HandleLogout handles POST /api/auth/logout
func (a *AuthGatewayAdapter) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from Authorization header
	token := extractBearerToken(r)
	if token == "" {
		// Already logged out, return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Logged out successfully",
		})
		return
	}

	// Invalidate session
	if err := a.service.InvalidateSession(r.Context(), token); err != nil {
		// Log error but still return success
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// HandleVerifyEmail handles GET /api/auth/verify-email?token=xxx
func (a *AuthGatewayAdapter) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from query params
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error": "Verification token is required"}`, http.StatusBadRequest)
		return
	}

	// Verify email
	if err := a.service.VerifyEmail(r.Context(), token); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Email verified successfully! You can now use all features.",
	})
}

// HandleResendVerification handles POST /api/auth/resend-verification
func (a *AuthGatewayAdapter) HandleResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from Authorization header
	token := extractBearerToken(r)
	if token == "" {
		http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
		return
	}

	// Get user from token
	user, err := a.service.VerifyToken(r.Context(), token)
	if err != nil {
		http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	// Resend verification email
	if err := a.service.ResendVerification(r.Context(), user.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Verification email sent successfully. Please check your inbox.",
	})
}

// HandleForgotPassword handles POST /api/auth/forgot-password
func (a *AuthGatewayAdapter) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ip := guestClientIP(r); !forgotPasswordRateLimiter.Allow(ip) {
		http.Error(w, `{"error":"too many attempts, slow down"}`, http.StatusTooManyRequests)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Send password reset email
	if err := a.service.SendPasswordReset(r.Context(), req.Email); err != nil {
		// Don't reveal if email exists - always return success
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "If that email exists, a password reset link has been sent.",
	})
}

// HandleResetPassword handles POST /api/auth/reset-password
func (a *AuthGatewayAdapter) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Reset password
	if err := a.service.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password reset successfully. Please login with your new password.",
	})
}

// HandleChangePassword handles POST /api/auth/change-password
// A logged-in user changes their own password by proving the old one. The
// auth routes are not behind authMiddleware, so the authenticated user is
// resolved from the Bearer token directly (same pattern as
// HandleResendVerification). The current session is intentionally preserved.
func (a *AuthGatewayAdapter) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Resolve the authenticated user from the Bearer token.
	token := extractBearerToken(r)
	if token == "" {
		http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
		return
	}
	user, err := a.service.VerifyToken(r.Context(), token)
	if err != nil {
		http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if err := a.service.ChangePassword(r.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password changed.",
	})
}

// VerifyToken implements authorization.AuthService interface for middleware
// This is used by the auth middleware to verify Bearer tokens
func (a *AuthGatewayAdapter) VerifyToken(token string) (*auth.User, error) {
	user, err := a.service.VerifyToken(context.Background(), token)
	if err != nil {
		return nil, err
	}

	// Return the auth.User directly (no conversion needed - single source of truth)
	return user, nil
}

// ValidateToken implements websocket.TokenValidator interface for WebSocket authentication
// This is used by the WebSocket handler to verify Bearer tokens
func (a *AuthGatewayAdapter) ValidateToken(token string) (string, error) {
	user, err := a.service.VerifyToken(context.Background(), token)
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

// extractBearerToken extracts the Bearer token from Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// guestClientIP is the address the sign-in limiter keys on: X-Real-IP, which
// nginx sets to the peer's address, else RemoteAddr. Never X-Forwarded-For:
// nginx appends to it, so its first value is the client's own claim.
func guestClientIP(r *http.Request) string {
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}
	addr := r.RemoteAddr
	if colon := strings.LastIndex(addr, ":"); colon >= 0 {
		return addr[:colon]
	}
	return addr
}
