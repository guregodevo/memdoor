package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"memdoor/pkg/message"
	"memdoor/pkg/shared"
)

// messageRepository implements message.Repository for SQLite
type messageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new SQLite message repository
func NewMessageRepository(db *sql.DB) message.Repository {
	return &messageRepository{db: db}
}

// messageColumns is the standard column list for SELECT queries
const messageColumns = `id, channel_id, author_id, content_text, content_mentions, content_link_previews, parent_id, is_read, created_at, updated_at`

// messageColumnsNoID is the column list without id (for FindByID where id is the WHERE clause)
const messageColumnsNoID = `channel_id, author_id, content_text, content_mentions, content_link_previews, parent_id, is_read, created_at, updated_at`

// scanMessage scans a row into a message.Message
func scanMessage(scanner interface{ Scan(...interface{}) error }, includeID bool) (*message.Message, error) {
	var (
		id               int64
		channelID        string
		authorIDStr      string
		contentText      string
		mentionsJSON     sql.NullString
		linkPreviewsJSON sql.NullString
		parentID         *int64
		isRead           int
		createdAt        time.Time
		updatedAt        sql.NullTime
	)

	var err error
	if includeID {
		err = scanner.Scan(&id, &channelID, &authorIDStr, &contentText, &mentionsJSON, &linkPreviewsJSON, &parentID, &isRead, &createdAt, &updatedAt)
	} else {
		err = scanner.Scan(&channelID, &authorIDStr, &contentText, &mentionsJSON, &linkPreviewsJSON, &parentID, &isRead, &createdAt, &updatedAt)
	}
	if err != nil {
		return nil, err
	}

	authorID := shared.ActorID(authorIDStr)

	var mentions []message.Mention
	if mentionsJSON.Valid && mentionsJSON.String != "" && mentionsJSON.String != "null" {
		if err := json.Unmarshal([]byte(mentionsJSON.String), &mentions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal mentions: %w", err)
		}
	}

	var linkPreviews []message.LinkPreview
	if linkPreviewsJSON.Valid && linkPreviewsJSON.String != "" && linkPreviewsJSON.String != "null" {
		if err := json.Unmarshal([]byte(linkPreviewsJSON.String), &linkPreviews); err != nil {
			return nil, fmt.Errorf("failed to unmarshal link previews: %w", err)
		}
	}

	var msgParentID *message.MessageID
	if parentID != nil {
		msgID := message.MessageID(*parentID)
		msgParentID = &msgID
	}

	var updatedAtPtr *time.Time
	if updatedAt.Valid {
		updatedAtPtr = &updatedAt.Time
	}

	msg := &message.Message{
		ID:        message.MessageID(id),
		ChannelID: channelID,
		AuthorID:  authorID,
		Content: message.MessageContent{
			Text:         contentText,
			Mentions:     mentions,
			LinkPreviews: linkPreviews,
		},
		ParentID:  msgParentID,
		IsRead:    isRead == 1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAtPtr,
	}

	return msg, nil
}

// scanMessages scans multiple rows into message slices
func scanMessages(rows *sql.Rows) ([]*message.Message, error) {
	var messages []*message.Message
	for rows.Next() {
		msg, err := scanMessage(rows, true)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

// Save saves a message and returns the assigned ID
func (r *messageRepository) Save(ctx context.Context, msg *message.Message) (message.MessageID, error) {
	mentionsJSON, err := json.Marshal(msg.Content.Mentions)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal mentions: %w", err)
	}

	linkPreviewsJSON, err := json.Marshal(msg.Content.LinkPreviews)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal link previews: %w", err)
	}

	var parentID *int64
	if msg.ParentID != nil {
		id := int64(*msg.ParentID)
		parentID = &id
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO messages (channel_id, author_id, content_text, content_mentions, content_link_previews, parent_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		msg.ChannelID,
		msg.AuthorID.String(),
		msg.Content.Text,
		string(mentionsJSON),
		string(linkPreviewsJSON),
		parentID,
		msg.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get inserted ID: %w", err)
	}

	return message.MessageID(id), nil
}

// FindByID retrieves a message by ID
func (r *messageRepository) FindByID(ctx context.Context, id message.MessageID) (*message.Message, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+messageColumnsNoID+`
		FROM messages
		WHERE id = ?
	`, int64(id))

	msg, err := scanMessage(row, false)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query message: %w", err)
	}

	msg.ID = id
	return msg, nil
}

// ListByChannel retrieves messages for a channel with cursor-based pagination.
func (r *messageRepository) ListByChannel(ctx context.Context, channelID string, page shared.CursorPage) (*message.PaginatedMessages, error) {
	fetchLimit := page.Limit + 1

	var rows *sql.Rows
	var err error

	if page.BeforeID > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+messageColumns+`
			FROM (
				SELECT `+messageColumns+`
				FROM messages
				WHERE channel_id = ? AND id < ?
				ORDER BY id DESC
				LIMIT ?
			) AS recent_messages
			ORDER BY id ASC
		`, channelID, page.BeforeID, fetchLimit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+messageColumns+`
			FROM (
				SELECT `+messageColumns+`
				FROM messages
				WHERE channel_id = ?
				ORDER BY id DESC
				LIMIT ?
			) AS recent_messages
			ORDER BY id ASC
		`, channelID, fetchLimit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	hasMore := len(messages) > page.Limit
	if hasMore {
		messages = messages[1:]
	}

	return &message.PaginatedMessages{
		Messages: messages,
		HasMore:  hasMore,
	}, nil
}

