package shared

import (
	"fmt"
	"strings"
)

// ActorID represents a unified identifier for actors (humans and agents)
// SHARED KERNEL: Used by both Team Collaboration and Multi-Agent Platform contexts
//
// Tagged union: "human:<uuid>" or "agent:<uuid>"
// This provides a unified way to identify who performed an action without
// requiring a full actor hierarchy
type ActorID string

// ActorType represents the type of actor
type ActorType string

const (
	ActorTypeHuman ActorType = "human"
	ActorTypeAgent ActorType = "agent"
)

// NewHumanActorID creates an actor ID for a human identity
func NewHumanActorID(identityID string) ActorID {
	return ActorID(fmt.Sprintf("human:%s", identityID))
}

// NewAgentActorID creates an actor ID for an AI agent
func NewAgentActorID(agentID string) ActorID {
	return ActorID(fmt.Sprintf("agent:%s", agentID))
}

// Type returns the actor type (human or agent)
func (id ActorID) Type() ActorType {
	parts := strings.SplitN(string(id), ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return ActorType(parts[0])
}

// ID returns the underlying ID (UUID)
func (id ActorID) ID() string {
	parts := strings.SplitN(string(id), ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// IsHuman returns true if this is a human actor
func (id ActorID) IsHuman() bool {
	return id.Type() == ActorTypeHuman
}

// IsAgent returns true if this is an agent actor
func (id ActorID) IsAgent() bool {
	return id.Type() == ActorTypeAgent
}

// Validate checks if the actor ID is valid
func (id ActorID) Validate() error {
	if id == "" {
		return fmt.Errorf("actor ID cannot be empty")
	}

	parts := strings.SplitN(string(id), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid actor ID format: %s (expected type:id)", id)
	}

	actorType := ActorType(parts[0])
	if actorType != ActorTypeHuman && actorType != ActorTypeAgent {
		return fmt.Errorf("invalid actor type: %s (expected human or agent)", actorType)
	}

	if parts[1] == "" {
		return fmt.Errorf("actor ID cannot be empty after type prefix")
	}

	return nil
}

// String returns the string representation
func (id ActorID) String() string {
	return string(id)
}
