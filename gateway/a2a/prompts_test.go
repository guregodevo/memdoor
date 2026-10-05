package a2a

import (
	"strings"
	"testing"
	"time"

	"memdoor/pkg/shared"
)

// TestBuildA2AContextPrompt tests the A2A context prompt builder
func TestBuildA2AContextPrompt(t *testing.T) {
	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Check calendar for tomorrow",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	prompt := BuildA2AContextPrompt(msg)

	// Verify prompt contains key elements
	requiredElements := []string{
		"Agent-to-agent message context:",
		"Requester agent: main",
		"Requester session: agent:main:main",
		"Target session: agent:work:main",
		"Message from main agent:",
		"Check calendar for tomorrow",
		"Instructions:",
		"Process this request and respond",
		"You are being asked by another agent",
	}

	for _, element := range requiredElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("Prompt missing required element: '%s'", element)
		}
	}

	// Verify prompt is not empty
	if len(prompt) == 0 {
		t.Error("Prompt should not be empty")
	}

	t.Logf("Generated prompt:\n%s", prompt)
}

// TestBuildA2AContextPrompt_LongMessage tests handling of long messages
func TestBuildA2AContextPrompt_LongMessage(t *testing.T) {
	longMessage := strings.Repeat("This is a long message. ", 100) // ~2400 chars

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:data:main",
		TargetAgentID:       "data",
		Message:             longMessage,
		TimeoutSeconds:      60,
		SentAt:              time.Now(),
	}

	prompt := BuildA2AContextPrompt(msg)

	// Should include the full message (no truncation in prompts)
	if !strings.Contains(prompt, longMessage) {
		t.Error("Prompt should contain full long message")
	}

	// Should still have all required elements
	if !strings.Contains(prompt, "Requester agent: main") {
		t.Error("Prompt should contain requester agent")
	}

	if !strings.Contains(prompt, "Instructions:") {
		t.Error("Prompt should contain instructions")
	}
}

// TestBuildA2AContextPrompt_SpecialCharacters tests handling of special characters
func TestBuildA2AContextPrompt_SpecialCharacters(t *testing.T) {
	specialMessage := "Query:\n\nSELECT * FROM users\nWHERE id = 'test@example.com'\n\n\"Check this\""

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:data:main",
		TargetAgentID:       "data",
		Message:             specialMessage,
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	prompt := BuildA2AContextPrompt(msg)

	// Should preserve special characters
	if !strings.Contains(prompt, specialMessage) {
		t.Error("Prompt should preserve special characters and formatting")
	}
}

// TestTruncateMessage tests message truncation
func TestTruncateMessage(t *testing.T) {
	testCases := []struct {
		name      string
		message   string
		maxLength int
		expected  string
	}{
		{
			name:      "Short message, no truncation",
			message:   "Hello",
			maxLength: 100,
			expected:  "Hello",
		},
		{
			name:      "Exact length",
			message:   "Hello",
			maxLength: 5,
			expected:  "Hello",
		},
		{
			name:      "Truncate with ellipsis",
			message:   "This is a long message that should be truncated",
			maxLength: 20,
			expected:  "This is a long me...",
		},
		{
			name:      "Very short max length",
			message:   "Hello world",
			maxLength: 5,
			expected:  "He...", // Truncates to fit with ellipsis
		},
		{
			name:      "Max length < 10",
			message:   "Hello world",
			maxLength: 8,
			expected:  "Hello...",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateMessage(tc.message, tc.maxLength)

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}

			// Verify result doesn't exceed maxLength
			if len(result) > tc.maxLength {
				t.Errorf("Truncated message exceeds maxLength: %d > %d", len(result), tc.maxLength)
			}
		})
	}
}

// TestFormatA2AMessageForLog tests log message formatting
func TestFormatA2AMessageForLog(t *testing.T) {
	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Check calendar for tomorrow",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	logMsg := FormatA2AMessageForLog(msg)

	// Should contain key info
	if !strings.Contains(logMsg, "main") {
		t.Error("Log message should contain requester agent ID")
	}

	if !strings.Contains(logMsg, "work") {
		t.Error("Log message should contain target agent ID")
	}

	if !strings.Contains(logMsg, "Check calendar") {
		t.Error("Log message should contain message content")
	}

	// Should have arrow notation
	if !strings.Contains(logMsg, "→") {
		t.Error("Log message should contain arrow (→)")
	}

	t.Logf("Log message: %s", logMsg)
}

