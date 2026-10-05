package domain

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// ChannelEntity Interface (DDD Pattern: Programming to Interfaces)
// =============================================================================

// ChannelEntity defines the public interface for Channel aggregate root
// Factory methods return this interface to support Dependency Inversion Principle
type ChannelEntity interface {
	// Identity
	GetID() uuid.UUID
	GetName() string
	GetWorkspaceID() uuid.UUID

	// Predicate Evaluation (Fluent Interface)
	Is(predicate Predicate[ChannelEntity]) bool

	// Validation
	Validate() error

	// Business Operations
	Archive()
	Unarchive()
	UpdateDescription(description string)
	MakePrivate()
	MakePublic()

	// State Queries
	IsPrivateChannel() bool
	IsPublicChannel() bool
	IsArchivedState() bool
	GetDescription() *string
	BelongsToWorkspace(workspaceID uuid.UUID) bool

	// Timestamps
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetArchivedAt() *time.Time
}

// =============================================================================
// Channel Factory Methods (DDD Pattern: Aggregate Root Creation)
// =============================================================================

// =============================================================================
// Channel Interface Implementation (ChannelEntity)
// =============================================================================

// GetID returns the channel's ID
func (c *Channel) GetID() uuid.UUID {
	return c.ID
}

// GetName returns the channel's name
func (c *Channel) GetName() string {
	return c.Name
}

// GetWorkspaceID returns the channel's workspace ID
func (c *Channel) GetWorkspaceID() uuid.UUID {
	return c.WorkspaceID
}

// IsPrivateChannel returns whether the channel is private
func (c *Channel) IsPrivateChannel() bool {
	return c.IsPrivate
}

// IsPublicChannel returns whether the channel is public
func (c *Channel) IsPublicChannel() bool {
	return !c.IsPrivate
}

// IsArchivedState returns whether the channel is archived
func (c *Channel) IsArchivedState() bool {
	return c.ArchivedAt != nil
}

// GetDescription returns the channel's description
func (c *Channel) GetDescription() *string {
	return c.Description
}

// GetCreatedAt returns when the channel was created
func (c *Channel) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetUpdatedAt returns when the channel was last updated
func (c *Channel) GetUpdatedAt() time.Time {
	return c.UpdatedAt
}

// GetArchivedAt returns when the channel was archived
func (c *Channel) GetArchivedAt() *time.Time {
	return c.ArchivedAt
}

// =============================================================================
// Channel Predicates (DDD Specification Pattern)
// =============================================================================

// ChannelPredicates are reusable business rules for channels
// Usage: channel.Is(ChannelPredicates.IsPrivate)
var ChannelPredicates = struct {
	IsPrivate      Predicate[ChannelEntity]
	IsPublic       Predicate[ChannelEntity]
	IsArchived     Predicate[ChannelEntity]
	IsActive       Predicate[ChannelEntity]
	HasDescription Predicate[ChannelEntity]
}{
	IsPrivate: func(c ChannelEntity) bool {
		return c.IsPrivateChannel()
	},
	IsPublic: func(c ChannelEntity) bool {
		return c.IsPublicChannel()
	},
	IsArchived: func(c ChannelEntity) bool {
		return c.IsArchivedState()
	},
	IsActive: func(c ChannelEntity) bool {
		return !c.IsArchivedState()
	},
	HasDescription: func(c ChannelEntity) bool {
		desc := c.GetDescription()
		return desc != nil && *desc != ""
	},
}

// Is evaluates a predicate against this channel
// This is the fluent interface: channel.Is(ChannelPredicates.IsPrivate)
func (c *Channel) Is(predicate Predicate[ChannelEntity]) bool {
	return predicate(c)
}

// =============================================================================
// Channel Validation (using Specifications)
// =============================================================================

// channelSpecifications are internal validation rules using concrete *Channel type
var channelSpecifications = struct {
	NameRequired   Specification[*Channel]
	NameNotTooLong Specification[*Channel]
	NameValidChars Specification[*Channel]
}{
	NameRequired: Specification[*Channel]{
		Name: "NameRequired",
		Predicate: func(c *Channel) bool {
			return c.Name != ""
		},
		Error: NewFieldValidationError("name", "channel name is required"),
	},
	NameNotTooLong: Specification[*Channel]{
		Name: "NameNotTooLong",
		Predicate: func(c *Channel) bool {
			return len(c.Name) <= 80
		},
		Error: NewFieldValidationError("name", "channel name must be 80 characters or less"),
	},
	NameValidChars: Specification[*Channel]{
		Name: "NameValidChars",
		Predicate: func(c *Channel) bool {
			// Slack-like: lowercase, numbers, hyphens, underscores
			for _, ch := range c.Name {
				if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
					return false
				}
			}
			return true
		},
		Error: NewFieldValidationError("name", "channel name must contain only lowercase letters, numbers, hyphens, and underscores"),
	},
}

// Validate validates the channel using all specifications
func (c *Channel) Validate() error {
	return ValidateAll(c,
		channelSpecifications.NameRequired,
		channelSpecifications.NameNotTooLong,
		channelSpecifications.NameValidChars,
	)
}

// =============================================================================
// Channel Business Methods (Domain Logic)
// =============================================================================

// Archive archives the channel (soft-delete)
func (c *Channel) Archive() {
	c.Archivable.Archive()
	c.UpdatedAt = time.Now()
}

// Unarchive restores an archived channel
func (c *Channel) Unarchive() {
	c.Archivable.Unarchive()
	c.UpdatedAt = time.Now()
}

// UpdateDescription changes the channel's description
func (c *Channel) UpdateDescription(description string) {
	c.Description = &description
	c.UpdatedAt = time.Now()
}

// MakePrivate converts the channel to private
func (c *Channel) MakePrivate() {
	c.IsPrivate = true
	c.UpdatedAt = time.Now()
}

// MakePublic converts the channel to public
func (c *Channel) MakePublic() {
	c.IsPrivate = false
	c.UpdatedAt = time.Now()
}
