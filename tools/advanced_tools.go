package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"memdoor/gateway/context"
	"memdoor/pkg/shared"
	"memdoor/skills"

	"github.com/google/uuid"
)

// ===============================================
// TOOL 1: task
// ===============================================

// TaskInput represents the input for task tool
type TaskInput struct {
	Description  string `json:"description" jsonschema_description:"Short description of task (3-5 words)"`
	Prompt       string `json:"prompt" jsonschema_description:"Detailed task for agent to perform autonomously"`
	SubagentType string `json:"subagent_type,omitempty" jsonschema_description:"Agent type: 'general-purpose', 'explore', or 'plan' (default: general-purpose)"`
	Model        string `json:"model,omitempty" jsonschema_description:"Model to use: 'sonnet', 'opus', or 'haiku' (default: sonnet)"`
}

var TaskInputSchema = GenerateSchema[TaskInput]()

// TaskDefinition defines the task tool
// Pattern: OpenClaw's task-tool.ts - launch specialized sub-agents for complex multi-step tasks
var TaskDefinition = ToolDefinition{
	Name: "task",
	Description: `Launch a specialized sub-agent to handle complex, multi-step tasks autonomously.

Use this tool when you need to:
- Break down complex tasks into independent sub-tasks
- Execute multiple operations in parallel
- Delegate specialized work to focused agents
- Perform research or exploration requiring multiple rounds

Agent Types:
- 'general-purpose': Full capabilities for complex multi-step tasks
- 'explore': Specialized for codebase exploration, file searching, and code discovery
- 'plan': Optimized for planning and analysis tasks

The sub-agent runs autonomously and returns results when complete.

Pattern: OpenClaw's Task tool - autonomous sub-agent execution for complex workflows`,
	InputSchema: TaskInputSchema,
	Function:    Task,
}

