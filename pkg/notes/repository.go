// Package notes is an agent's working memory for one conversation: findings
// it appends as it works and reads back after a cut reply or a compaction.
//
// Notes belong to the CONVERSATION, not to a folder. They lived in
// .memdoor-notes.md in the project folder, read at the start of every turn
// and never expired, so a rule from last week's task steered today's: after
// /fresh the coder read "work happens on the remote-control branch" and went
// looking for a deploy nobody had asked for (2026-09-29). Now a new
// conversation starts with none, /fresh and /clear wipe them with the
// agent's memory, and nothing is written into the person's repository.
package notes

import (
	"errors"
	"strings"
)

// ConversationKey names the conversation notes belong to: the session key
// of the screen the person talks to. A sub-session working for that screen
// uses the same key, so what it notes the parent reads.
type ConversationKey string

// ErrNoConversation is a notes call made outside any conversation.
var ErrNoConversation = errors.New("notes belong to a conversation, and this call has none")

// ParseConversationKey validates a key at the boundary.
func ParseConversationKey(s string) (ConversationKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrNoConversation
	}
	return ConversationKey(s), nil
}

func (k ConversationKey) String() string { return string(k) }

// Repository stores each conversation's notes.
type Repository interface {
	// Load returns the conversation's notes, "" when it has none.
	Load(key ConversationKey) (string, error)
	// Append adds text at the end of the conversation's notes.
	Append(key ConversationKey, text string) error
	// Delete removes the conversation's notes; none is not an error.
	Delete(key ConversationKey) error
}
