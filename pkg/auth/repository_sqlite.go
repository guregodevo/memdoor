package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// sqliteRepository implements Repository interface for SQLite
type sqliteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite auth repository
func NewSQLiteRepository(db *sql.DB) Repository {
	return &sqliteRepository{db: db}
}

// CreateUser creates a new user in the database
func (r *sqliteRepository) CreateUser(ctx context.Context, user *User, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, workspace_id, email, username, password_hash, name, email_verified, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.WorkspaceID, user.Email, user.Username, passwordHash, user.Name, user.EmailVerified, user.Role, user.CreatedAt, user.UpdatedAt)
	return err
}

// GetUserByEmail retrieves a user by email
func (r *sqliteRepository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	var user User
	var passwordHash string
	var avatarURL sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, email, username, password_hash, name, avatar_url, email_verified, role, created_at, updated_at
		FROM users
		WHERE email = ?
	`, email).Scan(
		&user.ID,
		&user.WorkspaceID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.Name,
		&avatarURL,
		&user.EmailVerified,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, "", err
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	return &user, passwordHash, nil
}

// GetUserByID retrieves a user by ID
func (r *sqliteRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var user User
	var avatarURL sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, email, username, name, avatar_url, email_verified, role, created_at, updated_at
		FROM users
		WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.WorkspaceID,
		&user.Email,
		&user.Username,
		&user.Name,
		&avatarURL,
		&user.EmailVerified,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (r *sqliteRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	var avatarURL sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, email, username, name, avatar_url, email_verified, role, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username).Scan(
		&user.ID,
		&user.WorkspaceID,
		&user.Email,
		&user.Username,
		&user.Name,
		&avatarURL,
		&user.EmailVerified,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	return &user, nil
}

// UpdateUser updates a user in the database
func (r *sqliteRepository) UpdateUser(ctx context.Context, user *User) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET name = ?, avatar_url = ?, email_verified = ?, role = ?, updated_at = ?
		WHERE id = ?
	`, user.Name, user.AvatarURL, user.EmailVerified, user.Role, time.Now(), user.ID)
	return err
}

// SetUserEmail moves a user to another email address. Sessions, the
// username and everything the user owns stay as they are.
func (r *sqliteRepository) SetUserEmail(ctx context.Context, userID, email string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = ?, updated_at = ?
		WHERE id = ?
	`, email, time.Now(), userID)
	return err
}

// MarkEmailVerified marks a user's email as verified
func (r *sqliteRepository) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email_verified = 1, updated_at = ?
		WHERE id = ?
	`, time.Now(), userID)
	return err
}

// CreateSession creates a new session
func (r *sqliteRepository) CreateSession(ctx context.Context, session *Session, tokenHash string) error {
	sessionID := fmt.Sprintf("session:%d", time.Now().UnixNano())
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (id, workspace_id, user_id, session_type, token_hash, expires_at, created_at, last_activity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, session.WorkspaceID, session.UserID, "auth", tokenHash, session.ExpiresAt, session.CreatedAt, session.CreatedAt)
	return err
}

// GetSessionByToken retrieves a session by token hash
func (r *sqliteRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*Session, error) {
	var session Session
	var workspaceID string

	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, workspace_id, expires_at, created_at
		FROM sessions
		WHERE token_hash = ? AND session_type = 'auth'
	`, tokenHash).Scan(&session.UserID, &workspaceID, &session.ExpiresAt, &session.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &session, nil
}

// DeleteSession deletes a session by token hash
func (r *sqliteRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE token_hash = ? AND session_type = 'auth'
	`, tokenHash)
	return err
}

// DeleteAllUserSessions deletes all sessions for a user
func (r *sqliteRepository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE user_id = ? AND session_type = 'auth'
	`, userID)
	return err
}

// UpdateLastLogin updates the last login time for a user
func (r *sqliteRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET last_login_at = ?
		WHERE id = ?
	`, time.Now(), userID)
	return err
}

// CreateVerificationToken creates a new email verification token
func (r *sqliteRepository) CreateVerificationToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (token, user_id, expires_at)
		VALUES (?, ?, ?)
	`, token, userID, expiresAt)
	return err
}

// GetVerificationToken retrieves a verification token
func (r *sqliteRepository) GetVerificationToken(ctx context.Context, token string) (userID string, expiresAt time.Time, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT user_id, expires_at
		FROM email_verification_tokens
		WHERE token = ?
	`, token).Scan(&userID, &expiresAt)
	return
}

// DeleteVerificationToken deletes a verification token
func (r *sqliteRepository) DeleteVerificationToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_verification_tokens
		WHERE token = ?
	`, token)
	return err
}

// DeleteUserVerificationTokens deletes all verification tokens for a user
func (r *sqliteRepository) DeleteUserVerificationTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_verification_tokens
		WHERE user_id = ?
	`, userID)
	return err
}

// CreatePasswordResetToken creates a new password reset token
func (r *sqliteRepository) CreatePasswordResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (token, user_id, expires_at)
		VALUES (?, ?, ?)
	`, token, userID, expiresAt)
	return err
}

// GetPasswordResetToken retrieves a password reset token
func (r *sqliteRepository) GetPasswordResetToken(ctx context.Context, token string) (userID string, expiresAt time.Time, used bool, err error) {
	var usedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, `
		SELECT user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token = ?
	`, token).Scan(&userID, &expiresAt, &usedAt)

	if err != nil {
		return
	}

	used = usedAt.Valid
	return
}

// MarkPasswordResetTokenUsed marks a password reset token as used
func (r *sqliteRepository) MarkPasswordResetTokenUsed(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE password_reset_tokens
		SET used_at = ?
		WHERE token = ?
	`, time.Now(), token)
	return err
}

// DeletePasswordResetToken deletes a password reset token
func (r *sqliteRepository) DeletePasswordResetToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM password_reset_tokens
		WHERE token = ?
	`, token)
	return err
}

// UpdatePassword updates a user's password hash
func (r *sqliteRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, updated_at = ?
		WHERE id = ?
	`, passwordHash, time.Now(), userID)
	return err
}

// EmailExists checks if an email already exists
func (r *sqliteRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE email = ?
	`, email).Scan(&count)
	return count > 0, err
}
