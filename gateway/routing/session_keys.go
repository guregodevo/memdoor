package routing

import (
	"fmt"
	"regexp"
	"strings"
)

// Session key parsing and normalization following OpenClaw's pattern
// Pattern: OpenClaw src/routing/session-key.ts
//
// This package handles session key routing and agent resolution,
// maintaining clean separation of concerns following OpenClaw's architecture.

const (
	// DefaultAgentID is the default agent identifier
	DefaultAgentID = "main"

	// Maximum length for agent IDs
	MaxAgentIDLength = 64
)

// Regular expressions for agent ID validation
var (
	// ValidIDRegex matches valid agent IDs: alphanumeric + dash + underscore
	ValidIDRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

	// InvalidCharsRegex matches invalid characters that should be replaced
	InvalidCharsRegex = regexp.MustCompile(`[^a-z0-9_-]`)

	// LeadingDashRegex matches leading dashes
	LeadingDashRegex = regexp.MustCompile(`^-+`)

	// TrailingDashRegex matches trailing dashes
	TrailingDashRegex = regexp.MustCompile(`-+$`)
)

// ParsedSessionKey represents a parsed agent session key
// Format: agent:<agent-id>:<session-type>[:<extra>]
//
// Examples:
//   - agent:main:main              → {AgentID: "main", Rest: "main"}
//   - agent:researcher:main        → {AgentID: "researcher", Rest: "main"}
//   - agent:data-analyst:cron:daily → {AgentID: "data-analyst", Rest: "cron:daily"}
type ParsedSessionKey struct {
	AgentID string // The normalized agent identifier
	Rest    string // The remaining session key parts (e.g., "main", "cron:job-id")
}

// ParseAgentSessionKey parses a session key in the format:
// agent:<agent-id>:<session-type>[:<extra>]
//
// Pattern: OpenClaw src/routing/session-key.ts:6-26
//
// Returns nil if the session key is invalid or doesn't follow the agent format.
//
// Examples:
//
//	ParseAgentSessionKey("agent:main:main")           → {AgentID: "main", Rest: "main"}
//	ParseAgentSessionKey("agent:researcher:main")     → {AgentID: "researcher", Rest: "main"}
//	ParseAgentSessionKey("agent:monitor:cron:health") → {AgentID: "monitor", Rest: "cron:health"}
//	ParseAgentSessionKey("invalid")                   → nil
//	ParseAgentSessionKey("")                          → nil
func ParseAgentSessionKey(sessionKey string) *ParsedSessionKey {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return nil
	}

	// Split by colons
	parts := strings.Split(raw, ":")
	// Filter out empty parts
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	// Minimum: agent:id:session (3 parts)
	if len(filtered) < 3 {
		return nil
	}

	// First part must be "agent"
	if filtered[0] != "agent" {
		return nil
	}

	// Extract agent ID and rest
	agentID := filtered[1]
	rest := strings.Join(filtered[2:], ":")

	return &ParsedSessionKey{
		AgentID: agentID,
		Rest:    rest,
	}
}

// NormalizeAgentID normalizes an agent ID to a valid format
// Pattern: OpenClaw src/routing/session-key.ts:73-91
//
// Rules:
//   - Convert to lowercase
//   - Only alphanumeric, dash, and underscore allowed
//   - Replace invalid characters with "-"
//   - Remove leading/trailing dashes
//   - Maximum 64 characters
//   - Default to "main" if empty or invalid
//
// Examples:
//
//	NormalizeAgentID("Researcher")     → "researcher"
//	NormalizeAgentID("Data Analyst")   → "data-analyst"
//	NormalizeAgentID("monitor_01")     → "monitor_01"
//	NormalizeAgentID("---test---")     → "test"
//	NormalizeAgentID("")               → "main"
func NormalizeAgentID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return DefaultAgentID
	}

	// Convert to lowercase
	normalized := strings.ToLower(trimmed)

	// Clean up: replace invalid chars with "-"
	normalized = InvalidCharsRegex.ReplaceAllString(normalized, "-")

	// Remove leading dashes
	normalized = LeadingDashRegex.ReplaceAllString(normalized, "")

	// Remove trailing dashes
	normalized = TrailingDashRegex.ReplaceAllString(normalized, "")

	// Apply length limit
	if len(normalized) > MaxAgentIDLength {
		normalized = normalized[:MaxAgentIDLength]
	}

	// Remove trailing dashes again after truncation
	normalized = TrailingDashRegex.ReplaceAllString(normalized, "")

	// If empty after cleanup, use default
	if normalized == "" {
		return DefaultAgentID
	}

	return normalized
}

// BuildAgentSessionKey builds a session key from components
// Pattern: OpenClaw session key format
//
// Examples:
//
//	BuildAgentSessionKey("main", "main")           → "agent:main:main"
//	BuildAgentSessionKey("researcher", "main")     → "agent:researcher:main"
//	BuildAgentSessionKey("monitor", "cron:health") → "agent:monitor:cron:health"
func BuildAgentSessionKey(agentID, sessionType string) string {
	normalizedAgentID := NormalizeAgentID(agentID)
	return fmt.Sprintf("agent:%s:%s", normalizedAgentID, sessionType)
}

// BuildAgentMainSessionKey builds a main session key for an agent
// Convenience function for the common case of main sessions
//
// Example:
//
//	BuildAgentMainSessionKey("researcher") → "agent:researcher:main"
func BuildAgentMainSessionKey(agentID string) string {
	return BuildAgentSessionKey(agentID, "main")
}

// BuildAgentCronSessionKey builds a cron session key for an agent
// Pattern: OpenClaw cron session keys
//
// Example:
//
//	BuildAgentCronSessionKey("monitor", "health-check") → "agent:monitor:cron:health-check"
func BuildAgentCronSessionKey(agentID, cronJobID string) string {
	return BuildAgentSessionKey(agentID, fmt.Sprintf("cron:%s", cronJobID))
}

// ResolveSessionAgentID extracts the agent ID from a session key
// Returns the default agent ID if the session key is invalid or not in agent format
//
// Example:
//
//	ResolveSessionAgentID("agent:researcher:main")  → "researcher"
//	ResolveSessionAgentID("agent:main:main")        → "main"
//	ResolveSessionAgentID("legacy-session")         → "main" (default)
//	ResolveSessionAgentID("")                       → "main" (default)
func ResolveSessionAgentID(sessionKey string) string {
	parsed := ParseAgentSessionKey(sessionKey)
	if parsed == nil {
		return DefaultAgentID
	}
	return NormalizeAgentID(parsed.AgentID)
}
