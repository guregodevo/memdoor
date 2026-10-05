package message

import (
	"testing"

	"memdoor/pkg/shared"
)

func TestActorID(t *testing.T) {
	t.Run("human actor ID", func(t *testing.T) {
		id := shared.NewHumanActorID("user-123")

		if !id.IsHuman() {
			t.Errorf("IsHuman() = false, want true")
		}

		if id.IsAgent() {
			t.Errorf("IsAgent() = true, want false")
		}

		if id.Type() != shared.ActorTypeHuman {
			t.Errorf("Type() = %v, want %v", id.Type(), shared.ActorTypeHuman)
		}

		if id.ID() != "user-123" {
			t.Errorf("ID() = %v, want user-123", id.ID())
		}

		if err := id.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("agent actor ID", func(t *testing.T) {
		id := shared.NewAgentActorID("agent-456")

		if id.IsHuman() {
			t.Errorf("IsHuman() = true, want false")
		}

		if !id.IsAgent() {
			t.Errorf("IsAgent() = false, want true")
		}

		if id.Type() != shared.ActorTypeAgent {
			t.Errorf("Type() = %v, want %v", id.Type(), shared.ActorTypeAgent)
		}

		if id.ID() != "agent-456" {
			t.Errorf("ID() = %v, want agent-456", id.ID())
		}

		if err := id.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("invalid actor ID format", func(t *testing.T) {
		id := shared.ActorID("invalid")

		if err := id.Validate(); err == nil {
			t.Errorf("Validate() expected error for invalid format")
		}
	})

	t.Run("invalid actor type", func(t *testing.T) {
		id := shared.ActorID("bot:123")

		if err := id.Validate(); err == nil {
			t.Errorf("Validate() expected error for invalid type")
		}
	})
}

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single mention",
			text:     "@john can you review this?",
			expected: []string{"john"},
		},
		{
			name:     "multiple mentions",
			text:     "@john @mary please discuss with @bob",
			expected: []string{"john", "mary", "bob"},
		},
		{
			name:     "duplicate mentions",
			text:     "@john @john @john",
			expected: []string{"john"}, // De-duplicated
		},
		{
			name:     "no mentions",
			text:     "hello world",
			expected: []string{},
		},
		{
			name:     "mention with hyphen",
			text:     "@marketing-buddy write a post",
			expected: []string{"marketing-buddy"},
		},
		{
			name:     "mention with underscore",
			text:     "@dev_agent debug this",
			expected: []string{"dev_agent"},
		},
		{
			name:     "mention at start",
			text:     "@agent help me",
			expected: []string{"agent"},
		},
		{
			name:     "mention at end",
			text:     "thanks @agent",
			expected: []string{"agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentions := ParseMentions(tt.text)

			if len(mentions) != len(tt.expected) {
				t.Errorf("ParseMentions() = %v, want %v", mentions, tt.expected)
				return
			}

			for i, mention := range mentions {
				if mention != tt.expected[i] {
					t.Errorf("ParseMentions()[%d] = %v, want %v", i, mention, tt.expected[i])
				}
			}
		})
	}
}
