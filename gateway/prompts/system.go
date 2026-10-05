package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"memdoor/gateway/config"
	"memdoor/gateway/skills"
	"memdoor/tools"
)

// PromptMode determines which sections to include in system prompt
type PromptMode string

const (
	PromptModeFull    PromptMode = "full"    // All sections (default for main agent)
	PromptModeChat    PromptMode = "chat"    // Lean prompt for fan-facing chat agents
	PromptModeMinimal PromptMode = "minimal" // Minimal sections (for sub-agents)
	PromptModeNone    PromptMode = "none"    // Identity only (tools + doer buddies whose seed is self-contained)
)

// SystemPromptBuilder builds the system prompt for agent inference
type SystemPromptBuilder struct {
	projectDir     string // the turn's working directory (SetProjectDir); "" = none
	projectSection string // a judged selection of the project's instructions
	agentConfig    *config.AgentConfig
	tools          []tools.ToolDefinition
	workspace      string
	promptMode     PromptMode
	model          string
	skills         []skills.Skill
	secretNames    []string
	agentName      string // the actual agent being run (e.g. "coder"); overrides the static agentConfig.ID in the identity line
}

// SetAgentName sets the real identity of the agent being built, so the prompt says
// "You are agent 'coder'" and not the shared runtime's static config id ("main") —
// which would contradict the buddy's own prompt.
func (spb *SystemPromptBuilder) SetAgentName(name string) {
	spb.agentName = name
}

// NewSystemPromptBuilder creates a new system prompt builder
func NewSystemPromptBuilder(
	agentConfig *config.AgentConfig,
	tools []tools.ToolDefinition,
	workspace string,
	model string,
) *SystemPromptBuilder {
	return &SystemPromptBuilder{
		agentConfig: agentConfig,
		tools:       tools,
		workspace:   workspace,
		promptMode:  PromptModeFull, // Default to full
		model:       model,
	}
}

// SetPromptMode sets the prompt mode
func (spb *SystemPromptBuilder) SetPromptMode(mode PromptMode) {
	spb.promptMode = mode
}

// SetSkills sets the skills for the prompt
func (spb *SystemPromptBuilder) SetSkills(sk []skills.Skill) {
	spb.skills = sk
}

// SetSecretNames sets available secret names for the agent
func (spb *SystemPromptBuilder) SetSecretNames(names []string) {
	spb.secretNames = names
}

// Build assembles the complete system prompt
func (spb *SystemPromptBuilder) Build() (string, error) {
	if spb.promptMode == PromptModeNone {
		// A doer buddy runs on its own seeded prompt, not this builder's
		// sections — but the project's instructions are the project's, and
		// every coder turn carries them (AGENTS.md at the workdir's root).
		if project := spb.buildProjectInstructionsSection(); project != "" {
			return spb.buildIdentityOnly() + "\n\n" + project, nil
		}
		return spb.buildIdentityOnly(), nil
	}

	// Chat mode: lean prompt for fan-facing agents (minimal tokens)
	if spb.promptMode == PromptModeChat {
		return spb.buildIdentityOnly() + "\n\n" + spb.buildCollaborationSection(), nil
	}

	sections := []string{}

	// 1. Tooling Section
	sections = append(sections, spb.buildToolingSection())

	// 2. Safety Section
	sections = append(sections, spb.buildSafetySection())

	// 2b. Anomaly Reporting Section — tells the model WHEN to call
	// the report_bug tool. Without this nudge the tool sits in the
	// palette but the model only invokes it on explicit user request.
	// docs/internal/TELEMETRY.md captures the design rationale.
	sections = append(sections, spb.buildAnomalyReportingSection())

	// 3. Agent Collaboration Section (for channels)
	sections = append(sections, spb.buildCollaborationSection())

	// 4. Workspace Section
	sections = append(sections, spb.buildWorkspaceSection())

	// 4b. The project's own instructions: AGENTS.md at the
	// root of the directory the turn works in. The convention every coding
	// agent reads (Codex, OpenCode, Claude Code since 2.1.277); without it
	// the coder ignores the build, test and style rules a repo wrote down
	// for exactly this reader (docs/roadmap/MUST.md, TUI table stakes).
	if section := spb.buildProjectInstructionsSection(); section != "" {
		sections = append(sections, section)
	}

	// 5. Bootstrap Files Section (full mode only)
	if spb.promptMode == PromptModeFull {
		bootstrapSection, err := spb.buildBootstrapSection()
		if err != nil {
			// Log error but continue (bootstrap files are optional)
			fmt.Fprintf(os.Stderr, "Warning: Bootstrap section build failed: %v\n", err)
		} else if bootstrapSection != "" {
			sections = append(sections, bootstrapSection)
		}
	}

	// 6. Skills Section (full mode only, if skills are loaded)
	if spb.promptMode == PromptModeFull && len(spb.skills) > 0 {
		skillsSection := spb.buildSkillsSection()
		if skillsSection != "" {
			sections = append(sections, skillsSection)
		}
	}

	// 7. Secrets Section (if agent has secrets)
	if len(spb.secretNames) > 0 {
		sections = append(sections, spb.buildSecretsSection())
	}

	// 8. Date/Time Section
	sections = append(sections, spb.buildDateTimeSection())

	// 9. Runtime Section
	sections = append(sections, spb.buildRuntimeSection())

	// Join all sections with double newline
	return strings.Join(sections, "\n\n"), nil
}

