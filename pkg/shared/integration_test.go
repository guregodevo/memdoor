package shared

import "testing"

// These tests verify that the shared kernel maintains model integrity
// across both bounded contexts (Team Collaboration and Multi-Agent Platform)

func TestActorID_SharedByBothContexts(t *testing.T) {
	// Test that Team Collaboration context can use ActorID
	t.Run("team collaboration usage", func(t *testing.T) {
		// Human member posts message
		humanID := NewHumanActorID("user-123")
		if !humanID.IsHuman() {
			t.Errorf("IsHuman() = false, want true")
		}

		// Agent member responds
		agentID := NewAgentActorID("agent-456")
		if !agentID.IsAgent() {
			t.Errorf("IsAgent() = false, want true")
		}

		// Both are valid actors
		if err := humanID.Validate(); err != nil {
			t.Errorf("human ActorID should be valid: %v", err)
		}
		if err := agentID.Validate(); err != nil {
			t.Errorf("agent ActorID should be valid: %v", err)
		}
	})

	// Test that Multi-Agent Platform context can use ActorID
	t.Run("multi-agent platform usage", func(t *testing.T) {
		// Platform assigns execution to agent
		agentID := NewAgentActorID("agent-789")

		// Extract underlying agent ID for platform use
		platformAgentID := agentID.ID()
		if platformAgentID != "agent-789" {
			t.Errorf("ID() = %v, want 'agent-789'", platformAgentID)
		}

		// Platform checks if actor is agent
		if !agentID.IsAgent() {
			t.Errorf("Platform should recognize agent actors")
		}
	})

	// Test that both contexts agree on ActorID semantics
	t.Run("cross-context consistency", func(t *testing.T) {
		// Team Collaboration creates ActorID
		teamActorID := NewAgentActorID("shared-agent-1")

		// Multi-Agent Platform receives same ActorID
		platformActorID := ActorID(teamActorID.String())

		// Both contexts see same type
		if teamActorID.Type() != platformActorID.Type() {
			t.Errorf("Both contexts must agree on actor type")
		}

		// Both contexts extract same ID
		if teamActorID.ID() != platformActorID.ID() {
			t.Errorf("Both contexts must extract same underlying ID")
		}

		// String representation is identical
		if teamActorID.String() != platformActorID.String() {
			t.Errorf("String representation must be consistent across contexts")
		}
	})
}

func TestActorID_TaggedUnionInvariant(t *testing.T) {
	// Verify the tagged union invariant is maintained
	t.Run("human and agent are mutually exclusive", func(t *testing.T) {
		human := NewHumanActorID("user-1")
		agent := NewAgentActorID("agent-1")

		// A human actor is not an agent
		if human.IsAgent() {
			t.Errorf("Human actor should not be an agent")
		}

		// An agent actor is not a human
		if agent.IsHuman() {
			t.Errorf("Agent actor should not be a human")
		}

		// Each has exactly one type
		if human.Type() != ActorTypeHuman {
			t.Errorf("Human actor must have type 'human'")
		}
		if agent.Type() != ActorTypeAgent {
			t.Errorf("Agent actor must have type 'agent'")
		}
	})
}

func TestActorID_ValidationRules(t *testing.T) {
	tests := []struct {
		name      string
		actorID   ActorID
		wantValid bool
	}{
		{
			name:      "valid human",
			actorID:   NewHumanActorID("uuid-123"),
			wantValid: true,
		},
		{
			name:      "valid agent",
			actorID:   NewAgentActorID("uuid-456"),
			wantValid: true,
		},
		{
			name:      "empty string invalid",
			actorID:   ActorID(""),
			wantValid: false,
		},
		{
			name:      "missing colon invalid",
			actorID:   ActorID("humanuser123"),
			wantValid: false,
		},
		{
			name:      "unknown type invalid",
			actorID:   ActorID("bot:123"),
			wantValid: false,
		},
		{
			name:      "empty ID after colon invalid",
			actorID:   ActorID("human:"),
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.actorID.Validate()
			isValid := err == nil

			if isValid != tt.wantValid {
				t.Errorf("Validate() isValid = %v, want %v (error: %v)",
					isValid, tt.wantValid, err)
			}
		})
	}
}
