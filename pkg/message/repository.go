package message

import (
	"context"

	"memdoor/pkg/shared"
)

// PaginatedMessages wraps messages with cursor pagination info
type PaginatedMessages struct {
	Messages []*Message
	HasMore  bool
}

// Repository defines the interface for persisting messages
// Messages are immutable - only Save (insert), no Update
type Repository interface {
	// Save persists a new message (insert only, returns assigned ID)
	// INVARIANT: Author must be in channel (checked before calling)
	// INVARIANT: Parent must exist if ParentID != nil (checked before calling)
	Save(ctx context.Context, msg *Message) (MessageID, error)

	// FindByID retrieves a message by ID
	FindByID(ctx context.Context, id MessageID) (*Message, error)

	// ListByChannel retrieves messages in a channel with cursor-based pagination.
	// Returns messages ordered chronologically (oldest first within page).
	// Use CursorPage.BeforeID=0 for the latest page.
	ListByChannel(ctx context.Context, channelID string, page shared.CursorPage) (*PaginatedMessages, error)

	// ListReplies retrieves all replies to a message (thread)
	ListReplies(ctx context.Context, parentID MessageID) ([]*Message, error)

	// ListByAuthor retrieves messages by a specific author
	ListByAuthor(ctx context.Context, authorID shared.ActorID, limit int) ([]*Message, error)

	// ListSince retrieves messages in a channel since a given message ID
	// Used for real-time updates (polling)
	ListSince(ctx context.Context, channelID string, sinceID MessageID) ([]*Message, error)

	// MarkAsRead marks a message as read (updates is_read to true)
	MarkAsRead(ctx context.Context, id MessageID) error

	// MarkChannelAsRead marks all messages in a channel as read
	MarkChannelAsRead(ctx context.Context, channelID string) error

	// UpdateContent updates the content and updated_at timestamp of a message
	UpdateContent(ctx context.Context, msg *Message) error

	// CountUnreadMentionsByChannel returns a map of channel_id → count of unread @mentions
	// Only counts messages where:
	// - is_read = false
	// - author_id != userID (don't count own messages)
	// - content_mentions JSON contains userID
	CountUnreadMentionsByChannel(ctx context.Context, userID shared.ActorID) (map[string]int, error)

	// ListUnreadMentions retrieves all unread messages that mention the user
	// Returns messages across ALL channels where:
	// - is_read = false
	// - author_id != userID (don't return own messages)
	// - content_mentions JSON contains userID
	// Ordered by created_at DESC (most recent first)
	ListUnreadMentions(ctx context.Context, userID shared.ActorID, limit int) ([]*Message, error)

	// ListUnreadThreadReplies retrieves unread replies to the user's messages
	// Returns thread replies where:
	// - The parent message was authored by userID
	// - The reply is from someone else (author_id != userID)
	// - is_read = false
	// Ordered by created_at DESC (most recent first)
	ListUnreadThreadReplies(ctx context.Context, userID shared.ActorID, limit int) ([]*Message, error)
}

// UserWithID is a duck-typed interface for extracting ActorID from user objects
// This avoids circular dependency with pkg/auth while providing type safety
type UserWithID interface {
	GetID() string       // Returns the ActorID as a string
	GetUsername() string // Returns the username for display
}

// AuthRepository defines the minimal interface needed for resolving mentions to ActorIDs
// This is a duck-typed interface to avoid circular dependency with pkg/auth
type AuthRepository interface {
	// GetUserByEmail looks up a user by their email address
	// Returns (user, passwordHash, error)
	// Returns nil user if not found
	GetUserByEmail(ctx context.Context, email string) (UserWithID, string, error)
	// GetUserByUsername looks up a user by their username for @username mentions
	// Returns nil user if not found
	GetUserByUsername(ctx context.Context, username string) (UserWithID, error)
}