// ListReplies retrieves replies to a message (chronological order)
func (r *messageRepository) ListReplies(ctx context.Context, parentID message.MessageID) ([]*message.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+messageColumns+`
		FROM messages
		WHERE parent_id = ?
		ORDER BY created_at ASC
	`, int64(parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to query replies: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan replies: %w", err)
	}

	return messages, nil
}

// ListByAuthor retrieves messages by author (most recent first)
func (r *messageRepository) ListByAuthor(ctx context.Context, authorID shared.ActorID, limit int) ([]*message.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+messageColumns+`
		FROM messages
		WHERE author_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, authorID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	return messages, nil
}

// ListSince retrieves messages in a channel since a given message ID
func (r *messageRepository) ListSince(ctx context.Context, channelID string, sinceID message.MessageID) ([]*message.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+messageColumns+`
		FROM messages
		WHERE channel_id = ? AND id > ?
		ORDER BY created_at ASC
	`, channelID, int64(sinceID))
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	return messages, nil
}

// MarkAsRead marks a message as read
func (r *messageRepository) MarkAsRead(ctx context.Context, id message.MessageID) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE messages
		SET is_read = 1
		WHERE id = ?
	`, int64(id))
	if err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message not found: %d", id)
	}

	return nil
}

// MarkChannelAsRead marks all unread messages in a channel as read
func (r *messageRepository) MarkChannelAsRead(ctx context.Context, channelID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE messages
		SET is_read = 1
		WHERE channel_id = ? AND is_read = 0
	`, channelID)
	if err != nil {
		return fmt.Errorf("failed to mark channel messages as read: %w", err)
	}
	return nil
}

// UpdateContent updates the content and updated_at timestamp of a message
func (r *messageRepository) UpdateContent(ctx context.Context, msg *message.Message) error {
	mentionsJSON, err := json.Marshal(msg.Content.Mentions)
	if err != nil {
		return fmt.Errorf("failed to marshal mentions: %w", err)
	}

	linkPreviewsJSON, err := json.Marshal(msg.Content.LinkPreviews)
	if err != nil {
		return fmt.Errorf("failed to marshal link previews: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE messages
		SET content_text = ?, content_mentions = ?, content_link_previews = ?, updated_at = ?
		WHERE id = ?
	`, msg.Content.Text, string(mentionsJSON), string(linkPreviewsJSON), msg.UpdatedAt, int64(msg.ID))
	if err != nil {
		return fmt.Errorf("failed to update message content: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message not found: %d", msg.ID)
	}

	return nil
}

// CountUnreadMentionsByChannel returns a map of channel_id -> count of unread @mentions for a user
func (r *messageRepository) CountUnreadMentionsByChannel(ctx context.Context, userID shared.ActorID) (map[string]int, error) {
	query := `
		SELECT channel_id, COUNT(*) as mention_count
		FROM messages
		WHERE is_read = 0
		  AND author_id != ?
		  AND EXISTS (
			SELECT 1
			FROM json_each(content_mentions)
			WHERE json_extract(value, '$.ActorID') = ?
		  )
		GROUP BY channel_id
	`

	rows, err := r.db.QueryContext(ctx, query, userID.String(), userID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query unread mention counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var channelID string
		var count int
		if err := rows.Scan(&channelID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan mention count row: %w", err)
		}
		counts[channelID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating mention count rows: %w", err)
	}

	return counts, nil
}

// ListUnreadMentions retrieves all messages that mention the user
func (r *messageRepository) ListUnreadMentions(ctx context.Context, userID shared.ActorID, limit int) ([]*message.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT ` + messageColumns + `
		FROM messages
		WHERE author_id != ?
		  AND EXISTS (
			SELECT 1
			FROM json_each(content_mentions)
			WHERE json_extract(value, '$.ActorID') = ?
		  )
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID.String(), userID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unread mentions: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan unread mentions: %w", err)
	}

	return messages, nil
}

// ListUnreadThreadReplies retrieves unread replies to the user's messages
func (r *messageRepository) ListUnreadThreadReplies(ctx context.Context, userID shared.ActorID, limit int) ([]*message.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT m.id, m.channel_id, m.author_id, m.content_text, m.content_mentions, m.content_link_previews, m.parent_id, m.is_read, m.created_at, m.updated_at
		FROM messages m
		JOIN messages parent ON m.parent_id = parent.id
		WHERE parent.author_id = ?
		  AND m.author_id <> ?
		  AND m.is_read = 0
		ORDER BY m.created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID.String(), userID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unread thread replies: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan unread thread replies: %w", err)
	}

	return messages, nil
}
