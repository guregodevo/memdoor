package subagents

import (
	"fmt"
	"strings"
)

// SubagentPromptParams contains parameters for building a subagent system prompt
// Pattern: OpenClaw's buildSubagentSystemPrompt()
type SubagentPromptParams struct {
	Task         string
	AgentID      string
	WorkspaceDir string
	Label        string
}

// BuildSubagentSystemPrompt generates a focused system prompt for subagents
// Pattern: OpenClaw src/agents/tools/sessions-spawn-tool.ts
//
// The prompt emphasizes:
// - Task focus (do your assigned task, nothing else)
// - Ephemeral nature (you may be terminated after completion)
// - No proactive actions (no heartbeats, no side quests)
// - Clear output expectations
func BuildSubagentSystemPrompt(params SubagentPromptParams) string {
	var sb strings.Builder

	// Action-first framing. This is prepended to the agent's OWN system prompt
	// (its role/palette), so it must NOT turn a doer into a reporter: an earlier
	// version led with "report your findings / output format = what you
	// accomplished", which biased a coder to write PROSE instead of calling its
	// tools. Keep it minimal — the task + workdir + a one-line "report when done"
	// — and let the agent's own prompt drive HOW it works.
	sb.WriteString("# Your task\n\n")
	if params.Label != "" {
		sb.WriteString(fmt.Sprintf("**Label**: %s\n\n", params.Label))
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", params.Task))
	sb.WriteString("DO this task now by USING YOUR TOOLS — take the actual actions it needs " +
		"(write files, run commands). Do not describe what you would do; do it. Stay focused on " +
		"this one task; no side quests.\n\n")

	if params.WorkspaceDir != "" {
		sb.WriteString(fmt.Sprintf("Working directory for file operations: `%s`\n\n", params.WorkspaceDir))
	}

	sb.WriteString("When the task is actually done, reply with one or two sentences on what you " +
		"DID and the real result (the file you wrote, the command's real output). That reply is " +
		"reported back to the agent that spawned you.\n")

	return sb.String()
}
