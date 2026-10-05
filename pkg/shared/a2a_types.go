package shared

import (
	"strings"
	"time"
)

// Magic tokens for ping-pong conversation control
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts lines 8-9
const (
	ReplySkipToken    = "REPLY_SKIP"    // Agent signals end of ping-pong conversation
	AnnounceSkipToken = "ANNOUNCE_SKIP" // Agent chooses to remain silent (no announcement)
)

// A2AMessage represents a message sent from one agent to another
type A2AMessage struct {
	RequesterSessionKey string    `json:"requesterSessionKey"` // Who sent it
	RequesterAgentID    string    `json:"requesterAgentId"`    // Which agent sent it
	TargetSessionKey    string    `json:"targetSessionKey"`    // Who receives it
	TargetAgentID       string    `json:"targetAgentId"`       // Which agent receives it
	Message             string    `json:"message"`             // The message content
	TimeoutSeconds      int       `json:"timeoutSeconds"`      // 0 = fire-and-forget, >0 = wait
	SentAt              time.Time `json:"sentAt"`
}

// A2AResponse represents the response from the target agent
type A2AResponse struct {
	Message      string        `json:"message"`
	Status       string        `json:"status"` // "ok", "error", "timeout"
	Error        string        `json:"error,omitempty"`
	ResponseTime time.Duration `json:"responseTime"`
}

// SessionResolutionParams holds parameters for resolving a target session
type SessionResolutionParams struct {
	SessionKey string  // Explicit session key
	Label      string  // Session label
	AgentID    *string // Optional agent ID (for cross-agent label lookup)
}

// A2AAnnounceParams holds parameters for announcing A2A results
type A2AAnnounceParams struct {
	RequesterSessionKey string
	TargetAgentID       string
	OriginalMessage     string
	Response            *A2AResponse
	DurationMs          int64
}

// ConversationHistory tracks the progression of a ping-pong conversation
// Pattern: OpenClaw sessions-send-tool.a2a.ts (primaryReply vs latestReply)
type ConversationHistory struct {
	OriginalMessage string // Initial request from requester
	RoundOneReply   string // First response from target agent
	LatestReply     string // Current response after ping-pong exchanges
	TurnCount       int    // Number of ping-pong turns completed
}

// PingPongState manages state during multi-turn ping-pong conversation
type PingPongState struct {
	CurrentSessionKey string // Which agent's turn it is
	NextSessionKey    string // Which agent is next
	IncomingMessage   string // Message to deliver in this turn
	History           *ConversationHistory
	Turn              int // Current turn number (1-based)
	MaxTurns          int // Maximum allowed turns
}

// IsReplySkip checks if the given text is the REPLY_SKIP token
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts lines 150-152
func IsReplySkip(text string) bool {
	return strings.TrimSpace(text) == ReplySkipToken
}

// IsAnnounceSkip checks if the given text is the ANNOUNCE_SKIP token
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts lines 154-156
func IsAnnounceSkip(text string) bool {
	return strings.TrimSpace(text) == AnnounceSkipToken
}

// A2AReplyContextParams holds parameters for building the ping-pong reply prompt
// Pattern: OpenClaw buildAgentToAgentReplyContext()
type A2AReplyContextParams struct {
	RequesterSessionKey string
	RequesterChannel    string
	TargetSessionKey    string
	TargetChannel       string
	CurrentRole         string // "requester" or "target"
	Turn                int    // Current turn number (1-based)
	MaxTurns            int    // Maximum allowed turns
}

// A2AAnnounceContextParams holds parameters for building the announcement prompt
// Pattern: OpenClaw buildAgentToAgentAnnounceContext()
type A2AAnnounceContextParams struct {
	RequesterSessionKey string
	RequesterChannel    string
	TargetSessionKey    string
	TargetChannel       string
	OriginalMessage     string
	RoundOneReply       string
	LatestReply         string
}
