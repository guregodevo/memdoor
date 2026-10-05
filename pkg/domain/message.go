package domain

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// MessageEntity Interface (DDD Pattern: Programming to Interfaces)
// =============================================================================

// MessageEntity defines the public interface for Message aggregate root
// Factory methods return this interface to support Dependency Inversion Principle
type MessageEntity interface {
	// Identity
	GetID() int64
	GetChannelID() uuid.UUID
	GetWorkspaceID() uuid.UUID

	// Predicate Evaluation (Fluent Interface)
	Is(predicate Predicate[MessageEntity]) bool

	// Validation
	Validate() error

	// Business Operations
	Delete()
	Restore()
	UpdateContent(content string)

	// State Queries
	IsSentByUser() bool
	IsSentByBuddy() bool
	IsDeletedState() bool
	IsThreaded() bool
	GetContent() string
	GetContentType() string
	GetUserID() *uuid.UUID
	GetBuddyID() *uuid.UUID
	GetThreadID() *int64

	// Timestamps
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetDeletedAt() *time.Time
}

// =============================================================================
// Message Factory Methods (DDD Pattern: Aggregate Root Creation)
// =============================================================================

// =============================================================================
// Message Interface Implementation (MessageEntity)
// =============================================================================

// =============================================================================
// Message Predicates (DDD Specification Pattern)
// =============================================================================

// MessagePredicates are reusable business rules for messages
// Usage: message.Is(MessagePredicates.IsByUser)
var MessagePredicates = struct {
	IsByUser      Predicate[MessageEntity]
	IsByBuddy     Predicate[MessageEntity]
	IsDeleted     Predicate[MessageEntity]
	IsActive      Predicate[MessageEntity]
	IsThreaded    Predicate[MessageEntity]
	IsTopLevel    Predicate[MessageEntity]
	IsTextMessage Predicate[MessageEntity]
	IsCodeMessage Predicate[MessageEntity]
}{
	IsByUser: func(m MessageEntity) bool {
		return m.IsSentByUser()
	},
	IsByBuddy: func(m MessageEntity) bool {
		return m.IsSentByBuddy()
	},
	IsDeleted: func(m MessageEntity) bool {
		return m.IsDeletedState()
	},
	IsActive: func(m MessageEntity) bool {
		return !m.IsDeletedState()
	},
	IsThreaded: func(m MessageEntity) bool {
		return m.IsThreaded()
	},
	IsTopLevel: func(m MessageEntity) bool {
		return !m.IsThreaded()
	},
	IsTextMessage: func(m MessageEntity) bool {
		return m.GetContentType() == "text"
	},
	IsCodeMessage: func(m MessageEntity) bool {
		return m.GetContentType() == "code"
	},
}

// =============================================================================
// Message Validation (using Specifications)
// =============================================================================

// =============================================================================
// Message Business Methods (Domain Logic)
// =============================================================================
