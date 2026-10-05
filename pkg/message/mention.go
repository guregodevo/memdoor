package message

import (
	"fmt"
	"regexp"
	"strings"

	"memdoor/pkg/shared"
)

// REPLY_SKIP_TOKEN is the exact token agents can use to skip A2A mention processing
// Pattern adopted from OpenClaw for agent collaboration control
const REPLY_SKIP_TOKEN = "REPLY_SKIP"

// Mention represents an @ reference to a member in a message
type Mention struct {
	ActorID shared.ActorID // Who is being mentioned
	Name    string         // Display name (for rendering @john)
}

// Validate checks if the mention is valid
func (m Mention) Validate() error {
	if err := m.ActorID.Validate(); err != nil {
		return fmt.Errorf("invalid actor ID: %w", err)
	}
	if m.Name == "" {
		return fmt.Errorf("mention name cannot be empty")
	}
	return nil
}

// ParseMentions extracts @mentions from message text
// Returns list of @name strings found in the text
// Supports both @agent format and @user@domain format
// Actual ActorID lookup must be done by service layer
func ParseMentions(text string) []string {
	// Match @username or @email (e.g., @writer or @admin@localhost)
	// Pattern: @word or @word@word.word (supports agent names and emails)
	re := regexp.MustCompile(`@([a-zA-Z0-9_-]+(?:@[a-zA-Z0-9.-]+)?)`)
	matches := re.FindAllStringSubmatch(text, -1)

	mentions := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			name := strings.ToLower(match[1])
			if !seen[name] {
				mentions = append(mentions, name)
				seen[name] = true
			}
		}
	}

	return mentions
}

// IsReplySkip checks if the agent response contains the REPLY_SKIP token
// Returns true if the agent wants to skip further processing (no A2A mentions)
// Pattern: Agents can explicitly opt-out of triggering A2A communication
func IsReplySkip(text string) bool {
	return strings.TrimSpace(text) == REPLY_SKIP_TOKEN
}
