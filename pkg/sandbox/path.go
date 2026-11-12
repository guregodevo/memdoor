package sandbox

import (
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// CanAccessPath checks if the agent can access the given path based on its scope.
//
// The path is cleaned first so that `..` traversal cannot smuggle an escape
// past the scope-prefix check. Without this, a virtual path like
// "/user/alice/../../etc/passwd" would satisfy strings.HasPrefix("/user/alice/")
// yet resolve (via filepath.Join in ResolvePath) to a location outside the
// sandbox. After Clean it becomes "/etc/passwd" and fails the prefix check.
//
// A virtual path is POSIX-shaped on every OS, so it is cleaned with path.Clean:
// filepath.Clean on Windows rewrites "/user/<id>/x" as "\user\<id>\x", and then
// every prefix below misses and the agent may reach nothing at all (#283).
func (ctx SandboxContext) CanAccessPath(p string) bool {
	p = path.Clean(p)
	switch ctx.AgentScope {
	case ScopeUser:
		return ctx.canAccessAsUser(p)
	case ScopeChannel:
		return ctx.canAccessAsChannel(p)
	case ScopeWorkspace:
		return ctx.canAccessAsWorkspace(p)
	default:
		return false
	}
}

// canAccessAsUser checks if a user-scoped agent can access the path
// User scope can ONLY access: /user/{initiating_user_id}/
func (ctx SandboxContext) canAccessAsUser(path string) bool {
	allowedPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
	return strings.HasPrefix(path, allowedPrefix)
}

// canAccessAsChannel checks if a channel-scoped agent can access the path
// Channel scope can access:
// - /channel/{channel_id}/ (shared channel data)
// - /user/{initiating_user_id}/ (user's own data)
func (ctx SandboxContext) canAccessAsChannel(path string) bool {
	channelPrefix := fmt.Sprintf("/channel/%s/", ctx.ChannelID)
	userPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
	return strings.HasPrefix(path, channelPrefix) ||
		strings.HasPrefix(path, userPrefix)
}

// canAccessAsWorkspace checks if a workspace-scoped agent can access the path
// Workspace scope can access:
// - /workspace/{workspace_id}/ (company-wide data)
// - /channel/{channel_id}/ (channel data)
// - /user/{initiating_user_id}/ (user's own data)
func (ctx SandboxContext) canAccessAsWorkspace(path string) bool {
	workspacePrefix := fmt.Sprintf("/workspace/%s/", ctx.WorkspaceID)
	channelPrefix := fmt.Sprintf("/channel/%s/", ctx.ChannelID)
	userPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
	return strings.HasPrefix(path, workspacePrefix) ||
		strings.HasPrefix(path, channelPrefix) ||
		strings.HasPrefix(path, userPrefix)
}

// ValidatePath returns an error if the path cannot be accessed
func (ctx SandboxContext) ValidatePath(path string) error {
	if !ctx.CanAccessPath(path) {
		return fmt.Errorf("%w: scope '%s' cannot access path '%s'",
			ErrAccessDenied, ctx.AgentScope, path)
	}
	return nil
}

// ResolvePath maps a virtual sandbox path to a real filesystem path
// Virtual paths like /user/{id}/file.txt become ~/.greg/sandbox/user/{id}/file.txt
func (ctx SandboxContext) ResolvePath(virtualPath string) (string, error) {
	// First validate the path is accessible
	if err := ctx.ValidatePath(virtualPath); err != nil {
		return "", err
	}

	// Get sandbox base directory
	sandboxBase := shared.MemdoorHome("sandbox")

	// Map virtual path to real path
	// Remove leading slash and replace with sandbox base
	relativePath := strings.TrimPrefix(virtualPath, "/")
	realPath := filepath.Join(sandboxBase, relativePath)

	// Defense in depth: ValidatePath already rejects traversal, but verify the
	// resolved real path still lives under the sandbox root so no future change
	// to the validation logic can let an agent write outside the sandbox.
	if realPath != sandboxBase && !strings.HasPrefix(realPath, sandboxBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: resolved path '%s' escapes sandbox root", ErrAccessDenied, realPath)
	}

	return realPath, nil
}

// NormalizePath converts a relative path to a virtual sandbox path
// If the path already starts with /, it's treated as a virtual path
// Otherwise it's converted to the appropriate scope path
func (ctx SandboxContext) NormalizePath(path string) string {
	// Already a virtual path (starts with /)
	if strings.HasPrefix(path, "/") {
		return path
	}

	// Convert relative path to appropriate virtual path based on agent scope
	switch ctx.AgentScope {
	case ScopeChannel:
		// Channel-scoped agents default to channel directory
		return fmt.Sprintf("/channel/%s/%s", ctx.ChannelID, path)
	case ScopeWorkspace:
		// Workspace-scoped agents default to workspace directory
		return fmt.Sprintf("/workspace/%s/%s", ctx.WorkspaceID, path)
	default:
		// User-scoped agents default to user directory
		return fmt.Sprintf("/user/%s/%s", ctx.InitiatingUserID, path)
	}
}