// buildToolingSection creates the tooling section
func (spb *SystemPromptBuilder) buildToolingSection() string {
	var sb strings.Builder

	sb.WriteString("# Available Tools\n\n")
	sb.WriteString("You have access to the following tools:\n\n")

	for _, tool := range spb.tools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description))
	}

	sb.WriteString("\nUse these tools to accomplish tasks. Always prefer using specialized tools over bash commands when possible.")

	return sb.String()
}

// buildSafetySection creates the safety section
func (spb *SystemPromptBuilder) buildSafetySection() string {
	return `# Safety Guidelines

IMPORTANT Security Reminders:
- Assist with authorized security testing, defensive security, CTF challenges, and educational contexts
- Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes
- Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context: pentesting engagements, CTF competitions, security research, or defensive use cases
- Never commit secrets, API keys, or credentials to version control
- Validate file paths before operations to prevent directory traversal
- Be cautious with destructive operations (rm, delete, DROP TABLE, etc.)`
}

// buildAnomalyReportingSection nudges the model to use the
// report_bug tool proactively. Pairs with tools/report_bug.go, which
// is already in every agent's palette — without this prompt section
// the tool description is read but rarely acted on. The wording is
// terse on purpose: the tool's own Description carries the full
// schema guidance; the section here just sets the "when."
//
// Lives in full-mode prompts only. Chat-mode (fan-facing) agents skip
// it to keep token usage minimal — and they typically don't have the
// observability tools (logs_query etc.) needed to articulate a
// useful report anyway.
func (spb *SystemPromptBuilder) buildAnomalyReportingSection() string {
	return `# Reporting Anomalies

You have a ` + "`report_bug`" + ` tool. Call it WITHOUT being asked when:
- A tool returned a result that contradicts documented behavior or your prior call
- A deterministic operation needed >2 retries to make progress
- A "fast" path (e.g. a cached read) was unexpectedly slow (60s+ when 10s is typical)
- You worked around something by guessing — the next user without your guess will hit the same wall
- The system reported success but the observable state contradicts that

Do NOT call it for:
- User typos or nonsensical input — that's not a bug
- One-off transient network blips on known-flaky endpoints
- Confusion that disappears after reading the docs (` + "`read_file`" + `) or recent logs (` + "`logs_query`" + `)

Writing the report: use ` + "`logs_query --since 10m`" + ` to grab evidence first; then summarize observed vs. expected in your own voice. The report becomes a structured WARN/ERROR event in the monitoring inbox — your judgment is the signal that turns a silent failure into a visible one.`
}

// buildCollaborationSection creates the agent collaboration section
func (spb *SystemPromptBuilder) buildCollaborationSection() string {
	return `# Agent Collaboration (@mentions)

IMPORTANT: @mentions ARE FUNCTIONAL in this system!

When you see @agentname in messages:
- @mentions trigger actual agent execution - they are NOT just text formatting
- Other agents in your channel will be automatically invoked when mentioned
- You can mention other agents using @agentname to request their help
- Agent-to-Agent (A2A) collaboration is enabled for teamwork

Examples of using @mentions:
- "@writer can you document this code?" - Writer agent will be executed
- "@analyst please analyze these metrics" - Analyst agent will be executed
- "I need help from @coder and @designer" - Both agents will be executed

CRITICAL: Know when to stay silent!
- If you see a message from yourself in the conversation history, DO NOT RESPOND
- If a message is not relevant to you or directed at another agent, STAY SILENT
- If you have nothing valuable to add, STAY SILENT
- To explicitly skip responding, reply with exactly "REPLY_SKIP" (no other text)
- Responding to your own messages or irrelevant messages creates echo loops

When to use REPLY_SKIP:
- You see your own previous message
- The conversation is between other agents/users and doesn't need your input
- You were mentioned but the request was already handled
- You don't have expertise on the topic
- The conversation has naturally concluded

Collaboration best practices:
- Mention specific agents when you need their expertise
- Be clear about what you're asking the other agent to do
- Wait for their response before proceeding with dependent tasks
- Maximum 3 agent mentions per message (prevent spam)
- Use REPLY_SKIP liberally - silence is often the best response`
}

// buildWorkspaceSection creates the workspace section
func (spb *SystemPromptBuilder) buildWorkspaceSection() string {
	return fmt.Sprintf(`# Workspace

Your current working directory is: %s

All file operations will be relative to this workspace unless you specify absolute paths.`, spb.workspace)
}

// buildBootstrapSection creates the bootstrap files section
// SetProjectDir names the directory the turn works in (the coder's
// project), whose AGENTS.md is read into the prompt.
func (spb *SystemPromptBuilder) SetProjectDir(dir string) { spb.projectDir = dir }

