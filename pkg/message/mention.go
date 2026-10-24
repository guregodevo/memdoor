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
	// Match @username or @email (e.g., @writer or @admin@localhost), or a
	// peer's agent, @<peer>:<agent> (e.g., @box:coder; PeerAgent splits it).
	// Pattern: @word or @word@word.word (supports agent names and emails)
	re := regexp.MustCompile(`@([a-zA-Z0-9_.-]+:[a-zA-Z0-9_-]+|[a-zA-Z0-9_-]+(?:@[a-zA-Z0-9.-]+)?)`)
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

// PeerAgent splits a mention of another gateway's agent, @<peer>:<agent>
// (experimental, 2026-10-24: "we can do @<host>:agent_name"): the peer's
// name as `memdoor peer add` gave it, and the agent's name there. Whether
// the peer exists is the service's to say; "@coder:" followed by prose is
// not one.
func PeerAgent(name string) (peer, agent string, ok bool) {
	i := strings.Index(name, ":")
	if i <= 0 || i == len(name)-1 {
		return "", "", false
	}
	return name[:i], name[i+1:], true
}

// IsReplySkip checks if the agent response contains the REPLY_SKIP token
// Returns true if the agent wants to skip further processing (no A2A mentions)
// Pattern: Agents can explicitly opt-out of triggering A2A communication
func IsReplySkip(text string) bool {
	return strings.TrimSpace(text) == REPLY_SKIP_TOKEN
}
