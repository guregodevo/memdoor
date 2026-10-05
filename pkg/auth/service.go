package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"memdoor/pkg/domain"

	"github.com/google/uuid"
)

// Logger is the duck-typed interface for structured logging.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// SessionTTL is how long a CLI/web login stays valid before the user must
// re-authenticate. 30 days matches the convention of long-lived CLI tools
// (gh, gcloud, aws). Tokens are stored hashed in SQLite, so they survive
// gateway restarts; only natural expiry or `memdoor auth logout` clears them.
const SessionTTL = 30 * 24 * time.Hour

// service implements the Service interface
// Pattern: Dependency injection with interfaces
type service struct {
	repo           Repository
	emailService   EmailService
	tokenGenerator TokenGenerator
	logger         Logger
	baseURL        string
	workspaceID    string // Workspace ID for this server instance
}

// NewService creates a new auth service with dependency injection
func NewService(repo Repository, emailService EmailService, tokenGen TokenGenerator, logger Logger, baseURL string, workspaceID string) Service {
	return &service{
		repo:           repo,
		emailService:   emailService,
		tokenGenerator: tokenGen,
		logger:         logger,
		baseURL:        baseURL,
		workspaceID:    workspaceID,
	}
}

// Register creates a new user account
func (s *service) Register(ctx context.Context, email, username, password, name string) (*User, error) {
	// Validate inputs
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	// Validate username format (alphanumeric, hyphens, underscores, 3-30 chars)
	if len(username) < 3 || len(username) > 30 {
		return nil, fmt.Errorf("username must be 3-30 characters")
	}
	for _, ch := range username {
		isAlphanumeric := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		isAllowedSymbol := ch == '-' || ch == '_'
		if !isAlphanumeric && !isAllowedSymbol {
			return nil, fmt.Errorf("username must be alphanumeric with hyphens or underscores")
		}
	}

	// Check if email already exists
	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	// Check if username already exists
	existingUser, err := s.repo.GetUserByUsername(ctx, username)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("username already taken")
	}

	// Hash password
	passwordHash, err := s.tokenGenerator.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	now := time.Now()

	// Use server's workspace_id from config, fallback to default if not set
	workspaceID := s.workspaceID
	if workspaceID == "" {
		s.logger.Warn("service.workspaceID is empty, using default",
			"default", domain.DefaultWorkspaceID)
		workspaceID = domain.DefaultWorkspaceID
	} else {
		s.logger.Debug("Using workspace_id from config",
			"workspace_id", workspaceID)
	}

	user := &User{
		BaseUser: domain.BaseUser{
			ID:            fmt.Sprintf("human:%s", generateUUID()),
			Email:         email,
			Username:      username,
			Name:          name,
			EmailVerified: false,
			Timestamps: domain.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		WorkspaceID: workspaceID, // Assign to server's workspace
	}

	if err := s.repo.CreateUser(ctx, user, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send verification email (non-blocking - don't fail registration if email fails)
	if err := s.SendVerificationEmail(ctx, user.ID, user.Email, user.Name); err != nil {
		// Log error but don't fail registration
		// User can resend verification email later
	}

	return user, nil
}

// Login authenticates a user and returns a session token
func (s *service) Login(ctx context.Context, email, password string) (*User, string, error) {
	// Validate inputs
	if email == "" {
		return nil, "", fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, "", fmt.Errorf("password is required")
	}

	// Get user by email
	user, passwordHash, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	// Check password
	if !s.tokenGenerator.CheckPassword(password, passwordHash) {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	// Generate session token
	token, err := s.tokenGenerator.GenerateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	session := &Session{
		UserID:      user.ID,
		WorkspaceID: user.WorkspaceID,
		ExpiresAt:   time.Now().Add(SessionTTL),
		CreatedAt:   time.Now(),
	}

	tokenHash := s.tokenGenerator.HashToken(token)
	if err := s.repo.CreateSession(ctx, session, tokenHash); err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login time
	if err := s.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Log warning but don't fail login
	}

	return user, token, nil
}

// VerifyToken checks if a token is valid and returns the user
func (s *service) VerifyToken(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	// Hash token for lookup
	tokenHash := s.tokenGenerator.HashToken(token)

	// Get session
	session, err := s.repo.GetSessionByToken(ctx, tokenHash)
	if err != nil {
		// Improve error message to help with debugging
		// This could be a timing issue (session not committed yet) or truly invalid token
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session not found - token may be invalid or not created yet")
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// SendVerificationEmail generates a verification token and sends email
func (s *service) SendVerificationEmail(ctx context.Context, userID, email, name string) error {
	// Generate verification token
	token, err := s.tokenGenerator.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Store the HASH of the token, never the raw token (same as session
	// tokens). The raw token goes out in the email; a DB/backup leak then
	// can't be replayed to verify an account.
	expiresAt := time.Now().Add(24 * time.Hour)
	tokenHash := s.tokenGenerator.HashToken(token)
	if err := s.repo.CreateVerificationToken(ctx, userID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("failed to store verification token: %w", err)
	}

	// Send email
	if s.emailService != nil {
		if err := s.emailService.SendVerificationEmail(email, name, token, s.baseURL); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

// VerifyEmail marks a user's email as verified
func (s *service) VerifyEmail(ctx context.Context, token string) error {
	// Look up by the token's hash — the DB stores hashes, not raw tokens.
	tokenHash := s.tokenGenerator.HashToken(token)
	userID, expiresAt, err := s.repo.GetVerificationToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("invalid or expired verification token")
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		return fmt.Errorf("verification token has expired")
	}

	// Mark email as verified
	if err := s.repo.MarkEmailVerified(ctx, userID); err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	// Delete verification token (one-time use)
	if err := s.repo.DeleteVerificationToken(ctx, tokenHash); err != nil {
		// Log warning but don't fail verification
	}

	return nil
}

// ResendVerification resends the verification email
func (s *service) ResendVerification(ctx context.Context, userID string) error {
	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Check if already verified
	if user.EmailVerified {
		return fmt.Errorf("email already verified")
	}

	// Delete old verification tokens
	if err := s.repo.DeleteUserVerificationTokens(ctx, userID); err != nil {
		// Log warning but continue
	}

	// Send new verification email
	return s.SendVerificationEmail(ctx, userID, user.Email, user.Name)
}

// SendPasswordReset sends a password reset email
func (s *service) SendPasswordReset(ctx context.Context, email string) error {
	// Get user by email
	user, _, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal if email exists (security)
		return nil
	}

	// Generate reset token
	token, err := s.tokenGenerator.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Store the HASH of the token (same as session tokens). The raw token
	// is emailed; a DB/backup leak can't be replayed to reset a password.
	expiresAt := time.Now().Add(1 * time.Hour)
	tokenHash := s.tokenGenerator.HashToken(token)
	if err := s.repo.CreatePasswordResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("failed to store reset token: %w", err)
	}

	// Send email
	if s.emailService != nil {
		if err := s.emailService.SendPasswordResetEmail(user.Email, user.Name, token, s.baseURL); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

// ResetPassword resets a user's password using a reset token
func (s *service) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Validate password
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Look up by the token's hash — the DB stores hashes, not raw tokens.
	tokenHash := s.tokenGenerator.HashToken(token)
	userID, expiresAt, used, err := s.repo.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	// Check if already used
	if used {
		return fmt.Errorf("reset token already used")
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		return fmt.Errorf("reset token has expired")
	}

	// Hash new password
	passwordHash, err := s.tokenGenerator.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Get user and update password
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Update password in database
	if err := s.repo.UpdatePassword(ctx, userID, passwordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark token as used
	if err := s.repo.MarkPasswordResetTokenUsed(ctx, tokenHash); err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Invalidate all sessions (force re-login after password change)
	_ = s.InvalidateAllUserSessions(ctx, userID)
	_ = user

	return nil
}

// ChangePassword lets a logged-in user change their own password by proving
// the current one. The active session is intentionally NOT invalidated — the
// user stays logged in. Unlike ResetPassword (token-driven, forgot-password
// flow), this is self-service and requires the old password.
func (s *service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	// Validate new password (matches Register/ResetPassword).
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Resolve the user so we can fetch the stored hash via the
	// email-keyed getter (the only repository getter that returns the
	// password hash).
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	_, passwordHash, err := s.repo.GetUserByEmail(ctx, user.Email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Verify the current password before allowing the change.
	if !s.tokenGenerator.CheckPassword(oldPassword, passwordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash and store the new password.
	newHash, err := s.tokenGenerator.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// AdminSetPassword sets any user's password without proving the old one. This
// is the admin rotation/offboarding lever. All of the target's sessions are
// invalidated so they must re-login with the new password.
func (s *service) AdminSetPassword(ctx context.Context, userID, newPassword string) error {
	// Validate new password (matches Register/ResetPassword).
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Hash and store the new password.
	newHash, err := s.tokenGenerator.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Force the target to re-login with the new password.
	if err := s.InvalidateAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("failed to invalidate sessions: %w", err)
	}

	return nil
}

// InvalidateSession invalidates a single session
func (s *service) InvalidateSession(ctx context.Context, token string) error {
	tokenHash := s.tokenGenerator.HashToken(token)
	return s.repo.DeleteSession(ctx, tokenHash)
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (s *service) InvalidateAllUserSessions(ctx context.Context, userID string) error {
	return s.repo.DeleteAllUserSessions(ctx, userID)
}

// generateUUID creates a new cryptographically random UUID v4 string
func generateUUID() string {
	return uuid.New().String()
}
