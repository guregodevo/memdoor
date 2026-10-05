package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"
)

// reactionRepository implements reaction.Repository for SQLite
type reactionRepository struct {
	db *sql.DB
}

// NewReactionRepository creates a new SQLite reaction repository
func NewReactionRepository(db *sql.DB) reaction.Repository {
	return &reactionRepository{db: db}
}

// Add creates a new reaction
func (r *reactionRepository) Add(ctx context.Context, react *reaction.Reaction) (reaction.ReactionID, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO reactions (message_id, user_id, emoji, created_at)
		VALUES (?, ?, ?, ?)
	`,
		int64(react.MessageID),
		react.UserID.String(),
		react.Emoji,
		react.CreatedAt,
	)
	if err != nil {
		// Check for UNIQUE constraint violation
		if err.Error() == "UNIQUE constraint failed: reactions.message_id, reactions.user_id, reactions.emoji" {
			return 0, fmt.Errorf("reaction already exists")
		}
		return 0, fmt.Errorf("failed to insert reaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get inserted ID: %w", err)
	}

	return reaction.ReactionID(id), nil
}

// Remove deletes a reaction by message ID, user ID, and emoji
func (r *reactionRepository) Remove(ctx context.Context, messageID message.MessageID, userID string, emoji string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM reactions
		WHERE message_id = ? AND user_id = ? AND emoji = ?
	`,
		int64(messageID),
		userID,
		emoji,
	)
	if err != nil {
		return fmt.Errorf("failed to delete reaction: %w", err)
	}

	// Idempotent - no error if reaction didn't exist
	return nil
}

// ListByMessage retrieves all reactions for a specific message
func (r *reactionRepository) ListByMessage(ctx context.Context, messageID message.MessageID) ([]*reaction.Reaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_id, user_id, emoji, created_at
		FROM reactions
		WHERE message_id = ?
		ORDER BY created_at ASC
	`, int64(messageID))
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", err)
	}
	defer rows.Close()

	var reactions []*reaction.Reaction
	for rows.Next() {
		var (
			id        int64
			msgID     int64
			userIDStr string
			emoji     string
			createdAt time.Time
		)

		if err := rows.Scan(&id, &msgID, &userIDStr, &emoji, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan reaction: %w", err)
		}

		userID := shared.ActorID(userIDStr)

		reactions = append(reactions, &reaction.Reaction{
			ID:        reaction.ReactionID(id),
			MessageID: message.MessageID(msgID),
			UserID:    userID,
			Emoji:     emoji,
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reactions: %w", err)
	}

	return reactions, nil
}

// ListByMessages retrieves reactions for multiple messages
func (r *reactionRepository) ListByMessages(ctx context.Context, messageIDs []message.MessageID) (map[message.MessageID][]*reaction.Reaction, error) {
	if len(messageIDs) == 0 {
		return make(map[message.MessageID][]*reaction.Reaction), nil
	}

	// Build IN clause with placeholders
	placeholders := ""
	args := make([]interface{}, len(messageIDs))
	for i, msgID := range messageIDs {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = int64(msgID)
	}

	query := fmt.Sprintf(`
		SELECT id, message_id, user_id, emoji, created_at
		FROM reactions
		WHERE message_id IN (%s)
		ORDER BY message_id ASC, created_at ASC
	`, placeholders)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", err)
	}
	defer rows.Close()

	result := make(map[message.MessageID][]*reaction.Reaction)
	for rows.Next() {
		var (
			id        int64
			msgID     int64
			userIDStr string
			emoji     string
			createdAt time.Time
		)

		if err := rows.Scan(&id, &msgID, &userIDStr, &emoji, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan reaction: %w", err)
		}

		userID := shared.ActorID(userIDStr)

		react := &reaction.Reaction{
			ID:        reaction.ReactionID(id),
			MessageID: message.MessageID(msgID),
			UserID:    userID,
			Emoji:     emoji,
			CreatedAt: createdAt,
		}

		messageID := message.MessageID(msgID)
		result[messageID] = append(result[messageID], react)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reactions: %w", err)
	}

	return result, nil
}

// GetByID retrieves a single reaction by its ID
func (r *reactionRepository) GetByID(ctx context.Context, id reaction.ReactionID) (*reaction.Reaction, error) {
	var (
		messageID int64
		userIDStr string
		emoji     string
		createdAt time.Time
	)

	err := r.db.QueryRowContext(ctx, `
		SELECT message_id, user_id, emoji, created_at
		FROM reactions
		WHERE id = ?
	`, int64(id)).Scan(&messageID, &userIDStr, &emoji, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query reaction: %w", err)
	}

	userID := shared.ActorID(userIDStr)

	return &reaction.Reaction{
		ID:        id,
		MessageID: message.MessageID(messageID),
		UserID:    userID,
		Emoji:     emoji,
		CreatedAt: createdAt,
	}, nil
}
