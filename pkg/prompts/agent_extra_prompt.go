package prompts

// BuildAgentExtraPrompt assembles the "extra system prompt" that the
// message service prepends to the LLM's system message before every
// agent request. It is the single source of truth for that assembly —
// pkg/message/service.go calls it on the real request path, and the
// gateway calls it for a spawned agent's prompt, so both send the same
// bytes and a provider's prompt cache hits on either.
//
// Inputs:
//   - personality, systemPrompt come from the buddy row (empty string
//     when the column is NULL or blank — callers must dereference and
//     pass "").
//   - workspaceSlug, agentName drive the per-workspace override file
//     lookups (LoadWorkspacePromptOverride, LoadWorkspaceExamples).
//   - language is the workspace language enforcement string ("" to
//     skip the language section entirely).
//
// Output order — load-bearing, do not reorder:
//
//  1. "# Personality\n{personality}"     (skipped when personality == "")
//  2. "# Agent Instructions\n{prompt}"   uses workspace override if
//     present, else systemPrompt; skipped when both are empty.
//  3. workspace examples appended via LoadWorkspaceExamples (which
//     emits its own leading "\n\n## Workspace examples\n\n" — do NOT
//     add a separator before this call).
//  4. "# Language\nYou MUST respond in {language} language ONLY..."
//     (skipped when language == "").
func BuildAgentExtraPrompt(personality, systemPrompt, workspaceSlug, agentName, language string) string {
	var extraPrompt string
	if personality != "" {
		extraPrompt = "# Personality\n" + personality
	}
	if override := LoadWorkspacePromptOverride(workspaceSlug, agentName); override != "" {
		systemPrompt = override
	}
	if systemPrompt != "" {
		if extraPrompt != "" {
			extraPrompt += "\n\n"
		}
		extraPrompt += "# Agent Instructions\n" + systemPrompt
	}
	extraPrompt += LoadWorkspaceExamples(workspaceSlug, agentName)
	if language != "" {
		if extraPrompt != "" {
			extraPrompt += "\n\n"
		}
		extraPrompt += "# Language\nYou MUST respond in " + language + " language ONLY. Do NOT switch to any other language under any circumstances."
	}
	return extraPrompt
}
