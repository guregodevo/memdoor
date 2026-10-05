package sandbox

// SandboxScope defines what data an agent can access
type SandboxScope string

const (
	// ScopeUser - Most secure, agent only accesses initiating user's data
	// - Filesystem: /user/{initiating_user_id}/
	// - Credentials: Only initiating user's OAuth tokens
	// - Use case: Personal assistant, Gmail agent, calendar agent
	ScopeUser SandboxScope = "user"

	// ScopeChannel - Agent accesses channel-shared data
	// - Filesystem: /channel/{channel_id}/
	// - Credentials: Channel-scoped credentials (e.g., shared Slack bot token)
	// - Use case: Team knowledge base, channel bot
	ScopeChannel SandboxScope = "channel"

	// ScopeWorkspace - Agent accesses workspace-wide data
	// - Filesystem: /workspace/{workspace_id}/
	// - Credentials: Workspace-scoped credentials (e.g., company API keys)
	// - Use case: Company-wide data analysis, shared tools
	ScopeWorkspace SandboxScope = "workspace"
)

// IsValid returns true if the scope is valid
func (s SandboxScope) IsValid() bool {
	switch s {
	case ScopeUser, ScopeChannel, ScopeWorkspace:
		return true
	default:
		return false
	}
}

// Allows returns true if this scope allows access to the required scope
// Higher scopes can access lower scope data:
// - Workspace can access channel and user data
// - Channel can access user data
// - User can only access user data
func (s SandboxScope) Allows(requiredScope SandboxScope) bool {
	scopeLevel := map[SandboxScope]int{
		ScopeUser:      1,
		ScopeChannel:   2,
		ScopeWorkspace: 3,
	}
	return scopeLevel[s] >= scopeLevel[requiredScope]
}

// String returns the string representation of the scope
func (s SandboxScope) String() string {
	return string(s)
}
