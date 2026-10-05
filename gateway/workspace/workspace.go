package workspace

import (
	"strings"

	"memdoor/gateway/config"
	"memdoor/gateway/routing"
)

// Workspace management following OpenClaw's pattern
// Pattern: OpenClaw src/agents/workspace.ts
//
// This package handles per-agent workspace directories,
// path resolution, and workspace bootstrapping.

const (
	// DefaultWorkspaceName is the default workspace subdirectory
	DefaultWorkspaceName = "workspace"
)

// Bootstrap file names (OpenClaw pattern)
const (
	AgentsFileName    = "AGENTS.md"
	SoulFileName      = "SOUL.md"
	ToolsFileName     = "TOOLS.md"
	IdentityFileName  = "IDENTITY.md"
	UserFileName      = "USER.md"
	BootstrapFileName = "BOOTSTRAP.md"
	MemoryFileName    = "MEMORY.md"
)

// TemplateVariableExpansion replaces template variables in paths
// Pattern: OpenClaw src/config/sessions/paths.ts template expansion
//
// Supported variables:
//   - {agentId} - Replaced with agent ID
func ExpandTemplate(template string, agentID string) string {
	normalizedID := routing.NormalizeAgentID(agentID)
	expanded := strings.ReplaceAll(template, "{agentId}", normalizedID)
	return config.ExpandUserPath(expanded)
}

// Bootstrap file templates
// These are minimal templates - can be enhanced with actual content
