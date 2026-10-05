package sandbox

import (
	"fmt"

	"github.com/google/uuid"
)

// SandboxContext contains all security context for agent execution
type SandboxContext struct {
	// Tenant isolation
	WorkspaceID   uuid.UUID // Which company (e.g., Acme Corp)
	WorkspaceSlug string    // workspace slug (e.g. "memdoor")
	ChannelID     uuid.UUID // Which Slack channel (e.g., #engineering)

	// User identity (NEVER changes during A2A chain)
	InitiatingUserID uuid.UUID // Who started the request (e.g., Alice)

	// Agent identity
	CurrentAgentID string // Current agent in chain (e.g., agent:writer)

	// Sandbox scope (determines access level)
	AgentScope SandboxScope // user/channel/workspace
}

// GetSandboxPath returns the filesystem root for this agent based on scope
func (ctx SandboxContext) GetSandboxPath() string {
	switch ctx.AgentScope {
	case ScopeUser:
		return fmt.Sprintf("/sandbox/user/%s", ctx.InitiatingUserID)
	case ScopeChannel:
		return fmt.Sprintf("/sandbox/channel/%s", ctx.ChannelID)
	case ScopeWorkspace:
		return fmt.Sprintf("/sandbox/workspace/%s", ctx.WorkspaceID)
	default:
		// Fallback to most restrictive
		return fmt.Sprintf("/sandbox/user/%s", ctx.InitiatingUserID)
	}
}

// Validate checks if the context is valid
func (ctx SandboxContext) Validate() error {
	if ctx.WorkspaceID == uuid.Nil {
		return fmt.Errorf("workspace_id is required")
	}
	if ctx.ChannelID == uuid.Nil {
		return fmt.Errorf("channel_id is required")
	}
	if ctx.InitiatingUserID == uuid.Nil {
		return fmt.Errorf("initiating_user_id is required")
	}
	if ctx.CurrentAgentID == "" {
		return fmt.Errorf("current_agent_id is required")
	}
	if !ctx.AgentScope.IsValid() {
		return ErrInvalidScope
	}
	return nil
}

// GetAllowedPathsDescription returns a human-readable description of allowed paths
func (ctx SandboxContext) GetAllowedPathsDescription() string {
	switch ctx.AgentScope {
	case ScopeUser:
		return fmt.Sprintf("- /user/%s/   (your personal data only)", ctx.InitiatingUserID)
	case ScopeChannel:
		return fmt.Sprintf("- /user/%s/   (your personal data)\n- /channel/%s/   (channel shared data)",
			ctx.InitiatingUserID, ctx.ChannelID)
	case ScopeWorkspace:
		return fmt.Sprintf("- /user/%s/   (your personal data)\n- /channel/%s/   (channel shared data)\n- /workspace/%s/   (workspace-wide data)",
			ctx.InitiatingUserID, ctx.ChannelID, ctx.WorkspaceID)
	default:
		return "Unknown scope"
	}
}
