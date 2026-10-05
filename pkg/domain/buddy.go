package domain

import (
	"memdoor/pkg/sandbox"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// BuddyEntity Interface (DDD Pattern: Programming to Interfaces)
// =============================================================================

// BuddyEntity defines the public interface for Buddy aggregate root
// Factory methods return this interface to support Dependency Inversion Principle
type BuddyEntity interface {
	// Identity
	GetID() uuid.UUID
	GetName() string
	GetWorkspaceID() uuid.UUID

	// Predicate Evaluation (Fluent Interface)
	Is(predicate Predicate[BuddyEntity]) bool

	// Validation
	Validate() error

	// Business Operations
	Activate()
	Deactivate()
	UpdateSampling(temperature float64, maxTokens int) error
	SetSandboxScope(scope sandbox.SandboxScope) error

	// State Queries
	IsActiveState() bool
	IsRemoteAgent() bool
	IsLearningAgent() bool
	GetSandboxScope() sandbox.SandboxScope
	GetExecutionType() string

	// Timestamps
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// =============================================================================
// Buddy Factory Methods (DDD Pattern: Aggregate Root Creation)
// =============================================================================

// =============================================================================
// Buddy Interface Implementation (BuddyEntity)
// =============================================================================

// GetID returns the buddy's ID
func (b *Buddy) GetID() uuid.UUID {
	return b.ID
}

// GetName returns the buddy's name
func (b *Buddy) GetName() string {
	return b.Name
}

// GetWorkspaceID returns the buddy's workspace ID
func (b *Buddy) GetWorkspaceID() uuid.UUID {
	return b.WorkspaceID
}

// IsActiveState returns whether the buddy is active
func (b *Buddy) IsActiveState() bool {
	return b.IsActive
}

// IsRemoteAgent returns whether the buddy is a remote agent
func (b *Buddy) IsRemoteAgent() bool {
	return b.ExecutionType == "remote"
}

// IsLearningAgent returns whether the buddy has the learning loop enabled
func (b *Buddy) IsLearningAgent() bool {
	return b.LearningEnabled
}

// GetSandboxScope returns the buddy's sandbox scope
func (b *Buddy) GetSandboxScope() sandbox.SandboxScope {
	return b.SandboxScope
}

// GetExecutionType returns the buddy's execution type
func (b *Buddy) GetExecutionType() string {
	return b.ExecutionType
}

// GetCreatedAt returns when the buddy was created
func (b *Buddy) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// GetUpdatedAt returns when the buddy was last updated
func (b *Buddy) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}

// =============================================================================
// Buddy Predicates (DDD Specification Pattern)
// =============================================================================

// BuddyPredicates are reusable business rules for buddies
// Usage: buddy.Is(BuddyPredicates.IsActive)
var BuddyPredicates = struct {
	IsActive          Predicate[BuddyEntity]
	IsInactive        Predicate[BuddyEntity]
	IsLocal           Predicate[BuddyEntity]
	IsRemote          Predicate[BuddyEntity]
	HasUserScope      Predicate[BuddyEntity]
	HasChannelScope   Predicate[BuddyEntity]
	HasWorkspaceScope Predicate[BuddyEntity]
	IsLearning        Predicate[BuddyEntity]
}{
	IsActive: func(b BuddyEntity) bool {
		return b.IsActiveState()
	},
	IsInactive: func(b BuddyEntity) bool {
		return !b.IsActiveState()
	},
	IsLocal: func(b BuddyEntity) bool {
		execType := b.GetExecutionType()
		return execType == "local" || execType == ""
	},
	IsRemote: func(b BuddyEntity) bool {
		return b.GetExecutionType() == "remote"
	},
	HasUserScope: func(b BuddyEntity) bool {
		return b.GetSandboxScope() == sandbox.ScopeUser
	},
	HasChannelScope: func(b BuddyEntity) bool {
		return b.GetSandboxScope() == sandbox.ScopeChannel
	},
	HasWorkspaceScope: func(b BuddyEntity) bool {
		return b.GetSandboxScope() == sandbox.ScopeWorkspace
	},
	IsLearning: func(b BuddyEntity) bool {
		return b.IsLearningAgent()
	},
}

// Is evaluates a predicate against this buddy
// This is the fluent interface: buddy.Is(BuddyPredicates.IsActive)
func (b *Buddy) Is(predicate Predicate[BuddyEntity]) bool {
	return predicate(b)
}

// =============================================================================
// Buddy Validation (using Specifications)
// =============================================================================

// buddySpecifications are internal validation rules using concrete *Buddy type
// These need access to internal fields (RemoteConfig, etc.)
var buddySpecifications = struct {
	NameRequired        Specification[*Buddy]
	NameNotTooLong      Specification[*Buddy]
	AvatarEmojiRequired Specification[*Buddy]
	ValidExecutionType  Specification[*Buddy]
	ValidSandboxScope   Specification[*Buddy]
	RemoteConfigValid   Specification[*Buddy]
}{
	NameRequired: Specification[*Buddy]{
		Name: "NameRequired",
		Predicate: func(b *Buddy) bool {
			return b.Name != ""
		},
		Error: NewFieldValidationError("name", "name is required"),
	},
	NameNotTooLong: Specification[*Buddy]{
		Name: "NameNotTooLong",
		Predicate: func(b *Buddy) bool {
			return len(b.Name) <= 100
		},
		Error: NewFieldValidationError("name", "name must be 100 characters or less"),
	},
	AvatarEmojiRequired: Specification[*Buddy]{
		Name: "AvatarEmojiRequired",
		Predicate: func(b *Buddy) bool {
			return b.AvatarEmoji != ""
		},
		Error: NewFieldValidationError("avatar_emoji", "avatar emoji is required"),
	},
	ValidExecutionType: Specification[*Buddy]{
		Name: "ValidExecutionType",
		Predicate: func(b *Buddy) bool {
			return b.ExecutionType == "local" || b.ExecutionType == "remote" || b.ExecutionType == ""
		},
		Error: NewFieldValidationError("execution_type", "execution type must be 'local' or 'remote'"),
	},
	ValidSandboxScope: Specification[*Buddy]{
		Name: "ValidSandboxScope",
		Predicate: func(b *Buddy) bool {
			return b.SandboxScope.IsValid()
		},
		Error: NewFieldValidationError("sandbox_scope", "invalid sandbox scope"),
	},
	RemoteConfigValid: Specification[*Buddy]{
		Name: "RemoteConfigValid",
		Predicate: func(b *Buddy) bool {
			if b.ExecutionType != "remote" {
				return true // Not applicable for local agents
			}
			if b.RemoteConfig == nil {
				return false
			}
			return b.RemoteConfig.Endpoint != "" && b.RemoteConfig.Model != ""
		},
		Error: NewFieldValidationError("remote_config", "remote config with endpoint and model is required for remote agents"),
	},
}

// Validate validates the buddy using all specifications
func (b *Buddy) Validate() error {
	return ValidateAll(b,
		buddySpecifications.NameRequired,
		buddySpecifications.NameNotTooLong,
		buddySpecifications.AvatarEmojiRequired,
		buddySpecifications.ValidExecutionType,
		buddySpecifications.ValidSandboxScope,
		buddySpecifications.RemoteConfigValid,
	)
}

// =============================================================================
// Buddy Business Methods (Domain Logic)
// =============================================================================

// Activate activates the buddy (makes it available for use)
func (b *Buddy) Activate() {
	b.IsActive = true
	b.UpdatedAt = time.Now()
}

// Deactivate deactivates the buddy (makes it unavailable for use)
func (b *Buddy) Deactivate() {
	b.IsActive = false
	b.UpdatedAt = time.Now()
}

// UpdateSampling adjusts the buddy's sampling parameters. Model
// selection isn't here — workspace byok drives it.
func (b *Buddy) UpdateSampling(temperature float64, maxTokens int) error {
	if temperature < 0 || temperature > 2 {
		return NewFieldValidationError("temperature", "temperature must be between 0 and 2")
	}
	if maxTokens <= 0 {
		return NewFieldValidationError("max_tokens", "max tokens must be positive")
	}

	b.Temperature = temperature
	b.MaxTokens = maxTokens
	b.UpdatedAt = time.Now()
	return nil
}

// SetSandboxScope changes the sandbox scope with validation
func (b *Buddy) SetSandboxScope(scope sandbox.SandboxScope) error {
	if !scope.IsValid() {
		return NewFieldValidationError("sandbox_scope", "invalid sandbox scope")
	}
	b.SandboxScope = scope
	b.UpdatedAt = time.Now()
	return nil
}