// TestFormatA2AMessageForLog_LongMessage tests log formatting with long messages
func TestFormatA2AMessageForLog_LongMessage(t *testing.T) {
	longMessage := strings.Repeat("This is a long message. ", 50) // ~1200 chars

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             longMessage,
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	logMsg := FormatA2AMessageForLog(msg)

	// Should be truncated (FormatA2AMessageForLog uses TruncateMessage with maxLength=100)
	if len(logMsg) > 200 { // Generous limit accounting for agent names and formatting
		t.Errorf("Log message should be truncated, got length: %d", len(logMsg))
	}

	// Should contain ellipsis if truncated
	if !strings.Contains(logMsg, "...") {
		t.Error("Long message should be truncated with ellipsis")
	}

	t.Logf("Truncated log message: %s", logMsg)
}

// TestBuildA2AReplyContext tests the ping-pong reply prompt builder
func TestBuildA2AReplyContext(t *testing.T) {
	params := shared.A2AReplyContextParams{
		RequesterSessionKey: "agent:main:main",
		RequesterChannel:    "whatsapp",
		TargetSessionKey:    "agent:work:main",
		TargetChannel:       "internal",
		CurrentRole:         "requester",
		Turn:                2,
		MaxTurns:            5,
	}

	prompt := BuildA2AReplyContext(params)

	// Verify prompt contains key elements
	requiredElements := []string{
		"Agent-to-agent reply step:",
		"Current agent: Agent 1 (requester).",
		"Turn 2 of 5.",
		"Agent 1 (requester) session: agent:main:main.",
		"Agent 1 (requester) channel: whatsapp.",
		"Agent 2 (target) session: agent:work:main.",
		"Agent 2 (target) channel: internal.",
		"Instructions:",
		"Review the incoming message",
		"REPLY_SKIP",
	}

	for _, element := range requiredElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("Reply context prompt missing required element: '%s'", element)
		}
	}

	t.Logf("Generated reply context prompt:\n%s", prompt)
}

// TestBuildA2AReplyContext_TargetRole tests reply context when current role is target
func TestBuildA2AReplyContext_TargetRole(t *testing.T) {
	params := shared.A2AReplyContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		CurrentRole:         "target",
		Turn:                3,
		MaxTurns:            5,
	}

	prompt := BuildA2AReplyContext(params)

	// Should indicate target role
	if !strings.Contains(prompt, "Current agent: Agent 2 (target).") {
		t.Error("Prompt should indicate current agent is target")
	}

	if !strings.Contains(prompt, "Turn 3 of 5.") {
		t.Error("Prompt should show correct turn number")
	}

	t.Logf("Target role prompt:\n%s", prompt)
}

// TestBuildA2AReplyContext_NoChannels tests reply context without channels
func TestBuildA2AReplyContext_NoChannels(t *testing.T) {
	params := shared.A2AReplyContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		CurrentRole:         "requester",
		Turn:                1,
		MaxTurns:            5,
	}

	prompt := BuildA2AReplyContext(params)

	// Should not mention channels
	if strings.Contains(prompt, "channel:") {
		t.Error("Prompt should not mention channels when not provided")
	}

	// Should still have all other elements
	if !strings.Contains(prompt, "Agent 1 (requester) session:") {
		t.Error("Prompt should contain requester session")
	}

	if !strings.Contains(prompt, "Turn 1 of 5.") {
		t.Error("Prompt should show turn number")
	}

	t.Logf("No channels prompt:\n%s", prompt)
}

// TestBuildA2AReplyContext_FirstTurn tests reply context for first turn
func TestBuildA2AReplyContext_FirstTurn(t *testing.T) {
	params := shared.A2AReplyContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		CurrentRole:         "target",
		Turn:                1,
		MaxTurns:            5,
	}

	prompt := BuildA2AReplyContext(params)

	if !strings.Contains(prompt, "Turn 1 of 5.") {
		t.Error("Prompt should indicate first turn")
	}

	t.Logf("First turn prompt:\n%s", prompt)
}

// TestBuildA2AReplyContext_LastTurn tests reply context for last turn
func TestBuildA2AReplyContext_LastTurn(t *testing.T) {
	params := shared.A2AReplyContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		CurrentRole:         "requester",
		Turn:                5,
		MaxTurns:            5,
	}

	prompt := BuildA2AReplyContext(params)

	if !strings.Contains(prompt, "Turn 5 of 5.") {
		t.Error("Prompt should indicate last turn")
	}

	t.Logf("Last turn prompt:\n%s", prompt)
}

