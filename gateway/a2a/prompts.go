package a2a

import (
	"fmt"
	"strings"

	"memdoor/pkg/shared"
)

// BuildA2AContextPrompt builds the system prompt for an agent receiving an A2A message
// Pattern: OpenClaw src/agents/tools/sessions-send-tool.a2a.ts buildA2AContext()
func BuildA2AContextPrompt(msg *shared.A2AMessage) string {
	var parts []string

	// Header
	parts = append(parts, "Agent-to-agent message context:")
	parts = append(parts, "")

	// Requester information
	parts = append(parts, fmt.Sprintf("Requester agent: %s", msg.RequesterAgentID))
	parts = append(parts, fmt.Sprintf("Requester session: %s", msg.RequesterSessionKey))
	parts = append(parts, fmt.Sprintf("Target session: %s", msg.TargetSessionKey))
	parts = append(parts, "")

	// The message
	parts = append(parts, fmt.Sprintf("Message from %s agent:", msg.RequesterAgentID))
	parts = append(parts, msg.Message)
	parts = append(parts, "")

	// Instructions
	parts = append(parts, "Instructions:")
	parts = append(parts, "- Process this request and respond")
	parts = append(parts, "- You are being asked by another agent, not directly by the user")
	parts = append(parts, "- Provide a clear, actionable response")
	parts = append(parts, "- Use tools as needed to fulfill the request")

	return strings.Join(parts, "\n")
}

// TruncateMessage truncates a message to a maximum length for display
func TruncateMessage(message string, maxLength int) string {
	if len(message) <= maxLength {
		return message
	}

	// If maxLength is very small (< 4), can't fit ellipsis, just truncate
	if maxLength < 4 {
		return message[:maxLength]
	}

	// Truncate and add ellipsis
	return message[:maxLength-3] + "..."
}

// FormatA2AMessageForLog formats an A2A message for logging
func FormatA2AMessageForLog(msg *shared.A2AMessage) string {
	truncatedMsg := TruncateMessage(msg.Message, 100)
	return fmt.Sprintf("A2A: %s → %s: %s",
		msg.RequesterAgentID,
		msg.TargetAgentID,
		truncatedMsg)
}

// BuildA2AReplyContext builds the system prompt for a ping-pong turn
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts buildAgentToAgentReplyContext()
func BuildA2AReplyContext(params shared.A2AReplyContextParams) string {
	var parts []string

	// Header
	parts = append(parts, "Agent-to-agent reply step:")
	parts = append(parts, "")

	// Current agent info
	currentAgentLabel := "Agent 1 (requester)"
	if params.CurrentRole == "target" {
		currentAgentLabel = "Agent 2 (target)"
	}

	parts = append(parts, fmt.Sprintf("Current agent: %s.", currentAgentLabel))
	parts = append(parts, fmt.Sprintf("Turn %d of %d.", params.Turn, params.MaxTurns))
	parts = append(parts, "")

	// Agent context
	parts = append(parts, fmt.Sprintf("Agent 1 (requester) session: %s.", params.RequesterSessionKey))
	if params.RequesterChannel != "" {
		parts = append(parts, fmt.Sprintf("Agent 1 (requester) channel: %s.", params.RequesterChannel))
	}
	parts = append(parts, fmt.Sprintf("Agent 2 (target) session: %s.", params.TargetSessionKey))
	if params.TargetChannel != "" {
		parts = append(parts, fmt.Sprintf("Agent 2 (target) channel: %s.", params.TargetChannel))
	}
	parts = append(parts, "")

	// Instructions
	parts = append(parts, "Instructions:")
	parts = append(parts, "- Review the incoming message from the other agent")
	parts = append(parts, "- Respond with clarifications, follow-up questions, or additional information")
	parts = append(parts, fmt.Sprintf("- If you want to stop the ping-pong, reply exactly %q", shared.ReplySkipToken))
	parts = append(parts, "- Any other reply continues the conversation for the next turn")

	return strings.Join(parts, "\n")
}

// BuildA2AAnnounceContext builds the system prompt for the final announcement step
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts buildAgentToAgentAnnounceContext()
func BuildA2AAnnounceContext(params shared.A2AAnnounceContextParams) string {
	var parts []string

	// Header
	parts = append(parts, "Agent-to-agent announce step:")
	parts = append(parts, "")

	// Agent context
	parts = append(parts, fmt.Sprintf("Agent 1 (requester) session: %s.", params.RequesterSessionKey))
	if params.RequesterChannel != "" {
		parts = append(parts, fmt.Sprintf("Agent 1 (requester) channel: %s.", params.RequesterChannel))
	}
	parts = append(parts, fmt.Sprintf("Agent 2 (target) session: %s.", params.TargetSessionKey))
	if params.TargetChannel != "" {
		parts = append(parts, fmt.Sprintf("Agent 2 (target) channel: %s.", params.TargetChannel))
	}
	parts = append(parts, "")

	// Conversation summary
	parts = append(parts, "Conversation summary:")
	parts = append(parts, fmt.Sprintf("Original request: %s", TruncateMessage(params.OriginalMessage, 200)))

	if params.RoundOneReply != "" {
		parts = append(parts, fmt.Sprintf("Round 1 reply: %s", TruncateMessage(params.RoundOneReply, 200)))
	}

	if params.LatestReply != "" && params.LatestReply != params.RoundOneReply {
		parts = append(parts, fmt.Sprintf("Latest reply: %s", TruncateMessage(params.LatestReply, 200)))
	}
	parts = append(parts, "")

	// Instructions
	parts = append(parts, "Instructions:")
	parts = append(parts, "- Format a final announcement for the target channel")
	parts = append(parts, "- Summarize the key results from the conversation")
	parts = append(parts, fmt.Sprintf("- If you want to remain silent, reply exactly %q", shared.AnnounceSkipToken))
	parts = append(parts, "- Any other reply will be posted to the target channel")
	parts = append(parts, "- After this reply, the agent-to-agent conversation is over")

	return strings.Join(parts, "\n")
}
