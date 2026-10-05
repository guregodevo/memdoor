package dto

import (
	"context"
	"time"

	"github.com/google/uuid"
	"memdoor/pkg/domain"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"
)

// MessageDTO represents a message for API responses (presentation layer)
// This is separate from domain.Message to avoid mixing UI concerns with business logic
type MessageDTO struct {
	ID         string            `json:"id"`
	ChannelID  string            `json:"channel_id"`
	AuthorID   string            `json:"author_id"`
	AuthorName string            `json:"author_name"` // UI-specific: display name for the author
	Content    MessageContentDTO `json:"content"`
	Reactions  []ReactionDTO     `json:"reactions,omitempty"`
	ParentID   *int64            `json:"parent_id,omitempty"` // If this is a thread reply, ID of the parent message
	IsRead     bool              `json:"is_read"`
	CreatedAt  string            `json:"created_at"` // RFC3339 formatted
	UpdatedAt  string            `json:"updated_at"` // RFC3339 formatted
}

// MessageContentDTO represents message content for API responses
type MessageContentDTO struct {
	Text         string           `json:"text"`
	Mentions     []string         `json:"mentions"`
	LinkPreviews []LinkPreviewDTO `json:"link_previews,omitempty"`
	Attachments  []FileRefDTO     `json:"attachments,omitempty"`
}

// FileRefDTO represents a file attachment reference for API responses
type FileRefDTO struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	URL       string `json:"url"`
}

// LinkPreviewDTO represents a link preview for API responses
type LinkPreviewDTO struct {
	URL  string                 `json:"url"`
	Type string                 `json:"type"` // "message", "url", "image"
	Data *LinkPreviewMessageDTO `json:"message_data,omitempty"`
}

// LinkPreviewMessageDTO represents message preview data
type LinkPreviewMessageDTO struct {
	MessageID   string `json:"message_id"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	AuthorID    string `json:"author_id"`
	AuthorName  string `json:"author_name"`
	Text        string `json:"text"`
	CreatedAt   string `json:"created_at"`
}

// ReactionDTO represents a reaction for API responses
type ReactionDTO struct {
	ID        int64  `json:"id"`
	UserID    string `json:"user_id"`
	Emoji     string `json:"emoji"`
	CreatedAt string `json:"created_at"` // RFC3339 formatted
}

// NameEnricher interface for looking up display names
type NameEnricher interface {
	LoadSingle(ctx context.Context, actorID shared.ActorID) string
}

// EnrichLinkPreviewNames enriches link preview data with actual author and channel names
// channelRepo can be any repository that returns a channel with a Name field
func EnrichLinkPreviewNames(ctx context.Context, previews []message.LinkPreview, nameEnricher NameEnricher, channelRepo interface{}) []message.LinkPreview {
	enriched := make([]message.LinkPreview, len(previews))
	for i, preview := range previews {
		enriched[i] = preview // Copy the preview
		if preview.Data != nil {
			// Enrich author name
			if nameEnricher != nil {
				enriched[i].Data.AuthorName = nameEnricher.LoadSingle(ctx, preview.Data.AuthorID)
			}

			// Enrich channel name
			if channelRepo != nil {
				channelUUID, err := uuid.Parse(preview.Data.ChannelID)
				if err == nil {
					// Use type assertion to call GetByID and access Name field
					type channelRepositoryType interface {
						GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error)
					}
					if repo, ok := channelRepo.(channelRepositoryType); ok {
						if ch, err := repo.GetByID(ctx, channelUUID); err == nil && ch != nil {
							enriched[i].Data.ChannelName = ch.Name
						}
					}
				}
			}
		}
	}
	return enriched
}

// ToMessageDTO converts a domain Message to a DTO with enriched display name
func ToMessageDTO(msg *message.Message, authorName string, reactions []*reaction.Reaction, attachments ...FileRefDTO) MessageDTO {
	// Convert mentions to string array
	mentions := make([]string, len(msg.Content.Mentions))
	for i, mention := range msg.Content.Mentions {
		mentions[i] = mention.ActorID.String()
	}

	// Convert link previews
	linkPreviews := make([]LinkPreviewDTO, len(msg.Content.LinkPreviews))
	for i, preview := range msg.Content.LinkPreviews {
		var previewData *LinkPreviewMessageDTO
		if preview.Data != nil {
			previewData = &LinkPreviewMessageDTO{
				MessageID:   preview.Data.MessageID,
				ChannelID:   preview.Data.ChannelID,
				ChannelName: preview.Data.ChannelName,
				AuthorID:    preview.Data.AuthorID.String(),
				AuthorName:  preview.Data.AuthorName,
				Text:        preview.Data.Text,
				CreatedAt:   preview.Data.CreatedAt,
			}
		}
		linkPreviews[i] = LinkPreviewDTO{
			URL:  preview.URL,
			Type: preview.Type,
			Data: previewData,
		}
	}

	// Convert reactions
	reactionDTOs := make([]ReactionDTO, 0, len(reactions))
	for _, r := range reactions {
		reactionDTOs = append(reactionDTOs, ReactionDTO{
			ID:        int64(r.ID),
			UserID:    r.UserID.String(),
			Emoji:     r.Emoji,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}

	// Convert ParentID to *int64 for JSON omitempty
	var parentID *int64
	if msg.ParentID != nil {
		id := int64(*msg.ParentID)
		parentID = &id
	}

	return MessageDTO{
		ID:         msg.ID.String(),
		ChannelID:  msg.ChannelID,
		AuthorID:   msg.AuthorID.String(),
		AuthorName: authorName,
		Content: MessageContentDTO{
			Text:         msg.Content.Text,
			Mentions:     mentions,
			LinkPreviews: linkPreviews,
			Attachments:  attachments,
		},
		Reactions: reactionDTOs,
		ParentID:  parentID,
		IsRead:    msg.IsRead,
		CreatedAt: msg.CreatedAt.Format(time.RFC3339),
		UpdatedAt: formatUpdatedAt(msg),
	}
}

// formatUpdatedAt returns the updated_at timestamp, falling back to created_at
func formatUpdatedAt(msg *message.Message) string {
	if msg.UpdatedAt != nil {
		return msg.UpdatedAt.Format(time.RFC3339)
	}
	return msg.CreatedAt.Format(time.RFC3339)
}

// ToMessageDTOBatch converts multiple domain Messages to DTOs
// displayNames should be a map of actorID -> displayName
func ToMessageDTOBatch(messages []*message.Message, displayNames map[string]string, reactionsByMessage map[message.MessageID][]*reaction.Reaction) []MessageDTO {
	dtos := make([]MessageDTO, len(messages))
	for i, msg := range messages {
		authorID := msg.AuthorID.String()
		authorName := displayNames[authorID]
		if authorName == "" {
			authorName = authorID // Fallback
		}

		reactions := reactionsByMessage[msg.ID]
		if reactions == nil {
			reactions = []*reaction.Reaction{}
		}

		dtos[i] = ToMessageDTO(msg, authorName, reactions)
	}
	return dtos
}
