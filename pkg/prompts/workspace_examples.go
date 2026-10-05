package prompts

import (
	"memdoor/pkg/shared"
	"os"
	"strings"
)

// LoadWorkspaceExamples reads a workspace-scoped supplement file and returns
// its contents formatted as an appendable prompt section, or "" if no file
// is found. The convention is:
//
//	~/.memdoor/workspaces/<workspace-slug>/agents/<agent-name>.examples.md
//
// This lets per-workspace deployments ship domain-specific examples (legal
// citations for `cyberlaw`, exercise references for `exercises`, etc.)
// without hardcoding any of them into the global seeder prompt.
//
// The returned string includes a "## Workspace examples" header and a blank
// line prefix so the caller can safely concatenate it onto an existing
// prompt without worrying about separator hygiene. Returns "" when:
//   - either argument is empty
//   - the home dir can't be resolved
//   - the file doesn't exist, is unreadable, or is empty after trimming
//
// Lives in pkg/prompts (not gateway/prompts) so both the HTTP mention path
// (gateway/agent_handlers.go) and the CLI / message-service path
// (pkg/message/service.go) can call it without pkg importing gateway.
func LoadWorkspaceExamples(workspaceSlug, agentName string) string {
	if workspaceSlug == "" || agentName == "" {
		return ""
	}
	path := shared.MemdoorHome("workspaces", workspaceSlug,
		"agents", agentName+".examples.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return ""
	}
	return "\n\n## Workspace examples\n\n" + body
}

// LoadWorkspacePromptOverride reads a full-prompt override for an agent in
// a given workspace. Unlike LoadWorkspaceExamples (which APPENDS supplemental
// domain hints), this function returns content intended to REPLACE the
// default prompt that was seeded from embedded assets.
//
// Convention:
//
//	~/.memdoor/workspaces/<workspace-slug>/agents/<agent-name>.prompt.md
//
// Rationale: the generic seeder ships a domain-neutral default. Deployments
// that need workspace-specific wording (stricter tone, additional rules,
// tailored examples) drop a file under the workspace's agents dir and the
// agent runtime uses that body verbatim as the system prompt — no generic-
// class edits, no rebuild.
//
// Returns "" when no override exists (the caller should fall back to the
// DB-seeded default).
func LoadWorkspacePromptOverride(workspaceSlug, agentName string) string {
	if workspaceSlug == "" || agentName == "" {
		return ""
	}
	path := shared.MemdoorHome("workspaces", workspaceSlug,
		"agents", agentName+".prompt.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
