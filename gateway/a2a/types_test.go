package a2a

import (
	"testing"

	"memdoor/pkg/shared"
)

// TestIsReplySkip tests the REPLY_SKIP detection
func TestIsReplySkip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "exact match",
			input:    "REPLY_SKIP",
			expected: true,
		},
		{
			name:     "with leading whitespace",
			input:    "  REPLY_SKIP",
			expected: true,
		},
		{
			name:     "with trailing whitespace",
			input:    "REPLY_SKIP  ",
			expected: true,
		},
		{
			name:     "with both whitespace",
			input:    "  REPLY_SKIP  ",
			expected: true,
		},
		{
			name:     "with newlines",
			input:    "\nREPLY_SKIP\n",
			expected: true,
		},
		{
			name:     "wrong case",
			input:    "reply_skip",
			expected: false,
		},
		{
			name:     "partial match",
			input:    "REPLY_SKIP_NOW",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "different token",
			input:    "ANNOUNCE_SKIP",
			expected: false,
		},
		{
			name:     "with extra text",
			input:    "I want to REPLY_SKIP",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.IsReplySkip(tt.input)
			if result != tt.expected {
				t.Errorf("IsReplySkip(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsAnnounceSkip tests the ANNOUNCE_SKIP detection
func TestIsAnnounceSkip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "exact match",
			input:    "ANNOUNCE_SKIP",
			expected: true,
		},
		{
			name:     "with leading whitespace",
			input:    "  ANNOUNCE_SKIP",
			expected: true,
		},
		{
			name:     "with trailing whitespace",
			input:    "ANNOUNCE_SKIP  ",
			expected: true,
		},
		{
			name:     "with both whitespace",
			input:    "  ANNOUNCE_SKIP  ",
			expected: true,
		},
		{
			name:     "with newlines",
			input:    "\nANNOUNCE_SKIP\n",
			expected: true,
		},
		{
			name:     "wrong case",
			input:    "announce_skip",
			expected: false,
		},
		{
			name:     "partial match",
			input:    "ANNOUNCE_SKIP_THIS",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "different token",
			input:    "REPLY_SKIP",
			expected: false,
		},
		{
			name:     "with extra text",
			input:    "I want to ANNOUNCE_SKIP",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.IsAnnounceSkip(tt.input)
			if result != tt.expected {
				t.Errorf("IsAnnounceSkip(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConversationHistory tests the ConversationHistory struct
func TestConversationHistory(t *testing.T) {
	history := &shared.ConversationHistory{
		OriginalMessage: "Check my calendar",
		RoundOneReply:   "Found 3 events",
		LatestReply:     "The 2pm meeting can be moved",
		TurnCount:       3,
	}

	if history.OriginalMessage != "Check my calendar" {
		t.Errorf("OriginalMessage = %q, want %q", history.OriginalMessage, "Check my calendar")
	}
	if history.RoundOneReply != "Found 3 events" {
		t.Errorf("RoundOneReply = %q, want %q", history.RoundOneReply, "Found 3 events")
	}
	if history.LatestReply != "The 2pm meeting can be moved" {
		t.Errorf("LatestReply = %q, want %q", history.LatestReply, "The 2pm meeting can be moved")
	}
	if history.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want %d", history.TurnCount, 3)
	}
}

// TestPingPongState tests the PingPongState struct
func TestPingPongState(t *testing.T) {
	history := &shared.ConversationHistory{
		OriginalMessage: "Test message",
		RoundOneReply:   "Initial reply",
		LatestReply:     "Current reply",
		TurnCount:       2,
	}

	state := &shared.PingPongState{
		CurrentSessionKey: "agent:main:session1",
		NextSessionKey:    "agent:work:session2",
		IncomingMessage:   "Follow-up question",
		History:           history,
		Turn:              2,
		MaxTurns:          5,
	}

	if state.CurrentSessionKey != "agent:main:session1" {
		t.Errorf("CurrentSessionKey = %q, want %q", state.CurrentSessionKey, "agent:main:session1")
	}
	if state.NextSessionKey != "agent:work:session2" {
		t.Errorf("NextSessionKey = %q, want %q", state.NextSessionKey, "agent:work:session2")
	}
	if state.Turn != 2 {
		t.Errorf("Turn = %d, want %d", state.Turn, 2)
	}
	if state.MaxTurns != 5 {
		t.Errorf("MaxTurns = %d, want %d", state.MaxTurns, 5)
	}
	if state.History.TurnCount != 2 {
		t.Errorf("History.TurnCount = %d, want %d", state.History.TurnCount, 2)
	}
}
