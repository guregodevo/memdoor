package reaction

import (
	"context"

	"memdoor/pkg/message"
)

// Repository defines persistence operations for reactions
type Repository interface {
	// Add creates a new reaction
	// Returns error if reaction already exists (user + emoji + message combination)
	Add(ctx context.Context, reaction *Reaction) (ReactionID, error)

	// Remove deletes a reaction by message ID, user ID, and emoji
	// Returns nil if reaction doesn't exist (idempotent)
	Remove(ctx context.Context, messageID message.MessageID, userID string, emoji string) error

	// ListByMessage retrieves all reactions for a specific message
	// Returns empty slice if no reactions found
	ListByMessage(ctx context.Context, messageID message.MessageID) ([]*Reaction, error)

	// ListByMessages retrieves reactions for multiple messages
	// Returns map of message ID -> reactions
	// Useful for efficiently loading reactions for a list of messages
	ListByMessages(ctx context.Context, messageIDs []message.MessageID) (map[message.MessageID][]*Reaction, error)

	// GetByID retrieves a single reaction by its ID
	// Returns nil if not found
	GetByID(ctx context.Context, id ReactionID) (*Reaction, error)
}
