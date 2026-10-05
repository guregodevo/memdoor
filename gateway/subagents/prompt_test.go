package subagents

import (
	"strings"
	"testing"
)

func TestBuildSubagentSystemPrompt(t *testing.T) {
	params := SubagentPromptParams{
		Task:         "Search documentation for API endpoints",
		AgentID:      "main",
		WorkspaceDir: "/home/user/.memdoor/workspace",
		Label:        "Doc Search",
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Check that prompt contains key elements
	expectedStrings := []string{
		"# Your task",
		"Search documentation for API endpoints", // The task
		"Doc Search",                             // The label
		"/home/user/.memdoor/workspace",          // Working directory
		"USING YOUR TOOLS",                       // action-first framing
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Prompt should contain %q, but doesn't.\nPrompt:\n%s", expected, prompt)
		}
	}
}

func TestBuildSubagentSystemPrompt_NoLabel(t *testing.T) {
	params := SubagentPromptParams{
		Task:         "Run unit tests",
		AgentID:      "main",
		WorkspaceDir: "/workspace",
		Label:        "", // No label
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Should contain task but not "Label:"
	if !strings.Contains(prompt, "Run unit tests") {
		t.Error("Prompt should contain the task")
	}

	if strings.Contains(prompt, "**Label**:") {
		t.Error("Prompt should not contain label section when label is empty")
	}
}

func TestBuildSubagentSystemPrompt_NoWorkspace(t *testing.T) {
	params := SubagentPromptParams{
		Task:         "Analyze code complexity",
		AgentID:      "main",
		WorkspaceDir: "", // No workspace
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Should not contain working directory section
	if strings.Contains(prompt, "## Working Directory") {
		t.Error("Prompt should not contain working directory section when workspace is empty")
	}

	// Should still contain task
	if !strings.Contains(prompt, "Analyze code complexity") {
		t.Error("Prompt should contain the task")
	}
}

func TestBuildSubagentSystemPrompt_MinimalParams(t *testing.T) {
	params := SubagentPromptParams{
		Task:    "Fetch latest news",
		AgentID: "main",
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Should contain essential elements
	essentialStrings := []string{
		"# Your task",
		"Fetch latest news",
		"USING YOUR TOOLS",
	}

	for _, expected := range essentialStrings {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Prompt should contain %q", expected)
		}
	}
}

func TestPromptStructure(t *testing.T) {
	params := SubagentPromptParams{
		Task:         "Test task",
		AgentID:      "main",
		WorkspaceDir: "/workspace",
		Label:        "Test Label",
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Check section ordering (approximate)
	sections := []string{
		"# Your task",
		"USING YOUR TOOLS",
		"Working directory for file operations",
	}

	lastIndex := -1
	for i, section := range sections {
		index := strings.Index(prompt, section)
		if index == -1 {
			t.Errorf("Section %d (%q) not found in prompt", i, section)
			continue
		}

		if index <= lastIndex {
			t.Errorf("Section %d (%q) should appear after previous section", i, section)
		}

		lastIndex = index
	}
}

func TestPromptNoHTMLOrScriptTags(t *testing.T) {
	params := SubagentPromptParams{
		Task:         "Task with <script>alert('xss')</script>",
		AgentID:      "main",
		WorkspaceDir: "/path/to/<script>",
		Label:        "Label with <html>",
	}

	prompt := BuildSubagentSystemPrompt(params)

	// Prompt should preserve the raw text (not sanitize),
	// since it's meant for LLM consumption, not HTML rendering
	if !strings.Contains(prompt, "<script>") {
		t.Error("Prompt should preserve raw task text including angle brackets")
	}
}
