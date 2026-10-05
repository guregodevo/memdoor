package auth

import (
	"time"

	"memdoor/pkg/domain"
)

// User represents an authenticated user in the auth domain
// Uses embedded BaseUser to avoid field duplication
type User struct {
	domain.BaseUser
	WorkspaceID string          `json:"workspace_id"`
	Role        domain.UserRole `json:"role"` // User role (admin/user)
}

// Session represents an authentication session
type Session struct {
	Token       string
	UserID      string
	WorkspaceID string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
