package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"memdoor/tools"
	"regexp"

	"github.com/atotto/clipboard"
)

// model_slash: slash-command handling and the /context /doctor /agents status views.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// handleSlashCommand processes slash commands
// Returns (expandedText, handled) where:
// - handled=true means command was executed locally (don't send to gateway)
// - handled=false means command was expanded from a .agents/commands/ file (send expanded text to gateway)
func (m *Model) handleSlashCommand(text string) (string, bool) {
	// Parse command and arguments
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return text, false
	}
	cmdName := parts[0]

	// Handle built-in commands locally
	switch cmdName {
	case "/init":
		// The task every coding agent offers: write the project's AGENTS.md,
		// the file the coder reads at the top of every turn from now on
		// (gateway/prompts, project instructions). The coder does the
		// looking and the writing; this is only the brief.
		return initPrompt, false
	case "/help":
		helpText := `Available slash commands:
- /init - Write the project's AGENTS.md (build, test, conventions) so every turn starts from it
- /update - Install the newest memdoor build; the window restarts into this conversation
- /dir <path> - Re-root the session on another directory (bare /dir shows the current one)
- /model - Which model answers this conversation and why; /model 2 pins a rung, /model vendor/name pins any model, /model auto lets it go
- /model vendor/name price|throughput|latency|default - How its hosts are chosen; /model vendor/name order host1,host2 - your own host order
- /model-search <name> - Find a model in the catalogue (id, context, cost band); /model-search vendor/name lists its hosts
- /usage - What this workspace has used this month: model requests, tokens, cost on your key
- /remote - Open this conversation on your phone: a link and a QR code, end-to-end encrypted; /remote off revokes the link
- /share - A read-only link to this conversation, end-to-end encrypted, secrets removed; /unshare deletes it
- /context - View current context usage and status
- /compact [focus] - Summarize the conversation now; the focus says what to keep in mind
- /handoff [request] - The model summarizes goals, decisions, progress and next steps; a clean session starts from it
- /doctor - Run diagnostics on TUI session health
- /agents - List all configured agents
- /copy [N|all] - Copy the last interaction (or last N, or all) to the clipboard
- /clear - Wipe the agent's memory of this conversation: its next turn starts from nothing
- /fresh [request] - A clean session: the agent forgets this conversation (the screen stays); with a request, sends it
- /new - Start a new session (requires restart)
- /exit - Exit the TUI (same as Ctrl+C or Esc)
- /help - Show this help message
- /skill:<name> [args] - Run a named skill as the turn's task (tab-complete lists them)
- /mcp - The MCP servers the coder can use: add one, sign in, see their state (/mcp add, login, test, on, off, remove, trust)
- /<server>:<prompt> [args] - Run an MCP server's prompt as the turn's task (args in order, or name=value)

Keys:
- Esc - Interrupt a running turn
- ctrl+o - Expand tool frames (full tool output instead of the truncated preview)
- End - Jump to the latest message
- Shift+Tab - Cycle the reasoning effort (auto, low, medium, high)

Custom commands can be added in .agents/commands/ directory (.claude/commands/
still works); skills live in .agents/skills/ (project) and ~/.memdoor/skills/
(global).`

		// Add help message as assistant response
		helpMsg := Message{
			Role:      "assistant",
			Content:   helpText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, helpMsg)
		return "", true // handled locally

	case "/doctor":
		doctorText := m.getDoctorStatus()
		// Add doctor status as assistant response
		doctorMsg := Message{
			Role:      "assistant",
			Content:   doctorText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, doctorMsg)
		return "", true // handled locally

	case "/agents":
		agentsText := m.getAgentsList()
		// Add agents list as assistant response
		agentsMsg := Message{
			Role:      "assistant",
			Content:   agentsText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, agentsMsg)
		return "", true // handled locally

	case "/copy":
		// Copy recent conversation to the system clipboard (like Claude CLI's
		// /copy) so you can paste it into a bug report, a PR, or a message.
		// `/copy` = the last interaction (the last user turn + its response and
		// tool activity); `/copy N` = the last N interactions; `/copy all` = the
		// whole transcript.
		// The /copy invocation is itself echoed as the last user message before
		// this runs. Drop it (and any other trailing slash-command echoes) so
		// /copy copies the real interaction, not itself.
		end := len(m.messages)
		for end > 0 && isSlashEcho(m.messages[end-1]) {
			end--
		}
		work := m.messages[:end]
		if len(work) == 0 {
			m.note("Nothing to copy yet.")
			return "", true
		}
		from := startOfLastInteractions(work, 1)
		label := "the last interaction"
		if len(parts) > 1 {
			switch {
			case strings.EqualFold(parts[1], "all"):
				from, label = 0, "the full transcript"
			default:
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					from = startOfLastInteractions(work, n)
					label = fmt.Sprintf("the last %d interactions", n)
				}
			}
		}
		text := formatTranscriptForCopy(work[from:])
		if strings.TrimSpace(text) == "" {
			m.note("Nothing to copy yet.")
			return "", true
		}
		if err := clipboard.WriteAll(text); err != nil {
			m.note("Couldn't copy to the clipboard: " + err.Error())
			return "", true
		}
		m.note(fmt.Sprintf("Copied %s to the clipboard (%d lines).", label, strings.Count(text, "\n")+1))
		return "", true

	case "/context":
		contextText := m.getContextStatus()
		// Add context status as assistant response
		contextMsg := Message{
			Role:      "assistant",
			Content:   contextText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, contextMsg)
		return "", true // handled locally

	case "/new":
		newSessionText := `To start a new session:
1. Exit the TUI (press Ctrl+C or Esc)
2. Restart with: ./memdoor tui

Your current session history is preserved on disk.`

		// Add new session info as assistant response
		newSessionMsg := Message{
			Role:      "assistant",
			Content:   newSessionText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, newSessionMsg)
		return "", true // handled locally

	case "/dir":
		// Re-root the session on another directory without relaunching. The
		// poster sends the PROCESS directory with every turn, so os.Chdir is
		// the whole switch: the agent's confinement, cuts and previews follow.
		// Live 2026-09-02: a session launched one folder away from the media
		// had to be quit and relaunched to reach it.
		cwd, _ := os.Getwd()
		if len(parts) < 2 {
			m.messages = append(m.messages, Message{Role: "assistant",
				Content:   "Session directory: " + cwd + "\nUsage: /dir <path> — the agent's next turn works there (its files, cuts and previews land in it)",
				Timestamp: time.Now()})
			return "", true
		}
		dir := expandHome(parts[1])
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			m.messages = append(m.messages, Message{Role: "assistant",
				Content: fmt.Sprintf("Cannot use %s: not a directory", dir), Timestamp: time.Now()})
			return "", true
		}
		if err := os.Chdir(dir); err != nil {
			m.messages = append(m.messages, Message{Role: "assistant",
				Content: fmt.Sprintf("Cannot switch to %s: %v", dir, err), Timestamp: time.Now()})
			return "", true
		}
		now, _ := os.Getwd()
		m.messages = append(m.messages, Message{Role: "assistant",
			Content: "Session directory is now " + now + " — the agent's next turn works there.", Timestamp: time.Now()})
		return "", true

	case "/exit":
		exitText := `Goodbye! Your session has been saved.`

		// Add exit message as assistant response
		exitMsg := Message{
			Role:      "assistant",
			Content:   exitText,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, exitMsg)
		return "", true // handled locally (quit will be triggered in Update)

	default:
		// Custom command, agent-agnostic (2026-09-30): .agents/commands/ is
		// the convention; .claude/commands/ still works as a fallback.
		cmdFileName := strings.TrimPrefix(cmdName, "/") + ".md"
		for _, dir := range []string{".agents/commands", ".claude/commands"} {
			if content, err := os.ReadFile(filepath.Join(dir, cmdFileName)); err == nil {
				return string(content), false
			}
		}

		// SKILL COMMAND: "/skill:name" passes through UNEXPANDED — the gateway
		// is the single expansion point (agent_adapter slash-skills), so TUI
		// and CLI messages get the identical injection. The TUI's cwd is the
		// coder workdir, so resolution here agrees with the gateway's. An
		// unresolvable skill name errors locally with the available list
		// instead of burning an LLM turn.
		if name, ok := strings.CutPrefix(cmdName, "/skill:"); ok {
			wd, _ := os.Getwd()
			if _, found := tools.LookupSkill(name, wd); found {
				return text, false
			}
			m.messages = append(m.messages, Message{
				Role:      "assistant",
				Content:   fmt.Sprintf("Unknown skill: %q. Available: /skill:%s", name, strings.Join(ListedSkills(tools.AvailableSkillNames(wd)), ", /skill:")),
				Timestamp: time.Now(),
			})
			return "", true
		}

		// Unknown command - show error. A `/server:prompt` whose server is
		// simply off read as "Unknown slash command" a minute after the
		// person used it: name the server instead.
		content := fmt.Sprintf("Unknown slash command: '%s'\n\nUse /help to see available commands.", cmdName)
		if server, _, isPair := strings.Cut(strings.TrimPrefix(cmdName, "/"), ":"); isPair && m.knownMCPServer(server) {
			content = fmt.Sprintf("**%s** is not connected, so its prompts are not loaded. Open **/mcp** to turn it on or test it.", server)
		}
		errorMsg := Message{
			Role:      "assistant",
			Content:   content,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, errorMsg)
		return "", true // handled locally (error shown)
	}
}

