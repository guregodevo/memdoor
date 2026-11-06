package ui

import (
	"strings"

	"memdoor/pkg/workflow"
	"memdoor/tools"
)

// WHAT A COMMAND DOES, BESIDE IT (#132: "the /slash command should have
// messages otherwise the users cannot know what it does … grey messages on
// the right"). One table: /help prints it and the dropdown shows each line
// in grey beside its command, so the two never say different things. A
// workflow's line is the description `memdoor workflow` prints; a skill's is
// its own heading.
var slashHelp = []struct{ usage, help string }{
	{"/w", "The workflows and their runs; /w <name> <the task> runs one (/w team-solve <the task>)"},
	{"/init", "Write the project's AGENTS.md (build, test, conventions) so every turn starts from it"},
	{"/model", "Which model answers this conversation and why; /model 2 pins a rung, /model vendor/name pins any model, /model auto lets it go"},
	{"/model-search", "Find a model in the catalogue (id, context, cost band); /model-search vendor/name lists its hosts"},
	{"/connect", "Add a provider: OpenRouter, Anthropic, OpenAI, Gemini, a company gateway, any endpoint; /connect local for a model on this machine"},
	{"/usage", "What this workspace has used this month: model requests, tokens, cost on your key"},
	{"/files", "The project's files, what this conversation touched first (ctrl+f)"},
	{"/graph", "This turn as a graph: reads, edits, runs and what each depended on (ctrl+g)"},
	{"/mcp", "The MCP servers the coder can use: add one, sign in, see their state (/mcp add, login, test, on, off, remove, trust)"},
	{"/update", "Install the newest memdoor build; the window restarts into this conversation"},
	{"/dir", "Re-root the session on another directory (bare /dir shows the current one)"},
	{"/remote", "Open this conversation on your phone: a link and a QR code, end-to-end encrypted; /remote off revokes the link"},
	{"/share", "A read-only link to this conversation, end-to-end encrypted, secrets removed"},
	{"/unshare", "Delete this conversation's shared links"},
	{"/context", "View current context usage and status"},
	{"/compact", "Summarize the conversation now; /compact <focus> says what to keep in mind"},
	{"/handoff", "The model summarizes goals, decisions, progress and next steps; a clean session starts from it"},
	{"/agents", "List all configured agents"},
	{"/copy", "Copy the last interaction (or /copy N, or /copy all) to the clipboard"},
	{"/clear", "Wipe the agent's memory of this conversation: its next turn starts from nothing"},
	{"/fresh", "A clean session: the agent forgets this conversation (the screen stays); with a request, sends it"},
	{"/exit", "Close this window (ctrl+c does too); the gateway and the conversation stay"},
	{"/quit", "Close this window (ctrl+c does too); the gateway and the conversation stay"},
	{"/help", "Show this help message"},
}

// slashHelpText is what /help prints.
func slashHelpText() string {
	var b strings.Builder
	b.WriteString("Available slash commands:\n")
	for _, h := range slashHelp {
		if h.usage == "/quit" {
			continue // /exit's line names both
		}
		usage := h.usage
		if usage == "/exit" {
			usage = "/exit or /quit"
		}
		b.WriteString("- " + usage + " - " + h.help + "\n")
	}
	b.WriteString(`- /skill:<name> [args] - Run a named skill as the turn's task (tab-complete lists them)
- /<server>:<prompt> [args] - Run an MCP server's prompt as the turn's task (args in order, or name=value)

Keys:
- Esc - Interrupt a running turn
- Shift+Tab - The mode: auto (one turn) or team (the turn runs as team-solve)
- ctrl+o - Expand tool frames and a workflow's every step
- End - Jump to the latest message

Custom commands can be added in .agents/commands/ directory (.claude/commands/
still works); skills live in .agents/skills/ (project) and ~/.memdoor/skills/
(global).`)
	return b.String()
}

// slashDescriptions is what the dropdown shows beside each entry, filled when
// the command list is read (loadSlashCommands): the window's own goroutine
// writes it and draws from it.
var slashDescriptions = map[string]string{}

// describeSlashCommands fills slashDescriptions for these commands: the
// table's line for a built-in, the workflow's description for "/w <name>",
// the skill's heading for "/skill:<name>".
func describeSlashCommands(commands []string, dir string) {
	d := make(map[string]string, len(commands))
	for _, h := range slashHelp {
		d[h.usage] = h.help
	}
	for _, e := range workflow.Describe(dir) {
		if e.Description != "" {
			d["/w "+e.Name] = e.Description
		}
	}
	for _, c := range commands {
		if name, ok := strings.CutPrefix(c, "/skill:"); ok {
			if body, found := tools.LookupSkill(name, dir); found {
				d[c] = skillLead(body)
			}
		}
	}
	slashDescriptions = d
}

// skillLead is a skill's one line: the words in its heading's parentheses
// ("# Workflow (build a DAG of agent tasks …)"), else the heading, else the
// first line of text.
func skillLead(body string) string {
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || l == "---" || strings.HasPrefix(l, "name:") {
			continue
		}
		if d, ok := strings.CutPrefix(l, "description:"); ok {
			return strings.Trim(strings.TrimSpace(d), `"'`)
		}
		l = strings.TrimSpace(strings.TrimLeft(l, "#"))
		if i, j := strings.Index(l, "("), strings.LastIndex(l, ")"); i >= 0 && j > i+1 {
			return strings.TrimSpace(l[i+1 : j])
		}
		return l
	}
	return ""
}