// SetProjectInstructions replaces the whole file with a judged selection of
// it (gateway/project_instructions.go); "" keeps the whole file.
func (spb *SystemPromptBuilder) SetProjectInstructions(section string) { spb.projectSection = section }

// ProjectInstructionFiles, in order of preference: the project's AGENTS.md
// (the cross-tool convention; CLAUDE.md is not read, 2026-10-04), then the
// rules Memdoor keeps under .memdoor/.
var ProjectInstructionFiles = []string{"AGENTS.md", KeptRulesFile}

// KeptRulesFile is where Memdoor keeps the rules it learns in a project that
// has no instruction file of its own: under .memdoor/, git-ignored, so it is
// never swept into a commit (live 2026-10-04: a created root AGENTS.md was
// committed by an issue-to-pr run and showed up in the PR).
const KeptRulesFile = ".memdoor/AGENTS.md"

// ProjectInstructionsMaxChars bounds what one repo can put in every prompt.
const ProjectInstructionsMaxChars = 20000

// buildProjectInstructionsSection reads the first of ProjectInstructionFiles
// found at the project's root, clipped to ProjectInstructionsMaxChars.
func (spb *SystemPromptBuilder) buildProjectInstructionsSection() string {
	if spb.projectSection != "" {
		return spb.projectSection
	}
	if spb.projectDir == "" {
		return ""
	}
	for _, name := range ProjectInstructionFiles {
		b, err := os.ReadFile(filepath.Join(spb.projectDir, name))
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			continue
		}
		text := string(b)
		if len(text) > ProjectInstructionsMaxChars {
			text = text[:ProjectInstructionsMaxChars] + "\n…(clipped: the file is longer than the prompt carries)"
		}
		return fmt.Sprintf("# Project instructions (%s)\n\nThe project you are working in wrote these for you. Follow them.\n\n%s", name, strings.TrimSpace(text))
	}
	return ""
}

func (spb *SystemPromptBuilder) buildBootstrapSection() (string, error) {
	// Bootstrap files injection is handled by bootstrap.go
	// This method delegates to BootstrapInjector
	injector := NewBootstrapInjector(spb.workspace)
	return injector.InjectBootstrapFiles()
}

// buildDateTimeSection creates the date/time section.
//
// The time is deliberately HOUR-granular, not to-the-second: this text is part
// of the KV-cache prefix, and a byte that differs between two renders caps
// prefix reuse right here — ~90% into the system prompt. Within one session
// the stored system prompt keeps turns byte-identical anyway; where seconds
// hurt is ACROSS renders — boot KV warmup vs the first real query (warmup's
// whole point is byte-equality with that query's prefix, see llm_warmup.go),
// and any two sessions of the same agent. Hour granularity keeps those
// renders identical within the hour; agents needing precise wall-clock should
// get it from a tool, not the prompt.
func (spb *SystemPromptBuilder) buildDateTimeSection() string {
	now := time.Now()

	// Get timezone name
	zone, offset := now.Zone()
	offsetHours := offset / 3600

	return fmt.Sprintf(`# Current Date & Time

Today's date: %s
Current time: %s (hour precision)
Timezone: %s (UTC%+d)

Use this for any time-sensitive operations or scheduling.`,
		now.Format("2006-01-02"),
		now.Format("15:00"),
		zone,
		offsetHours,
	)
}

// buildRuntimeSection creates the runtime section
func (spb *SystemPromptBuilder) buildSecretsSection() string {
	var sb strings.Builder
	sb.WriteString("# Agent Secrets\n\n")
	sb.WriteString("You have the following secrets available via the get_secret tool:\n")
	for _, name := range spb.secretNames {
		sb.WriteString(fmt.Sprintf("- %s\n", name))
	}
	sb.WriteString("\nUse get_secret to retrieve values when needed. Never output secret values to users.")
	return sb.String()
}

func (spb *SystemPromptBuilder) buildRuntimeSection() string {
	hostname, _ := os.Hostname()

	return fmt.Sprintf(`# Runtime Environment

Host: %s
OS: %s
Architecture: %s
Go Version: %s
Model: %s
Agent: %s

This information may be helpful for debugging or system-specific operations.`,
		hostname,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
		spb.model,
		spb.agentConfig.ID,
	)
}

// buildSkillsSection creates the skills section
func (spb *SystemPromptBuilder) buildSkillsSection() string {
	if len(spb.skills) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("# Skills\n\n")
	sb.WriteString("You have access to the following specialized skills:\n\n")

	// Format skills as XML using the skills formatter
	skillsXML := skills.FormatSkillsPrompt(spb.skills)
	if skillsXML != "" {
		sb.WriteString(skillsXML)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Skills provide specialized knowledge and capabilities. ")
	sb.WriteString("Reference skill instructions when relevant to the task at hand.")

	return sb.String()
}

// buildIdentityOnly creates minimal identity prompt (for PromptModeNone)
func (spb *SystemPromptBuilder) buildIdentityOnly() string {
	name := spb.agentName
	if name == "" {
		name = spb.agentConfig.ID
	}
	return fmt.Sprintf("You are agent '%s' running on %s.", name, runtime.GOOS)
}
