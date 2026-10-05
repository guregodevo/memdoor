package message

import (
	"fmt"
	"time"

	"memdoor/pkg/shared"
)

// Message is an immutable aggregate representing a message in a channel
// Messages are append-only - no edits, no reactions (those are separate aggregates)
//
// INVARIANTS:
// 1. Author must be a member of channel (enforced by service on write)
// 2. Parent message must exist if ParentID != nil (enforced by service on write)
// 3. Message is immutable once created (no updates allowed)
type Message struct {
	// Aggregate identity
	ID MessageID // Auto-increment ID

	// Location
	ChannelID string // Which channel this message is in

	// Authorship
	AuthorID shared.ActorID // Who posted this (human:<uuid> or agent:<uuid>)

	// Content
	Content MessageContent // Message text and mentions

	// Threading
	ParentID *MessageID // If this is a reply (nil for top-level messages)

	// Read status (mutable)
	IsRead bool // Whether the message has been read by the user

	// Metadata
	CreatedAt time.Time  // When message was posted
	UpdatedAt *time.Time // When message was last edited (nil if never edited)
}

// NewMessage creates a new message
// ID is not set (assigned by repository on insert)
// CreatedAt is set to now
func NewMessage(channelID string, authorID shared.ActorID, content MessageContent) (*Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel ID cannot be empty")
	}

	if err := authorID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid author ID: %w", err)
	}

	if err := content.Validate(); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}

	return &Message{
		ID:        0, // Assigned by repository
		ChannelID: channelID,
		AuthorID:  authorID,
		Content:   content,
		ParentID:  nil,
		CreatedAt: time.Now(),
	}, nil
}

// NewReply creates a new message as a reply to an existing message
func NewReply(channelID string, authorID shared.ActorID, content MessageContent, parentID MessageID) (*Message, error) {
	msg, err := NewMessage(channelID, authorID, content)
	if err != nil {
		return nil, err
	}

	if err := parentID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parent ID: %w", err)
	}

	msg.ParentID = &parentID
	return msg, nil
}

// Validate checks if the message is valid
func (m *Message) Validate() error {
	// ID can be 0 (not yet persisted)
	if m.ID != 0 {
		if err := m.ID.Validate(); err != nil {
			return fmt.Errorf("invalid message ID: %w", err)
		}
	}

	if m.ChannelID == "" {
		return fmt.Errorf("channel ID cannot be empty")
	}

	if err := m.AuthorID.Validate(); err != nil {
		return fmt.Errorf("invalid author ID: %w", err)
	}

	if err := m.Content.Validate(); err != nil {
		return fmt.Errorf("invalid content: %w", err)
	}

	if m.ParentID != nil {
		if err := m.ParentID.Validate(); err != nil {
			return fmt.Errorf("invalid parent ID: %w", err)
		}
	}

	return nil
}

// EditContent updates the message content and sets the UpdatedAt timestamp
func (m *Message) EditContent(newContent MessageContent) error {
	if err := newContent.Validate(); err != nil {
		return fmt.Errorf("invalid content: %w", err)
	}
	m.Content = newContent
	now := time.Now()
	m.UpdatedAt = &now
	return nil
}

// IsEdited returns true if the message has been edited
func (m *Message) IsEdited() bool {
	return m.UpdatedAt != nil
}

// IsReply returns true if this message is a reply to another message
func (m *Message) IsReply() bool {
	return m.ParentID != nil
}

// IsFromHuman returns true if message was posted by a human
func (m *Message) IsFromHuman() bool {
	return m.AuthorID.IsHuman()
}

// IsFromAgent returns true if message was posted by an agent
func (m *Message) IsFromAgent() bool {
	return m.AuthorID.IsAgent()
}

// HasMentions returns true if message contains @ mentions
func (m *Message) HasMentions() bool {
	return m.Content.HasMentions()
}

// MentionsActor returns true if message mentions a specific actor
func (m *Message) MentionsActor(actorID shared.ActorID) bool {
	return m.Content.MentionsActor(actorID)
}

// GetMentionedActors returns all mentioned actor IDs
func (m *Message) GetMentionedActors() []shared.ActorID {
	return m.Content.GetMentionedActors()
}

// GetMentionedAgents returns only mentioned agents (filters out humans)
// Used to trigger agent executions
func (m *Message) GetMentionedAgents() []shared.ActorID {
	actors := m.GetMentionedActors()
	agents := make([]shared.ActorID, 0, len(actors))

	for _, actor := range actors {
		if actor.IsAgent() {
			agents = append(agents, actor)
		}
	}

	return agents
}

// GetThreadRootID returns the thread root message ID for flat threading (Slack-style)
// If this message is in a thread (has ParentID), returns the parent (thread root)
// Otherwise returns this message's ID (creating a new thread root)
// Pattern: All thread replies point to the same root, not nested (Reddit-style)
func (m *Message) GetThreadRootID() MessageID {
	if m.ParentID != nil {
		// Already in a thread, return the root
		return *m.ParentID
	}
	// This is the root (or will become the root when replied to)
	return m.ID
}