// Task launches a sub-agent to handle a complex task autonomously
// Pattern: Higher-level wrapper around sessions_spawn with task-specific metadata
func Task(input json.RawMessage) (string, error) {
	// Parse input
	var params TaskInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.Description == "" {
		return "", fmt.Errorf("description is required (short 3-5 word summary)")
	}
	if params.Prompt == "" {
		return "", fmt.Errorf("prompt is required (detailed task instructions)")
	}

	// Set defaults
	subagentType := params.SubagentType
	if subagentType == "" {
		subagentType = "general-purpose"
	}

	// Validate subagent type
	validTypes := map[string]bool{
		"general-purpose": true,
		"explore":         true,
		"plan":            true,
	}
	if !validTypes[subagentType] {
		return "", fmt.Errorf("invalid subagent_type: %s (must be general-purpose, explore, or plan)", subagentType)
	}

	model := params.Model
	if model == "" {
		model = "sonnet"
	}

	// Validate model
	validModels := map[string]bool{
		"sonnet": true,
		"opus":   true,
		"haiku":  true,
	}
	if !validModels[model] {
		return "", fmt.Errorf("invalid model: %s (must be sonnet, opus, or haiku)", model)
	}

	// Generate unique session key for task agent
	// Pattern: OpenClaw uses descriptive keys with agent types
	taskKey := fmt.Sprintf("task-%s", uuid.New().String()[:8])

	// Build task prompt with agent type context
	var enhancedPrompt string
	switch subagentType {
	case "explore":
		enhancedPrompt = fmt.Sprintf("[EXPLORATION TASK]\n%s\n\nFocus on: Code discovery, file searching, pattern finding, codebase understanding.", params.Prompt)
	case "plan":
		enhancedPrompt = fmt.Sprintf("[PLANNING TASK]\n%s\n\nFocus on: Analysis, design, planning, architecture, breaking down complex requirements.", params.Prompt)
	default: // general-purpose
		enhancedPrompt = params.Prompt
	}

	// Prepare request to gateway using /api/agent endpoint
	// Pattern: Similar to sessions_spawn but with task-specific metadata
	requestBody := map[string]interface{}{
		"message":     enhancedPrompt,
		"session_key": taskKey,
		"lane":        "task", // Task execution lane
		"metadata": map[string]interface{}{
			"task_description": params.Description,
			"subagent_type":    subagentType,
			"model":            model,
		},
	}

	// Call gateway
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	gatewayURL := apiURL("/api/agent")
	resp, err := http.Post(gatewayURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gateway returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Format result
	output := map[string]interface{}{
		"status":         result["status"],
		"task_key":       taskKey,
		"description":    params.Description,
		"subagent_type":  subagentType,
		"model":          model,
		"note":           "Task agent launched autonomously. Check session for results.",
		"session_access": fmt.Sprintf("Use session_status or sessions_history with key '%s' to see progress", taskKey),
	}

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}

// ===============================================
// TOOL 2: exit_plan_mode
// ===============================================

// ExitPlanModeInput represents the input for exit_plan_mode
type ExitPlanModeInput struct {
	Plan string `json:"plan" jsonschema_description:"The detailed plan to present to user for approval"`
}

var ExitPlanModeInputSchema = GenerateSchema[ExitPlanModeInput]()

// ExitPlanModeDefinition defines the exit_plan_mode tool
// Pattern: OpenClaw's plan mode control - separate planning from execution
var ExitPlanModeDefinition = ToolDefinition{
	Name: "exit_plan_mode",
	Description: `Leave plan mode: present a short plan and hand control back to the user for approval.

In plan mode you are READ-ONLY — bash, apply_patch, and file edits are blocked. This
is the ONLY way to move from planning to doing. Call it as soon as you know what you
will do (you do not need to research further); after the user approves, the same tools
become available and you implement the plan.

Call it when:
- A mutating tool (bash / apply_patch / edit) was just blocked because you are in plan mode.
- You know which file(s) you'll create or edit and how you'll verify (the build/run/test command).

Keep the plan SHORT and concrete:
- The file(s) you will create or edit.
- The change in one or two lines.
- How you will verify it works — the build/run/test command for THIS project's
  language and toolchain (whatever that is; there is no fixed command).

After you call it, STOP and wait — do not keep reading files or narrating.

Pattern: OpenClaw's exit_plan_mode - separates planning from execution for complex tasks`,
	InputSchema: ExitPlanModeInputSchema,
	Function:    ExitPlanMode,
}

// ExitPlanMode presents a plan to the user and exits plan mode
// Pattern: OpenClaw's plan mode control system
func ExitPlanMode(input json.RawMessage) (string, error) {
	// Parse input
	var params ExitPlanModeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.Plan == "" {
		return "", fmt.Errorf("plan is required (detailed plan description)")
	}

	// Format plan for presentation
	result := map[string]interface{}{
		"status": "plan_ready",
		"plan":   params.Plan,
		"note":   "Plan presented to user. Waiting for approval to proceed with implementation.",
		"next":   "User should review and approve/modify the plan before execution begins.",
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	// Return formatted plan
	// The actual plan content is already in params.Plan and will be visible to user
	// This tool serves as a signal that planning is complete
	return string(outputJSON), nil
}

// ===============================================
// TOOL 3: notebook_edit
// ===============================================

// NotebookEditInput represents the input for notebook_edit
type NotebookEditInput struct {
	NotebookPath string `json:"notebook_path" jsonschema_description:"Path to .ipynb file"`
	CellIndex    int    `json:"cell_index,omitempty" jsonschema_description:"Cell index to edit (0-based). Required for replace/delete modes."`
	CellType     string `json:"cell_type,omitempty" jsonschema_description:"Cell type: 'code' or 'markdown' (required for insert mode)"`
	NewSource    string `json:"new_source" jsonschema_description:"New cell content/source code"`
	EditMode     string `json:"edit_mode,omitempty" jsonschema_description:"Edit mode: 'replace', 'insert', or 'delete' (default: replace)"`
}

var NotebookEditInputSchema = GenerateSchema[NotebookEditInput]()

// NotebookEditDefinition defines the notebook_edit tool
// Pattern: OpenClaw's notebook-edit-tool.ts
var NotebookEditDefinition = ToolDefinition{
	Name: "notebook_edit",
	Description: `Edit Jupyter notebooks (.ipynb files) programmatically.

Use this to:
- Add new cells to notebooks
- Replace existing cell content
- Delete cells
- Modify code or markdown cells

Edit Modes:
- 'replace': Replace cell at cell_index with new_source
- 'insert': Insert new cell at cell_index (requires cell_type)
- 'delete': Delete cell at cell_index

Cell Types:
- 'code': Python/code cell
- 'markdown': Markdown documentation cell

Pattern: OpenClaw's notebook_edit - programmatic notebook manipulation`,
	InputSchema: NotebookEditInputSchema,
	Function:    NotebookEdit,
}

// NotebookCell represents a Jupyter notebook cell
type NotebookCell struct {
	CellType       string                 `json:"cell_type"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Source         interface{}            `json:"source"` // Can be string or []string
	ExecutionCount interface{}            `json:"execution_count,omitempty"`
	Outputs        []interface{}          `json:"outputs,omitempty"`
}

// NotebookStructure represents a Jupyter notebook structure
type NotebookStructure struct {
	Cells       []NotebookCell         `json:"cells"`
	Metadata    map[string]interface{} `json:"metadata"`
	NBFormat    int                    `json:"nbformat"`
	NBFormatMin int                    `json:"nbformat_minor"`
}

// NotebookEdit edits a Jupyter notebook file
// Pattern: OpenClaw's notebook editing - JSON manipulation with cell operations
func NotebookEdit(input json.RawMessage) (string, error) {
	// Parse input
	var params NotebookEditInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.NotebookPath == "" {
		return "", fmt.Errorf("notebook_path is required")
	}

	// Set default edit mode
	editMode := params.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	// Validate edit mode
	validModes := map[string]bool{
		"replace": true,
		"insert":  true,
		"delete":  true,
	}
	if !validModes[editMode] {
		return "", fmt.Errorf("invalid edit_mode: %s (must be replace, insert, or delete)", editMode)
	}

	// Read existing notebook
	notebookJSON, err := ReadFile(json.RawMessage(fmt.Sprintf(`{"path":"%s"}`, params.NotebookPath)))
	if err != nil {
		return "", fmt.Errorf("failed to read notebook: %w", err)
	}

	// Parse notebook structure
	var notebook NotebookStructure
	if err := json.Unmarshal([]byte(notebookJSON), &notebook); err != nil {
		return "", fmt.Errorf("failed to parse notebook JSON: %w", err)
	}

	// Perform edit based on mode
	var operationDesc string
	switch editMode {
	case "replace":
		if params.CellIndex < 0 || params.CellIndex >= len(notebook.Cells) {
			return "", fmt.Errorf("invalid cell_index: %d (notebook has %d cells)", params.CellIndex, len(notebook.Cells))
		}
		notebook.Cells[params.CellIndex].Source = params.NewSource
		operationDesc = fmt.Sprintf("Replaced cell %d", params.CellIndex)

	case "insert":
		if params.CellType == "" {
			return "", fmt.Errorf("cell_type is required for insert mode")
		}
		if params.CellType != "code" && params.CellType != "markdown" {
			return "", fmt.Errorf("invalid cell_type: %s (must be code or markdown)", params.CellType)
		}

		newCell := NotebookCell{
			CellType: params.CellType,
			Metadata: make(map[string]interface{}),
			Source:   params.NewSource,
		}
		if params.CellType == "code" {
			newCell.ExecutionCount = nil
			newCell.Outputs = make([]interface{}, 0)
		}

		// Insert at index (or append if index >= len)
		if params.CellIndex >= len(notebook.Cells) {
			notebook.Cells = append(notebook.Cells, newCell)
			operationDesc = fmt.Sprintf("Appended new %s cell", params.CellType)
		} else {
			// Insert at position
			notebook.Cells = append(notebook.Cells[:params.CellIndex], append([]NotebookCell{newCell}, notebook.Cells[params.CellIndex:]...)...)
			operationDesc = fmt.Sprintf("Inserted new %s cell at index %d", params.CellType, params.CellIndex)
		}

	case "delete":
		if params.CellIndex < 0 || params.CellIndex >= len(notebook.Cells) {
			return "", fmt.Errorf("invalid cell_index: %d (notebook has %d cells)", params.CellIndex, len(notebook.Cells))
		}
		notebook.Cells = append(notebook.Cells[:params.CellIndex], notebook.Cells[params.CellIndex+1:]...)
		operationDesc = fmt.Sprintf("Deleted cell %d", params.CellIndex)
	}

	// Write modified notebook back
	notebookBytes, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal notebook: %w", err)
	}

	writeInput := WriteFileInput{
		Path:    params.NotebookPath,
		Content: string(notebookBytes),
	}
	writeJSON, _ := json.Marshal(writeInput)
	_, err = WriteFile(writeJSON)
	if err != nil {
		return "", fmt.Errorf("failed to write notebook: %w", err)
	}

	// Format result
	result := map[string]interface{}{
		"status":      "success",
		"notebook":    params.NotebookPath,
		"operation":   operationDesc,
		"total_cells": len(notebook.Cells),
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}

// ===============================================
// TOOL 4: skill
// ===============================================

// SkillInput represents the input for skill tool
type SkillInput struct {
	Command string `json:"command" jsonschema_description:"The skill command to execute (e.g., 'pdf', 'xlsx')"`
}

var SkillInputSchema = GenerateSchema[SkillInput]()

// SkillDefinition defines the skill tool
// Pattern: OpenClaw's skill execution system
var SkillDefinition = ToolDefinition{
	Name:        "skill",
	Description: `Load a named skill — a step-by-step procedure — and follow it. Built in: review (read and assess existing code), chrome (search or read the web with the memdoor chrome CLI). An unknown name lists what is available. To save a procedure you worked out, write it to .agents/skills/<name>.md in the project.`,
	InputSchema: SkillInputSchema,
	Function:    Skill,
}

// Skill loads a named skill — a reusable workflow written as markdown — and
// returns its content for the agent to follow. Skills are resolved by name
// (`<name>.md`) from, in order: <project>/.agents/skills,
// ./skills, and ~/.memdoor/workspace/skills. Not found → an error listing
// what's available.
func Skill(input json.RawMessage) (string, error) {
	var params SkillInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("command is required (skill name to load)")
	}
	// Cwd is harness-injected (coder-workdir confinement), not model-facing —
	// it adds the project-local .agents/skills tier to the resolution chain.
	var conf struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(input, &conf)
	// Sanitize: name only, no path traversal, tolerate a trailing .md.
	name := strings.TrimSuffix(filepath.Base(strings.TrimSpace(params.Command)), ".md")

	if content, ok := LookupSkill(name, conf.Cwd); ok {
		return fmt.Sprintf("# Skill: %s\n\n%s\n\n(Follow this workflow now.)", name, content), nil
	}
	// The not-found error TEACHES the creation move: a model told to "save a
	// skill then run it" skips the save and loops on this error (live: three
	// identical failures) unless the error itself names the missing step.
	return "", fmt.Errorf("skill %q not found — it does not exist yet. To CREATE it, apply_patch a new file .agents/skills/%s.md containing the workflow steps, THEN call skill %q again. Available skills: %s",
		name, name, name, strings.Join(availableSkills(skillDirs(conf.Cwd)), ", "))
}

// LookupSkill resolves a skill by name through the standard chain (see
// skillDirs) with the embedded library as the final fallback, returning its
// trimmed content. Shared by the skill tool and the gateway's slash-command
// expansion so both surfaces resolve identically.
func LookupSkill(name, projectDir string) (string, bool) {
	name = strings.TrimSuffix(filepath.Base(strings.TrimSpace(name)), ".md")
	if name == "" {
		return "", false
	}
	for _, dir := range skillDirs(projectDir) {
		if content, err := os.ReadFile(filepath.Join(dir, name+".md")); err == nil {
			return strings.TrimSpace(string(content)), true
		}
		// agentskills.io folder layout: <dir>/<name>/SKILL.md (e.g. skills
		// copied from oh-my-pi). Resolution is keyed on <name>, so a folder
		// skill and a flat skill with the same name resolve identically.
		if content, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md")); err == nil {
			return strings.TrimSpace(string(content)), true
		}
	}
	if content, err := skills.FS.ReadFile(name + ".md"); err == nil {
		return strings.TrimSpace(string(content)), true
	}
	return "", false
}

// skillDirs is the ordered list of directories skills are resolved from —
// first hit wins. The chain is cwd-free except for the explicit dev tier (the
// gateway's own cwd), so a DEPLOYED gateway resolves the same skills no matter
// where it was started from:
//  1. <project>/.agents/skills — project-local (agentskills.io convention);
//     inside the coder's confinement, so the coder can WRITE skills here
//  2. gateway-cwd ./skills — dev checkout override (beats the managed seed,
//     so editing repo skills works during development)
//  3. ~/.memdoor/workspace/skills — workspace bootstrap dir
//  4. ~/.memdoor/skills — managed tier, seeded from the embedded library at
//     boot (see gateway.SeedEmbeddedSkills); user edits here are live
//     immediately and never overwritten
func skillDirs(projectDir string) []string {
	var dirs []string
	if projectDir != "" {
		dirs = append(dirs, filepath.Join(projectDir, ".agents", "skills"))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "skills"))
	}
	dirs = append(dirs, shared.MemdoorHome("workspace", "skills"), shared.MemdoorHome("skills"))
	return dirs
}

// availableSkills lists the distinct skill names found across dirs, sorted.
func availableSkills(dirs []string) []string {
	seen := map[string]bool{}
	var names []string
	// The embedded library is always available, regardless of cwd.
	if entries, err := skills.FS.ReadDir("."); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				n := strings.TrimSuffix(e.Name(), ".md")
				if !seen[n] {
					seen[n] = true
					names = append(names, n)
				}
			}
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				n := strings.TrimSuffix(e.Name(), ".md")
				if !seen[n] {
					seen[n] = true
					names = append(names, n)
				}
			}
		}
		// Folder-layout skills: <name>/SKILL.md counts as skill <name>.
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil && !seen[e.Name()] {
					seen[e.Name()] = true
					names = append(names, e.Name())
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

// ===============================================
// TOOL 5: slash_command
// ===============================================

// SlashCommandInput represents the input for slash_command tool
type SlashCommandInput struct {
	Command string `json:"command" jsonschema_description:"The slash command to execute (including arguments, e.g., '/help', '/review-pr 123')"`
}

var SlashCommandInputSchema = GenerateSchema[SlashCommandInput]()

// SlashCommandDefinition defines the slash_command tool
// Pattern: OpenClaw's slash command execution system
var SlashCommandDefinition = ToolDefinition{
	Name: "slash_command",
	Description: `Execute a custom slash command.

Slash commands are user-defined shortcuts for common operations. They are defined
in .agents/commands/ directory (.claude/commands/ also works).

Examples:
- /help - Show available commands
- /review-pr 123 - Review pull request #123
- /deploy staging - Deploy to staging environment

Usage: Provide the full slash command including arguments.

Pattern: OpenClaw's SlashCommand tool - execute custom command shortcuts`,
	InputSchema: SlashCommandInputSchema,
	Function:    SlashCommand,
}

// SlashCommand executes a custom slash command
// Pattern: Simple command execution - can be extended with command registry
func SlashCommand(input json.RawMessage) (string, error) {
	// Parse input
	var params SlashCommandInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.Command == "" {
		return "", fmt.Errorf("command is required (slash command to execute)")
	}

	// Validate command starts with /
	if params.Command[0] != '/' {
		return "", fmt.Errorf("command must start with / (e.g., '/help', '/compact')")
	}

	// Parse command and arguments
	parts := strings.Fields(params.Command)
	cmdName := parts[0]

	// Handle built-in commands
	switch cmdName {
	case "/compact":
		return handleCompactCommand(parts[1:])

	case "/new":
		return `{"status": "info", "message": "To start a new session, please exit and restart the TUI, or use the gateway API to create a new session."}`, nil

	case "/help":
		return `Available slash commands:
- /compact - Check context usage and compaction status
- /new - Start a new session (requires restart)
- /help - Show this help message

Pattern: OpenClaw slash command system`, nil

	default:
		// For unknown commands, return a helpful message
		result := map[string]interface{}{
			"command": params.Command,
			"status":  "unknown_command",
			"message": fmt.Sprintf("Unknown slash command: '%s'. Use /help to see available commands.", cmdName),
		}

		outputJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format output: %w", err)
		}

		return string(outputJSON), nil
	}
}

// handleCompactCommand implements the /compact slash command
// Pattern: Manual compaction trigger with context awareness
func handleCompactCommand(args []string) (string, error) {
	// Check if context state is available
	if ContextToolState.Model == "" {
		return "", fmt.Errorf("context information not available")
	}

	// Create context inspector
	inspector, err := context.NewContextInspector(
		ContextToolState.Model,
		ContextToolState.SystemPrompt,
		ContextToolState.ToolSchemas,
		ContextToolState.Messages,
		false,
	)
	if err != nil {
		return "", fmt.Errorf("failed to inspect context: %w", err)
	}

	summary := inspector.GetSummary()

	// Build response
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Context Status: %.1f%% (%d / %d tokens)\n",
		summary.Utilization,
		summary.TotalTokens,
		summary.EffectiveLimit))
	response.WriteString(fmt.Sprintf("Messages in session: %d\n\n", summary.MessageCount))

	// Determine recommendation
	if summary.Utilization < 60.0 {
		response.WriteString("✅ No compaction needed yet (context below 60%)\n")
		response.WriteString("Auto-compaction will trigger when context reaches:\n")
		response.WriteString("- 60%: Local compaction (fast, drops old messages)\n")
		response.WriteString("- 90%: AI compaction (smart summarization)\n")
	} else if summary.Utilization < 90.0 {
		response.WriteString("⚡ Local compaction will trigger on your next message\n")
		response.WriteString("- Type: Fast local pruning (no AI call)\n")
		response.WriteString("- Keeps: ~20 recent messages\n")
		response.WriteString("- Speed: Instant (no API latency)\n")
	} else if summary.MessageCount > 500 {
		response.WriteString("⚠️  Session heavily bloated with 500+ messages\n\n")
		response.WriteString("Recommendations:\n")
		response.WriteString("1. Type /new to start a fresh session (recommended)\n")
		response.WriteString("2. Continue - AI compaction will trigger on next message\n")
		response.WriteString("   (slower, creates intelligent summary)\n\n")
		response.WriteString("Note: Old session history is preserved on disk\n")
	} else {
		response.WriteString("🔄 AI compaction will trigger on your next message\n")
		response.WriteString("- Type: AI-powered summarization\n")
		response.WriteString("- Preserves important context\n")
		response.WriteString("- Speed: Slower (API call required)\n")
	}

	return response.String(), nil
}

// AvailableSkillNames lists every skill name resolvable from projectDir's
// perspective (disk chain + embedded), sorted — the TUI's autocomplete source.
func AvailableSkillNames(projectDir string) []string {
	return availableSkills(skillDirs(projectDir))
}