// getContextStatus returns the current context usage status
func (m *Model) getContextStatus() string {
	var response strings.Builder

	// No report yet: the gateway's first context event arrives with the first
	// turn (and at subscribe, when the gateway can measure the baseline).
	// Before it, showing "0 / 200.0k" invented a window that was really the
	// compaction ceiling. Say nothing is known instead.
	if m.contextTokens == 0 && m.contextLimit == 0 {
		response.WriteString("## Context Usage\n\n")
		response.WriteString("No context report yet — the gateway measures the window (system prompt + tool definitions) when a turn starts. Run a turn, then `/context` again.\n")
		return response.String()
	}

	// Header
	response.WriteString("## Context Usage\n\n")

	// Current usage
	response.WriteString(fmt.Sprintf("**Tokens**: %s / %s (%.1f%%)\n\n",
		formatTokens(m.contextTokens),
		formatTokens(m.contextLimit),
		m.contextPercent))

	// Visual progress bar
	barWidth := 40
	filledWidth := int(float64(barWidth) * (m.contextPercent / 100.0))
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	// Color indicator
	var indicator string
	if m.contextPercent >= 90 {
		indicator = ""
	} else if m.contextPercent >= 60 {
		indicator = ""
	} else {
		indicator = ""
	}

	response.WriteString(fmt.Sprintf("%s %s\n\n", indicator, bar))

	// What is eating it: tool output is what compaction gives up first.
	if p := m.contextParts; p.system+p.tools+p.conversation > 0 {
		response.WriteString(fmt.Sprintf("- System prompt: %s\n", formatTokens(p.system)))
		response.WriteString(fmt.Sprintf("- Tool definitions: %s\n", formatTokens(p.tools)))
		response.WriteString(fmt.Sprintf("- Conversation: %s, of which tool output %s\n\n",
			formatTokens(p.conversation), formatTokens(p.toolOutput)))
		switch left := p.compactAt - m.contextTokens; {
		case strings.HasPrefix(p.compactRule, "off"):
			response.WriteString("Compaction at a threshold is " + p.compactRule + "; a turn over the window is still fitted. ")
		case p.compactAt > 0 && left > 0:
			response.WriteString(fmt.Sprintf("Compacts at %s (%s): %s to go. ", formatTokens(p.compactAt), p.compactRule, formatTokens(left)))
		case p.compactAt > 0:
			response.WriteString(fmt.Sprintf("Past %s (%s): the next turn compacts. ", formatTokens(p.compactAt), p.compactRule))
		}
	}
	response.WriteString("Compaction drops spent tool calls and stubs superseded output first, then summarizes older messages; " +
		"`/compact [focus]` does it now")
	if k := m.contextParts.keepRecent; k > 0 {
		response.WriteString(fmt.Sprintf(", keeping the last %s word for word", formatTokens(k)))
	}
	response.WriteString(". Settings: `compaction` in ~/.memdoor/config.json (enabled, compactionPercent, thresholdTokens, keepRecentTokens).\n")

	return response.String()
}

