package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"memdoor/gateway/config"
	"memdoor/gateway/logs"
	"memdoor/pkg/platform"
	"memdoor/pkg/sandbox"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// remoteToolAdapter implements platform.RemoteToolExecutor
// It dispatches tool calls from remote agents to local tool functions
type remoteToolAdapter struct {
	tools []tools.ToolDefinition
	log   *logs.EventLogger
}

// NewRemoteToolAdapter creates a tool executor for remote agents
func NewRemoteToolAdapter(allTools []tools.ToolDefinition) *remoteToolAdapter {
	return &remoteToolAdapter{
		tools: allTools,
		log:   logs.New("RemoteTool"),
	}
}

// GetToolDefinitions returns platform-format tool definitions filtered by agent's allowed tools
func (a *remoteToolAdapter) GetToolDefinitions(ctx context.Context, agentTools []string) []platform.ToolDefinition {
	var filtered []tools.ToolDefinition

	if len(agentTools) > 0 {
		// Filter to agent's allowed tools
		allowed := make(map[string]bool, len(agentTools))
		for _, name := range agentTools {
			allowed[name] = true
		}
		for _, tool := range a.tools {
			if allowed[tool.Name] {
				filtered = append(filtered, tool)
			}
		}
	} else {
		// No specific tools — use default profile
		filtered = filterToolsByDefaultProfile(a.tools)
	}

	// Convert to platform format
	defs := make([]platform.ToolDefinition, 0, len(filtered))
	for _, tool := range filtered {
		defs = append(defs, platform.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return defs
}

// ExecuteToolCall executes a single tool call and returns the result
func (a *remoteToolAdapter) ExecuteToolCall(ctx context.Context, toolName string, arguments string) (string, error) {
	// Extract sandbox context from Go context
	var sandboxCtx sandbox.SandboxContext
	if v := ctx.Value(sharedctx.SandboxContextKey); v != nil {
		sandboxCtx = v.(sandbox.SandboxContext)
	}

	// Find and execute the tool
	for _, tool := range a.tools {
		if tool.Name == toolName {
			input := json.RawMessage(arguments)

			if tool.FunctionWithContext != nil {
				result, err := tool.FunctionWithContext(input, sandboxCtx)
				if err != nil {
					a.log.Error("Remote tool execution failed",
						slog.String("tool", toolName),
						slog.Any("error", err))
					return "", err
				}
				a.log.Info("Remote tool executed",
					slog.String("tool", toolName),
					slog.Int("result_length", len(result)))
				return result, nil
			}

			if tool.Function != nil {
				result, err := tool.Function(input)
				if err != nil {
					return "", err
				}
				return result, nil
			}

			return "", fmt.Errorf("tool '%s' has no execution function", toolName)
		}
	}

	return "", fmt.Errorf("tool '%s' not found", toolName)
}

// filterToolsByDefaultProfile returns tools matching the default profile
func filterToolsByDefaultProfile(allTools []tools.ToolDefinition) []tools.ToolDefinition {
	profile, ok := config.TOOL_PROFILES[config.DefaultToolProfile]
	if !ok {
		return allTools
	}

	// Expand profile's allow list (resolves groups like "group:fs_read")
	expandedNames := config.ExpandToolGroups(profile.Allow)
	allowed := make(map[string]bool, len(expandedNames))
	for _, name := range expandedNames {
		allowed[name] = true
	}

	var filtered []tools.ToolDefinition
	for _, tool := range allTools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}
