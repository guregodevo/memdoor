package message

import (
	"fmt"

	"memdoor/pkg/shared"
)

// MessageContent represents the content of a message
// For v1, keep it simple - just text with parsed mentions
// Later: Add rich formatting blocks (markdown, code, attachments)
type MessageContent struct {
	Text         string        // Raw message text
	Mentions     []Mention     // Parsed @ mentions
	LinkPreviews []LinkPreview // Deep link previews (unfurled message links)
}

// LinkPreview represents an unfurled link preview (e.g., message deep links)
type LinkPreview struct {
	URL  string              `json:"url"`  // Original URL
	Type string              `json:"type"` // "message", "url", "image"
	Data *LinkPreviewMessage `json:"message_data,omitempty"`
}

// LinkPreviewMessage contains preview data for message deep links
type LinkPreviewMessage struct {
	MessageID   string         `json:"message_id"`
	ChannelID   string         `json:"channel_id"`
	ChannelName string         `json:"channel_name"`
	AuthorID    shared.ActorID `json:"author_id"`
	AuthorName  string         `json:"author_name"`
	Text        string         `json:"text"`
	CreatedAt   string         `json:"created_at"`
}

// Validate checks if the content is valid
func (c MessageContent) Validate() error {
	if c.Text == "" {
		return fmt.Errorf("message text cannot be empty")
	}

	// Validate all mentions
	for _, mention := range c.Mentions {
		if err := mention.Validate(); err != nil {
			return fmt.Errorf("invalid mention: %w", err)
		}
	}

	return nil
}

// HasMentions returns true if the message contains @ mentions
func (c MessageContent) HasMentions() bool {
	return len(c.Mentions) > 0
}

// MentionsActor returns true if the message mentions a specific actor
func (c MessageContent) MentionsActor(actorID shared.ActorID) bool {
	for _, mention := range c.Mentions {
		if mention.ActorID == actorID {
			return true
		}
	}
	return false
}

// GetMentionedActors returns all mentioned actor IDs
func (c MessageContent) GetMentionedActors() []shared.ActorID {
	actorIDs := make([]shared.ActorID, len(c.Mentions))
	for i, mention := range c.Mentions {
		actorIDs[i] = mention.ActorID
	}
	return actorIDs
}