// TestBuildA2AAnnounceContext tests the announcement prompt builder
func TestBuildA2AAnnounceContext(t *testing.T) {
	params := shared.A2AAnnounceContextParams{
		RequesterSessionKey: "agent:main:main",
		RequesterChannel:    "whatsapp",
		TargetSessionKey:    "agent:work:main",
		TargetChannel:       "internal",
		OriginalMessage:     "Check my calendar for tomorrow",
		RoundOneReply:       "Found 3 events with 1 conflict at 2pm",
		LatestReply:         "The 2pm 1:1 with Alice is easiest to reschedule",
	}

	prompt := BuildA2AAnnounceContext(params)

	// Verify prompt contains key elements
	requiredElements := []string{
		"Agent-to-agent announce step:",
		"Agent 1 (requester) session: agent:main:main.",
		"Agent 1 (requester) channel: whatsapp.",
		"Agent 2 (target) session: agent:work:main.",
		"Agent 2 (target) channel: internal.",
		"Conversation summary:",
		"Original request: Check my calendar for tomorrow",
		"Round 1 reply: Found 3 events with 1 conflict at 2pm",
		"Latest reply: The 2pm 1:1 with Alice is easiest to reschedule",
		"Instructions:",
		"Format a final announcement",
		"ANNOUNCE_SKIP",
	}

	for _, element := range requiredElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("Announce context prompt missing required element: '%s'", element)
		}
	}

	t.Logf("Generated announce context prompt:\n%s", prompt)
}

// TestBuildA2AAnnounceContext_NoChannels tests announcement without channels
func TestBuildA2AAnnounceContext_NoChannels(t *testing.T) {
	params := shared.A2AAnnounceContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		OriginalMessage:     "Query data",
		RoundOneReply:       "Found 50 rows",
		LatestReply:         "Found 50 rows",
	}

	prompt := BuildA2AAnnounceContext(params)

	// Should not mention channels
	if strings.Contains(prompt, "channel:") {
		t.Error("Prompt should not mention channels when not provided")
	}

	// Should still have conversation summary
	if !strings.Contains(prompt, "Original request: Query data") {
		t.Error("Prompt should contain original request")
	}

	t.Logf("No channels announce prompt:\n%s", prompt)
}

// TestBuildA2AAnnounceContext_SameReply tests announcement when replies didn't change
func TestBuildA2AAnnounceContext_SameReply(t *testing.T) {
	params := shared.A2AAnnounceContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		OriginalMessage:     "Check status",
		RoundOneReply:       "System is healthy",
		LatestReply:         "System is healthy", // Same as round 1
	}

	prompt := BuildA2AAnnounceContext(params)

	// Should contain round 1 reply
	if !strings.Contains(prompt, "Round 1 reply: System is healthy") {
		t.Error("Prompt should contain round 1 reply")
	}

	// Should NOT contain "Latest reply" since it's the same
	if strings.Contains(prompt, "Latest reply:") {
		t.Error("Prompt should not show 'Latest reply' when it's same as round 1")
	}

	t.Logf("Same reply prompt:\n%s", prompt)
}

// TestBuildA2AAnnounceContext_LongMessages tests announcement with long messages
func TestBuildA2AAnnounceContext_LongMessages(t *testing.T) {
	longOriginal := strings.Repeat("This is a very long original message that should be truncated. ", 10) // ~640 chars
	longReply := strings.Repeat("This is a very long reply that should also be truncated. ", 10)          // ~580 chars

	params := shared.A2AAnnounceContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		OriginalMessage:     longOriginal,
		RoundOneReply:       longReply,
		LatestReply:         longReply,
	}

	prompt := BuildA2AAnnounceContext(params)

	// Should contain truncated messages (max 200 chars each)
	if !strings.Contains(prompt, "Original request:") {
		t.Error("Prompt should contain original request label")
	}

	if !strings.Contains(prompt, "Round 1 reply:") {
		t.Error("Prompt should contain round 1 reply label")
	}

	// Verify truncation with ellipsis
	if !strings.Contains(prompt, "...") {
		t.Error("Long messages should be truncated with ellipsis")
	}

	t.Logf("Long messages prompt:\n%s", prompt)
}

// TestBuildA2AAnnounceContext_EmptyReplies tests announcement with empty replies
func TestBuildA2AAnnounceContext_EmptyReplies(t *testing.T) {
	params := shared.A2AAnnounceContextParams{
		RequesterSessionKey: "agent:main:main",
		TargetSessionKey:    "agent:work:main",
		OriginalMessage:     "Test message",
		RoundOneReply:       "",
		LatestReply:         "",
	}

	prompt := BuildA2AAnnounceContext(params)

	// Should still have basic structure
	if !strings.Contains(prompt, "Original request: Test message") {
		t.Error("Prompt should contain original message even if replies are empty")
	}

	if !strings.Contains(prompt, "Instructions:") {
		t.Error("Prompt should contain instructions")
	}

	t.Logf("Empty replies prompt:\n%s", prompt)
}
