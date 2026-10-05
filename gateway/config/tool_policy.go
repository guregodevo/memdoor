package config

// Tool policy configuration following OpenClaw's pattern
// Pattern: OpenClaw src/agents/tool-policy.ts

// ToolProfileID represents a predefined tool configuration profile
type ToolProfileID string

const (
	ToolProfileMinimal   ToolProfileID = "minimal"
	ToolProfileChat      ToolProfileID = "chat"
	ToolProfileDefault   ToolProfileID = "default"
	ToolProfileCoding    ToolProfileID = "coding"
	ToolProfileMessaging ToolProfileID = "messaging"
	ToolProfileFull      ToolProfileID = "full"
	DefaultToolProfile                 = ToolProfileChat
)

// TOOL_GROUPS defines logical groupings of tools
// Pattern: OpenClaw TOOL_GROUPS
var TOOL_GROUPS = map[string][]string{
	// Cross-conversation memory store. Kept as its own group so the
	// default tool profiles do NOT inject it automatically — agents
	// that genuinely need persistent memory must opt in by listing
	// "memory" or "group:memory" in their Tools config. This avoids
	// the contamination surface where memory replays "I already
	// answered this" reasoning across user turns and bypasses fresh
	// tool calls (same class of bug as session JSONL contamination).
	"group:memory": {"memory"},

	// Secrets — bundled separately because they don't carry the
	// contamination risk (get_secret returns a value).
	"group:secrets": {"get_secret"},

	// Web tools
	"group:web": {"web_fetch", "web_search"},

	// Read-only file tools
	"group:fs_read": {
		"read_file",
		"list_files",
		"glob",
		"grep",
	},

	// Write file tools (for coder agents)
	"group:fs_write": {
		"write_file",
		"edit_file",
		"search_replace",
	},

	// All file tools
	"group:fs": {
		"read_file",
		"write_file",
		"edit_file",
		"list_files",
		"glob",
		"grep",
		"search_replace",
	},

	// Host/runtime execution tools
	"group:runtime": {"bash"},

	// Session management tools
	"group:sessions": {
		"sessions_list",
		"sessions_history",
		"sessions_send",
		"session_status",
	},

	// Session spawning (for coding/advanced agents)
	"group:sessions_spawn": {"sessions_spawn"},

	// UI helpers
	"group:ui": {"chrome_devtools"},

	// Automation + infra
	"group:automation": {"cron", "channels", "send_invite"},

	// Productivity tools
	"group:productivity": {
		"ask_user_question",
		"todo_write",
	},

	// Context management
	"group:context": {"context", "status"},

	// Introspection
	"group:introspection": {"agents_list", "gateway", "agent_log", "execution_flow"},
}

// TOOL_PROFILES defines predefined tool configurations for common use cases
// Pattern: OpenClaw TOOL_PROFILES
var TOOL_PROFILES = map[ToolProfileID]ToolProfilePolicy{
	ToolProfileMinimal: {
		Allow: []string{"session_status"},
	},
	// Chat profile: for fan-facing creator agents (conversations, web,
	// secrets, RAG retrieval). Does NOT include the cross-conversation
	// memory tool — agents that need persistent memory must opt in
	// explicitly via Tools config. This keeps stateless retrievers free of
	// memory contamination.
	ToolProfileChat: {
		Allow: []string{
			"group:web",
			"group:secrets",
			"group:context",
			"channels",
			"send_email",
		},
	},
	// Default profile: general-purpose agents with read access. Same
	// rule as chat — memory is opt-in only.
	ToolProfileDefault: {
		Allow: []string{
			"group:fs_read",
			"group:sessions",
			"group:secrets",
			"group:context",
			"group:web",
			"ask_user_question",
			"send_invite",
			"send_email",
			"channels",
		},
	},
	// Coding profile: coder agents with full filesystem + runtime access.
	// Memory is opt-in here too — coding agents reason about the codebase
	// in front of them, not about cross-session state.
	ToolProfileCoding: {
		Allow: []string{
			"group:fs",
			"group:runtime",
			"group:sessions",
			"group:sessions_spawn",
			"group:secrets",
			"group:context",
			"group:web",
			"group:introspection",
			"chrome_devtools",
			"ask_user_question",
		},
	},
	ToolProfileMessaging: {
		Allow: []string{
			"sessions_list",
			"sessions_history",
			"sessions_send",
			"session_status",
			"group:context",
		},
	},
	ToolProfileFull: {
		// Empty allow list = all tools
	},
}

// ToolProfilePolicy represents an allow/deny policy for a profile
type ToolProfilePolicy struct {
	Allow []string
	Deny  []string
}

// NormalizeToolName normalizes a tool name to lowercase
func NormalizeToolName(name string) string {
	// Tool name aliases (if any)
	aliases := map[string]string{
		"exec": "bash",
	}

	normalized := name
	if alias, ok := aliases[name]; ok {
		normalized = alias
	}

	return normalized
}

// ExpandToolGroups expands tool group references into individual tool names
// Pattern: OpenClaw expandToolGroups
func ExpandToolGroups(list []string) []string {
	if list == nil {
		return nil
	}

	expanded := make([]string, 0)
	seen := make(map[string]bool)

	for _, value := range list {
		normalized := NormalizeToolName(value)

		// Check if it's a group
		if group, ok := TOOL_GROUPS[normalized]; ok {
			for _, tool := range group {
				if !seen[tool] {
					expanded = append(expanded, tool)
					seen[tool] = true
				}
			}
			continue
		}

		// Regular tool name
		if !seen[normalized] {
			expanded = append(expanded, normalized)
			seen[normalized] = true
		}
	}

	return expanded
}

// ResolveToolProfilePolicy resolves a profile ID to its policy
// Pattern: OpenClaw resolveToolProfilePolicy
func ResolveToolProfilePolicy(profile string) *ToolProfilePolicy {
	if profile == "" {
		return nil
	}

	profileID := ToolProfileID(profile)
	policy, ok := TOOL_PROFILES[profileID]
	if !ok {
		return nil
	}

	// Return a copy to avoid mutations
	return &ToolProfilePolicy{
		Allow: append([]string{}, policy.Allow...),
		Deny:  append([]string{}, policy.Deny...),
	}
}
