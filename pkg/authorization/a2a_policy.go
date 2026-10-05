package authorization

import (
	"fmt"
)

// A2APolicy defines the agent-to-agent communication policy
// This is domain logic - authorization rules for which agents can communicate
// Pattern: OpenClaw src/agents/tool-policy.ts
type A2APolicy struct {
	Enabled          bool      `json:"enabled"`
	Allow            []A2ARule `json:"allow"`
	MaxPingPongTurns int       `json:"maxPingPongTurns"` // Week 28: multi-turn conversations
}

// A2ARule defines a permission rule for agent-to-agent messaging
type A2ARule struct {
	From string `json:"from"` // Source agent ID or "*" for wildcard
	To   string `json:"to"`   // Target agent ID or "*" for wildcard
}

// DefaultA2APolicy returns the default policy (deny all)
func DefaultA2APolicy() *A2APolicy {
	// Local single-user product: the seeded agents collaborate (one hands work
	// to another) so A2A is on by default. Fire-and-forget
	// (MaxPingPongTurns=0) bounds it —
	// the spawned agent runs once and can't ping-pong back into a loop. Deployments
	// that want a stricter roster set tools.agentToAgent.allow in config, which
	// replaces this default (see loadA2APolicy).
	return &A2APolicy{
		Enabled:          true,
		Allow:            []A2ARule{{From: "*", To: "*"}},
		MaxPingPongTurns: 0,
	}
}

// IsAllowed checks if agent-to-agent messaging is allowed from source to target
// This is a domain business rule - authorization logic
func (p *A2APolicy) IsAllowed(fromAgentID, toAgentID string) bool {
	// If A2A is disabled globally, deny
	if !p.Enabled {
		return false
	}

	// If no rules defined, deny (default deny)
	if len(p.Allow) == 0 {
		return false
	}

	// Check each rule
	for _, rule := range p.Allow {
		if p.ruleMatches(rule, fromAgentID, toAgentID) {
			return true
		}
	}

	// No matching rule found, deny
	return false
}

// ruleMatches checks if a rule matches the from/to agent pair
func (p *A2APolicy) ruleMatches(rule A2ARule, fromAgentID, toAgentID string) bool {
	// Check "from" field
	fromMatches := rule.From == "*" || rule.From == fromAgentID

	// Check "to" field
	toMatches := rule.To == "*" || rule.To == toAgentID

	return fromMatches && toMatches
}

// Validate checks if the policy configuration is valid
// Domain invariant - ensures policy is well-formed
func (p *A2APolicy) Validate() error {
	// Check for invalid agent IDs in rules
	for i, rule := range p.Allow {
		if rule.From == "" {
			return fmt.Errorf("rule %d: 'from' field cannot be empty", i)
		}
		if rule.To == "" {
			return fmt.Errorf("rule %d: 'to' field cannot be empty", i)
		}

		// Warn about overly permissive rules (both wildcards)
		if rule.From == "*" && rule.To == "*" {
			// This is valid but potentially dangerous
			// Just document it, don't reject
		}
	}

	// Check MaxPingPongTurns is reasonable
	if p.MaxPingPongTurns < 0 {
		return fmt.Errorf("maxPingPongTurns cannot be negative: %d", p.MaxPingPongTurns)
	}

	if p.MaxPingPongTurns > 10 {
		return fmt.Errorf("maxPingPongTurns too high (max 10): %d", p.MaxPingPongTurns)
	}

	return nil
}

// GetMaxPingPongTurns returns the maximum number of ping-pong turns
func (p *A2APolicy) GetMaxPingPongTurns() int {
	return p.MaxPingPongTurns
}

// String returns a human-readable description of the policy
func (p *A2APolicy) String() string {
	if !p.Enabled {
		return "A2A Policy: DISABLED"
	}

	if len(p.Allow) == 0 {
		return "A2A Policy: ENABLED but no rules (deny all)"
	}

	return fmt.Sprintf("A2A Policy: ENABLED with %d rule(s)", len(p.Allow))
}
