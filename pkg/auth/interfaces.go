package auth

import (
	"context"
	"time"
)

// Service defines the authentication service interface
// Pattern: Interface-based dependency injection
// This allows different implementations (SQLite, PostgreSQL, etc.)
type Service interface {
	// User management
	Register(ctx context.Context, email, username, password, name string) (*User, error)
	Login(ctx context.Context, email, password string) (*User, string, error)
	VerifyToken(ctx context.Context, token string) (*User, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)

	// Email verification
	SendVerificationEmail(ctx context.Context, userID, email, name string) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, userID string) error

	// Password management
	SendPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	AdminSetPassword(ctx context.Context, userID, newPassword string) error

	// Session management
	InvalidateSession(ctx context.Context, token string) error
	InvalidateAllUserSessions(ctx context.Context, userID string) error
}

// Repository defines the auth data access interface
// Pattern: Repository pattern for data persistence abstraction
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *User, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*User, string, error) // returns user + password hash
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	MarkEmailVerified(ctx context.Context, userID string) error
	SetUserEmail(ctx context.Context, userID, email string) error

	// Session operations
	CreateSession(ctx context.Context, session *Session, tokenHash string) error
	GetSessionByToken(ctx context.Context, tokenHash string) (*Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
	UpdateLastLogin(ctx context.Context, userID string) error

	// Verification token operations
	CreateVerificationToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	GetVerificationToken(ctx context.Context, token string) (userID string, expiresAt time.Time, err error)
	DeleteVerificationToken(ctx context.Context, token string) error
	DeleteUserVerificationTokens(ctx context.Context, userID string) error

	// Password reset token operations
	CreatePasswordResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, token string) (userID string, expiresAt time.Time, used bool, err error)
	MarkPasswordResetTokenUsed(ctx context.Context, token string) error
	DeletePasswordResetToken(ctx context.Context, token string) error

	// Password update
	UpdatePassword(ctx context.Context, userID string, passwordHash string) error

	// User existence check
	EmailExists(ctx context.Context, email string) (bool, error)
}

// EmailService defines the email sending interface
// Pattern: Interface for external service abstraction
type EmailService interface {
	SendVerificationEmail(to, name, token, baseURL string) error
	SendPasswordResetEmail(to, name, token, baseURL string) error
}

// TokenGenerator defines the token generation interface
// Pattern: Interface for security operations
type TokenGenerator interface {
	GenerateToken() (string, error)
	HashToken(token string) string
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) bool
}
