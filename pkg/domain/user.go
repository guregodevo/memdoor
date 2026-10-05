package domain

import (
	"time"
)

// =============================================================================
// UserEntity Interface (DDD Pattern: Programming to Interfaces)
// =============================================================================

// UserEntity defines the public interface for User aggregate root
// Factory methods return this interface to support Dependency Inversion Principle
type UserEntity interface {
	// Identity
	GetID() string
	GetEmail() string
	GetUsername() string
	GetName() string
	GetWorkspaceID() string
	GetRole() UserRole

	// Predicate Evaluation (Fluent Interface)
	Is(predicate Predicate[UserEntity]) bool

	// Validation
	Validate() error

	// Business Operations
	VerifyEmail()
	UnverifyEmail()
	UpdateProfile(name string, avatarURL string)
	RecordLogin()

	// State Queries
	IsEmailVerified() bool
	GetAvatarURL() string
	GetLastLoginAt() *time.Time

	// Timestamps
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// =============================================================================
// User Factory Methods (DDD Pattern: Aggregate Root Creation)
// =============================================================================

// =============================================================================
// User Interface Implementation (UserEntity)
// =============================================================================

// GetID returns the user's ID
func (u *User) GetID() string {
	return u.ID
}

// GetEmail returns the user's email
func (u *User) GetEmail() string {
	return u.Email
}

// GetUsername returns the user's username
func (u *User) GetUsername() string {
	return u.Username
}

// GetName returns the user's name
func (u *User) GetName() string {
	return u.Name
}

// GetWorkspaceID returns the user's workspace ID
func (u *User) GetWorkspaceID() string {
	return u.WorkspaceID
}

// GetRole returns the user's role
func (u *User) GetRole() UserRole {
	return u.Role
}

// IsEmailVerified returns whether the user's email is verified
func (u *User) IsEmailVerified() bool {
	return u.EmailVerified
}

// GetAvatarURL returns the user's avatar URL
func (u *User) GetAvatarURL() string {
	return u.AvatarURL
}

// GetLastLoginAt returns when the user last logged in
func (u *User) GetLastLoginAt() *time.Time {
	return u.LastLoginAt
}

// GetCreatedAt returns when the user was created
func (u *User) GetCreatedAt() time.Time {
	return u.CreatedAt
}

// GetUpdatedAt returns when the user was last updated
func (u *User) GetUpdatedAt() time.Time {
	return u.UpdatedAt
}

// =============================================================================
// User Predicates (DDD Specification Pattern)
// =============================================================================

// UserPredicates are reusable business rules for users
// Usage: user.Is(UserPredicates.IsVerified)
var UserPredicates = struct {
	IsVerified    Predicate[UserEntity]
	IsUnverified  Predicate[UserEntity]
	HasAvatar     Predicate[UserEntity]
	HasLoggedIn   Predicate[UserEntity]
	NeverLoggedIn Predicate[UserEntity]
	IsAdmin       Predicate[UserEntity]
	IsRegularUser Predicate[UserEntity]
}{
	IsVerified: func(u UserEntity) bool {
		return u.IsEmailVerified()
	},
	IsUnverified: func(u UserEntity) bool {
		return !u.IsEmailVerified()
	},
	HasAvatar: func(u UserEntity) bool {
		return u.GetAvatarURL() != ""
	},
	HasLoggedIn: func(u UserEntity) bool {
		return u.GetLastLoginAt() != nil
	},
	NeverLoggedIn: func(u UserEntity) bool {
		return u.GetLastLoginAt() == nil
	},
	IsAdmin: func(u UserEntity) bool {
		return u.GetRole() == UserRoleAdmin
	},
	IsRegularUser: func(u UserEntity) bool {
		return u.GetRole() == UserRoleUser
	},
}

// Is evaluates a predicate against this user
// This is the fluent interface: user.Is(UserPredicates.IsVerified)
func (u *User) Is(predicate Predicate[UserEntity]) bool {
	return predicate(u)
}

// =============================================================================
// User Validation (using Specifications)
// =============================================================================

// userSpecifications are internal validation rules using concrete *User type
var userSpecifications = struct {
	EmailRequired    Specification[*User]
	EmailValid       Specification[*User]
	UsernameRequired Specification[*User]
	UsernameValid    Specification[*User]
	NameRequired     Specification[*User]
	NameNotTooLong   Specification[*User]
	WorkspaceIDValid Specification[*User]
}{
	EmailRequired: Specification[*User]{
		Name: "EmailRequired",
		Predicate: func(u *User) bool {
			return u.Email != ""
		},
		Error: NewFieldValidationError("email", "email is required"),
	},
	EmailValid: Specification[*User]{
		Name: "EmailValid",
		Predicate: func(u *User) bool {
			// Simple email validation: contains @ and has characters before and after
			email := u.Email
			if len(email) < 3 {
				return false
			}
			atIndex := -1
			for i, ch := range email {
				if ch == '@' {
					atIndex = i
					break
				}
			}
			return atIndex > 0 && atIndex < len(email)-1
		},
		Error: NewFieldValidationError("email", "email must be a valid email address"),
	},
	UsernameRequired: Specification[*User]{
		Name: "UsernameRequired",
		Predicate: func(u *User) bool {
			return u.Username != ""
		},
		Error: NewFieldValidationError("username", "username is required"),
	},
	UsernameValid: Specification[*User]{
		Name: "UsernameValid",
		Predicate: func(u *User) bool {
			username := u.Username
			// Username must be 3-30 characters
			if len(username) < 3 || len(username) > 30 {
				return false
			}
			// Username must be alphanumeric, hyphens, underscores (no spaces)
			for _, ch := range username {
				isAlphanumeric := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
				isAllowedSymbol := ch == '-' || ch == '_'
				if !isAlphanumeric && !isAllowedSymbol {
					return false
				}
			}
			return true
		},
		Error: NewFieldValidationError("username", "username must be 3-30 characters, alphanumeric with hyphens or underscores"),
	},
	NameRequired: Specification[*User]{
		Name: "NameRequired",
		Predicate: func(u *User) bool {
			return u.Name != ""
		},
		Error: NewFieldValidationError("name", "name is required"),
	},
	NameNotTooLong: Specification[*User]{
		Name: "NameNotTooLong",
		Predicate: func(u *User) bool {
			return len(u.Name) <= 100
		},
		Error: NewFieldValidationError("name", "name must be 100 characters or less"),
	},
	WorkspaceIDValid: Specification[*User]{
		Name: "WorkspaceIDValid",
		Predicate: func(u *User) bool {
			return u.WorkspaceID != ""
		},
		Error: NewFieldValidationError("workspace_id", "workspace ID is required"),
	},
}

// Validate validates the user using all specifications
func (u *User) Validate() error {
	return ValidateAll(u,
		userSpecifications.EmailRequired,
		userSpecifications.EmailValid,
		userSpecifications.UsernameRequired,
		userSpecifications.UsernameValid,
		userSpecifications.NameRequired,
		userSpecifications.NameNotTooLong,
		userSpecifications.WorkspaceIDValid,
	)
}

// =============================================================================
// User Business Methods (Domain Logic)
// =============================================================================

// VerifyEmail marks the user's email as verified
func (u *User) VerifyEmail() {
	u.EmailVerified = true
	u.UpdatedAt = time.Now()
}

// UnverifyEmail marks the user's email as unverified
func (u *User) UnverifyEmail() {
	u.EmailVerified = false
	u.UpdatedAt = time.Now()
}

// UpdateProfile updates the user's name and avatar
func (u *User) UpdateProfile(name string, avatarURL string) {
	u.Name = name
	u.AvatarURL = avatarURL
	u.UpdatedAt = time.Now()
}

// RecordLogin updates the user's last login timestamp
func (u *User) RecordLogin() {
	now := time.Now()
	u.LastLoginAt = &now
	u.UpdatedAt = now
}
