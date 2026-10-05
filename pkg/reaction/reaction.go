package reaction

import (
	"fmt"
	"time"

	"memdoor/pkg/message"
	"memdoor/pkg/shared"
)

// Reaction is an immutable aggregate representing a user's emoji reaction to a message
//
// INVARIANTS:
// 1. User can only have one reaction with the same emoji per message (enforced by database UNIQUE constraint)
// 2. Message must exist (enforced by foreign key)
// 3. Emoji must be non-empty
// 4. Reaction is immutable once created (no updates, only add/remove)
type Reaction struct {
	// Aggregate identity
	ID ReactionID // Auto-increment ID

	// Relationships
	MessageID message.MessageID // Which message this reaction is on
	UserID    shared.ActorID    // Who reacted (human:<uuid> or agent:<uuid>)

	// Content
	Emoji string // The emoji used for the reaction (e.g., "👍", "❤️", "😂")

	// Metadata (immutable)
	CreatedAt time.Time // When the reaction was added
}

// NewReaction creates a new reaction
// ID is not set (assigned by repository on insert)
// CreatedAt is set to now
func NewReaction(messageID message.MessageID, userID shared.ActorID, emoji string) (*Reaction, error) {
	if messageID == 0 {
		return nil, fmt.Errorf("message ID cannot be zero")
	}

	if err := userID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	if emoji == "" {
		return nil, fmt.Errorf("emoji cannot be empty")
	}

	// Basic emoji validation - check it's not just whitespace
	if len(emoji) > 100 {
		return nil, fmt.Errorf("emoji is too long (max 100 characters)")
	}

	return &Reaction{
		ID:        0, // Assigned by repository
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
		CreatedAt: time.Now(),
	}, nil
}

// Validate checks if the reaction is valid
func (r *Reaction) Validate() error {
	if r.MessageID == 0 {
		return fmt.Errorf("message ID cannot be zero")
	}

	if err := r.UserID.Validate(); err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	if r.Emoji == "" {
		return fmt.Errorf("emoji cannot be empty")
	}

	return nil
}