// getDoctorStatus returns TUI session health diagnostics
func (m *Model) getDoctorStatus() string {
	var response strings.Builder

	// Header
	response.WriteString("## TUI Session Diagnostics\n\n")

	// Connection Status
	if m.connected {
		response.WriteString("**Gateway Connection**: Connected\n")
		response.WriteString(fmt.Sprintf("   Session: %s\n\n", m.sessionKey))
	} else {
		response.WriteString("**Gateway Connection**: Disconnected\n\n")
	}

	// Session Information
	response.WriteString(fmt.Sprintf("**Messages**: %d in current session\n", len(m.messages)))
	// Same "no report yet" rule as /context: before the gateway's first
	// context event there is no window to quote — 200.0k was the compaction
	// ceiling, not the model's.
	if m.contextTokens == 0 && m.contextLimit == 0 {
		response.WriteString("**Context**: no report yet — run a turn\n\n")
	} else {
		response.WriteString(fmt.Sprintf("**Context**: %s / %s (%.1f%%)\n\n",
			formatTokens(m.contextTokens),
			formatTokens(m.contextLimit),
			m.contextPercent))
	}

	// Active Operations
	if len(m.activeTools) > 0 {
		response.WriteString(fmt.Sprintf("**Active Tools**: %d running\n", len(m.activeTools)))
	}
	if len(m.activeSubagents) > 0 {
		response.WriteString(fmt.Sprintf("**Active Subagents**: %d running\n", len(m.activeSubagents)))
	}
	if len(m.todos) > 0 {
		response.WriteString(fmt.Sprintf("**Todos**: %d items tracked\n", len(m.todos)))
	}

	// Overall Health
	response.WriteString("\n")
	if m.connected && m.contextPercent < 90 {
		response.WriteString("**Overall Health**: Good\n")
		response.WriteString("- Gateway connected\n")
		response.WriteString("- Context healthy\n")
	} else if !m.connected {
		response.WriteString("**Overall Health**: Gateway disconnected\n")
		response.WriteString("- Check gateway is running: `./memdoor gateway`\n")
	} else if m.contextPercent >= 90 {
		response.WriteString("**Overall Health**: Context near limit\n")
		response.WriteString("- AI compaction will trigger soon\n")
		response.WriteString(fmt.Sprintf("- Context at %.1f%% utilization\n", m.contextPercent))
	}

	return response.String()
}

// getAgentsList returns list of configured agents by calling the agents_list tool
func (m *Model) getAgentsList() (out string) {
	// A slash command must never crash the TUI — recover any panic (e.g. the
	// tools package's "gateway base URL not set") into an inline error.
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("Error loading agents: %v", r)
		}
	}()
	// Call agents_list tool
	result, err := tools.AgentsList(json.RawMessage("{}"))
	if err != nil {
		return fmt.Sprintf("Error loading agents: %s", err.Error())
	}

	// Parse the JSON response
	type AgentInfo struct {
		ID          string   `json:"id"`
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Workspace   string   `json:"workspace,omitempty"`
		Tools       []string `json:"tools,omitempty"`
		Skills      []string `json:"skills,omitempty"`
	}

	type AgentsResponse struct {
		Agents    []AgentInfo `json:"agents"`
		DefaultID string      `json:"default_id,omitempty"`
	}

	var agentsResp AgentsResponse
	if err := json.Unmarshal([]byte(result), &agentsResp); err != nil {
		return fmt.Sprintf("Error parsing agents list: %s", err.Error())
	}

	// Build formatted response
	var response strings.Builder
	response.WriteString("## Configured Agents\n\n")

	if len(agentsResp.Agents) == 0 {
		response.WriteString("No agents configured.\n")
		return response.String()
	}

	// Show default agent if set
	if agentsResp.DefaultID != "" {
		response.WriteString(fmt.Sprintf("**Default Agent**: %s\n\n", agentsResp.DefaultID))
	}

	// List each agent
	for i, agent := range agentsResp.Agents {
		// Agent header with ID
		isDefault := agent.ID == agentsResp.DefaultID
		if isDefault {
			response.WriteString(fmt.Sprintf("### %s (default)\n", agent.ID))
		} else {
			response.WriteString(fmt.Sprintf("### %s\n", agent.ID))
		}

		// Name if different from ID
		if agent.Name != "" && agent.Name != agent.ID {
			response.WriteString(fmt.Sprintf("**Name**: %s\n", agent.Name))
		}

		// Description
		if agent.Description != "" {
			response.WriteString(fmt.Sprintf("**Description**: %s\n", agent.Description))
		}

		// Workspace
		if agent.Workspace != "" {
			response.WriteString(fmt.Sprintf("**Workspace**: %s\n", agent.Workspace))
		}

		// Tools count
		if len(agent.Tools) > 0 {
			response.WriteString(fmt.Sprintf("**Tools**: %d configured\n", len(agent.Tools)))
		}

		// Skills
		if len(agent.Skills) > 0 {
			response.WriteString(fmt.Sprintf("**Skills**: %d loaded\n", len(agent.Skills)))
		}

		// Add separator between agents
		if i < len(agentsResp.Agents)-1 {
			response.WriteString("\n")
		}
	}

	return response.String()
}

// isSlashEcho reports whether a message is a slash-command invocation echoed
// into the transcript (e.g. "> /copy"). These are TUI chrome, not part of the
// conversation to copy.
func isSlashEcho(msg Message) bool {
	return msg.Role == "user" && looksLikeSlashCommand(msg.Content)
}

// slashCommandName is what a command looks like: "/help", "/skill:review",
// "/workflow release". A second slash or a dot in the first word means a
// path — "/Users/sara/app/main.go fix this" is a file and a request, not an
// unknown slash command (2026-09-12).
var slashCommandName = regexp.MustCompile(`^/[A-Za-z][A-Za-z0-9_:-]*$`)

// looksLikeSlashCommand reports whether text is a command invocation rather
// than a message that happens to start with "/".
func looksLikeSlashCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	return slashCommandName.MatchString(fields[0])
}

// startOfLastInteractions returns the index where the last n interactions begin.
// An interaction starts at a real (non slash-command) user message; everything
// after it (the assistant's reply, tool frames, notes) belongs to it. Fewer than
// n interactions → 0.
func startOfLastInteractions(msgs []Message, n int) int {
	seen := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && !isSlashEcho(msgs[i]) {
			seen++
			if seen == n {
				return i
			}
		}
	}
	return 0
}

// formatTranscriptForCopy renders messages as plain text (no ANSI) suitable for
// pasting elsewhere: the user turn, the assistant's reply, and each tool call
// with its output or error.
func formatTranscriptForCopy(msgs []Message) string {
	var b strings.Builder
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			b.WriteString("> " + strings.TrimSpace(msg.Content) + "\n\n")
		case "assistant":
			if msg.IsThinking {
				continue
			}
			if c := strings.TrimSpace(msg.Content); c != "" {
				b.WriteString(c + "\n\n")
			}
		case "system":
			if c := strings.TrimSpace(msg.Content); c != "" {
				b.WriteString(c + "\n\n")
			}
		case "workflow":
			// The DAG block is drawn for the screen; the clipboard gets the words.
			if c := strings.TrimSpace(plainText(msg.Content)); c != "" {
				b.WriteString(c + "\n\n")
			}
		case "tool_call":
			view := viewFor(msg.ToolName)
			b.WriteString("⏺ " + view.Label(msg.ToolInput, 120) + "\n")
			// The same rules the SCREEN uses, minus colour. /copy is how
			// rendering is inspected, and it used to dump raw tool output
			// instead: the live view said "Read 17 lines" and drew a diff
			// while the clipboard got the whole file twice plus the patch
			// echo (measured 2026-08-31).
			body := view.Body(ToolRender{
				Input: msg.ToolInput, Output: msg.ToolOutput, Err: msg.ToolError,
				Width: 120,
			})
			switch {
			case msg.ToolError != "":
				b.WriteString("  Error: " + strings.TrimSpace(msg.ToolError) + "\n")
			case body != "":
				// The view said what happened — a diff, a summary. The raw
				// output underneath is the model's working copy (the echoed
				// file), suppressed on screen and suppressed here too.
				b.WriteString(plainText(body) + "\n")
			case strings.TrimSpace(msg.ToolOutput) != "":
				for _, ln := range strings.Split(strings.TrimSpace(msg.ToolOutput), "\n") {
					b.WriteString("  " + ln + "\n")
				}
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// ansiSeq matches colour/style escapes so rendered frames can be copied as
// plain text — the frames come from lipgloss, and a transcript pasted into a
// PR must not carry terminal codes.
var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

func plainText(s string) string { return ansiSeq.ReplaceAllString(s, "") }

// expandHome turns a leading "~/" into the home directory. The shell does
// this before a command ever sees it, so a path typed INTO the TUI ("/cd
// ~/project") reaches os.Stat with a literal tilde and fails as "no such
// file" for a directory that plainly exists (live 2026-09-02).
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// initPrompt is the brief /init sends to the coder.
const initPrompt = "Write this project's AGENTS.md at the repository root — the instructions a coding agent reads before every turn. " +
	"First look: the build, test and lint commands that actually work here (run them), the layout of the code, the conventions you can " +
	"see (naming, error handling, how tests are written, what must never be edited), and any existing AGENTS.md or CONTRIBUTING " +
	"whose content should be kept. Then write AGENTS.md: short (under 80 lines), concrete, commands verbatim, no filler; if one exists, " +
	"improve it rather than replacing it. Finish by showing the file."
