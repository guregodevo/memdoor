package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// model_lifecycle: Init, gateway connection, and the network-op commands.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.connectToGateway,
		// Ask for the status line NOW as well as on the ticker — waiting a full
		// tick means the footer is blank for the first half minute of every
		// session, which is exactly when you look at it.
		func() tea.Msg { return statusTickMsg{} },
		m.tickStatus(),
		m.mcpStartupCheck(),
		m.mcpLoadPrompts(),
	)
}

// SetProgram sets the program reference (called from main after program creation)
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// GetWebSocketClient returns the WebSocket adapter
func (m Model) GetWebSocketAdapter() *WebSocketAdapter {
	return m.ws
}

// connectToGateway attempts to connect to the gateway WebSocket
func (m Model) connectToGateway() tea.Msg {
	if err := m.ws.Connect(); err != nil {
		return websocketDisconnectedMsg{err: err}
	}
	return websocketConnectedMsg{}
}

// isNetworkCmd reports whether a slash command is one of the async network /
// background ops handled by runNetworkCmd.
func isNetworkCmd(cmd string) bool {
	switch cmd {
	case "/model-search",
		"/usage", "/meter", "/model", "/update", "/remote", "/share", "/unshare",
		"/clear", "/fresh", "/compact", "/handoff", "/mcp", "/workflow", "/connect":
		return true
	}
	return strings.HasPrefix(cmd, "/workflow:")
}

// runNetworkCmd kicks off an async op (/model, /usage, /update): it
// echoes a status line and returns a tea.Cmd that does the slow, networked work
// off the event loop and reports back into Update.
func (m *Model) runNetworkCmd(fields []string) tea.Cmd {
	if p, ok := m.mcpPromptFor(fields[0]); ok {
		return m.runMCPPrompt(p, fields[1:])
	}
	if name, ok := strings.CutPrefix(fields[0], "/workflow:"); ok && name != "" {
		partition := ""
		if len(fields) > 1 {
			partition = fields[1]
		}
		return m.workflowStart(name, partition)
	}
	switch fields[0] {
	case "/connect":
		return m.connectCommand(fields[1:])
	case "/mcp":
		return m.mcpCommand(fields[1:])
	case "/workflow":
		return m.workflowCommand(fields[1:])
	case "/model-search":
		// The model catalogue (every connected provider's): find a
		// model by name, then /model <id> pins it for this conversation
		// (Greg, 2026-09-26: "user can change models").
		if len(fields) < 2 {
			// Nothing typed: a picker of the providers, then one's models —
			// the person need not know a provider's name.
			return m.openProviderPicker()
		}
		query := strings.Join(fields[1:], " ")
		if strings.Contains(query, "/") && !strings.Contains(query, " ") {
			return m.openModelHosts(query) // an exact id: its hosts
		}
		return m.openModelSearch(query)

	case "/model":
		// Which model answers this conversation — the agent's ladder with the
		// current rung marked — and a pin (gateway/route_handler.go). One
		// command, one meaning (Greg, 2026-09-26: "it pins a model", "less is
		// more").
		if m.route == nil {
			m.note("Routing isn't available in this build.")
			return nil
		}
		arg := strings.Join(fields[1:], " ")
		// A bare model id opens its hosts in the picker — sort, reorder,
		// enter pins. Anything else (a rung, auto, id + typed preference)
		// goes straight through.
		// "provider:model" names the provider outright (groq:qwen/qwen3.8-27b,
		// for an id two providers list) and pins at once: hosts are
		// OpenRouter's notion.
		if len(fields) == 2 && strings.Contains(arg, "/") && !hasProviderPrefix(arg) && m.modelHosts != nil {
			return m.openModelHosts(arg)
		}
		op := m.route
		return func() tea.Msg { s, e := op(arg); return routeResultMsg{summary: s, err: e} }
	case "/update":
		// Install the published build (update.go): the installer memdoor
		// upgrade uses, then this window re-executes into the conversation.
		if m.updateOp == nil {
			m.note("Updating isn't available in this build.")
			return nil
		}
		m.note("Updating memdoor … (a minute; this window restarts into the same conversation)")
		op := m.updateOp
		return func() tea.Msg { s, e := op(); return updateResultMsg{summary: s, err: e} }
	case "/remote":
		// This conversation on another device: a link and a QR code
		// (cmd/cli/cmd/tui_remote.go).
		if m.remote == nil {
			m.note("Remote control isn't available in this build.")
			return nil
		}
		arg := strings.Join(fields[1:], " ")
		op := m.remote
		return func() tea.Msg { s, e := op(arg); return noteResultMsg{summary: s, err: e} }
	case "/clear", "/fresh":
		// FRESH SESSION, KEEP THE TASK (session_fresh.go on the gateway): a
		// long conversation full of old reads degrades every later turn. Both
		// wipe what the agent remembers of this conversation — the gateway
		// cancels a running turn and waits for it to save first, so the wipe
		// is not undone a moment later. /clear also starts the screen over;
		// /fresh keeps it, and `/fresh <request>` sends that request into the
		// clean session. It never re-sends the last one by itself: "go ahead"
		// with the conversation wiped sent the coder off on a task of its own
		// making (live 2026-09-29).
		if m.fresh == nil {
			m.note("Starting over isn't available in this build.")
			return nil
		}
		op, agent := m.fresh, m.codeAgent
		if fields[0] == "/clear" {
			m.note("Wiping the agent's memory of this conversation …")
			return func() tea.Msg { return clearedMsg{err: op(agent)} }
		}
		task := strings.TrimSpace(strings.Join(fields[1:], " "))
		if m.busy() {
			m.stopLocally()
		}
		m.note("Starting over in a fresh session …")
		return func() tea.Msg { return freshMsg{task: task, err: op(agent)} }
	case "/handoff":
		// Summarize the conversation with the model, start over from the
		// summary (session_compact.go), then send the request if one came.
		if m.handoff == nil {
			m.note("Handoff isn't available in this build.")
			return nil
		}
		if m.busy() {
			m.note("A turn is running — /handoff when it ends, or Esc first.")
			return nil
		}
		op, agent, task := m.handoff, m.codeAgent, strings.TrimSpace(strings.Join(fields[1:], " "))
		m.note("Writing a handoff of this conversation …")
		return func() tea.Msg { h, e := op(agent); return freshMsg{task: task, err: e, handoff: h} }
	case "/compact":
		// Summarize the conversation now (session_compact.go on the gateway):
		// superseded tool output stubbed, all but the recent messages
		// summarized, the focus at the head of the summary. No model call.
		if m.compact == nil {
			m.note("Compacting isn't available in this build.")
			return nil
		}
		if m.busy() {
			m.note("A turn is running — /compact when it ends, or Esc first.")
			return nil
		}
		op, agent, focus := m.compact, m.codeAgent, strings.TrimSpace(strings.Join(fields[1:], " "))
		m.note("Compacting the conversation …")
		return func() tea.Msg { s, e := op(agent, focus); return noteResultMsg{summary: s, err: e} }
	case "/share", "/unshare":
		// A read-only, end-to-end encrypted link to this conversation
		// (cmd/cli/cmd/tui_share.go); /unshare deletes them.
		if m.share == nil {
			m.note("Sharing isn't available in this build.")
			return nil
		}
		arg := "off"
		if fields[0] == "/share" {
			arg = strings.Join(fields[1:], " ")
		}
		op := m.share
		return func() tea.Msg { s, e := op(arg); return noteResultMsg{summary: s, err: e} }
	case "/usage", "/meter": // /meter kept as an alias
		// USAGE, NOT MONEY. A subscription may show what has been used;
		// what it cost us is the operator's, behind their token.
		if m.usage == nil {
			m.note("Usage isn't available in this build.")
			return nil
		}
		m.note("Reading usage …")
		op := m.usage
		return func() tea.Msg { s, e := op(); return noteResultMsg{summary: s, err: e} }
	}
	return nil
}

// note appends a system message (status/notice) to the transcript AND puts it
// on screen.
//
// A NOTE NOBODY SEES IS NOT A NOTE (battle test, 2026-09-27). View renders the
// viewport's stored content, so a handler that appended a message without
// calling SetContent left it invisible until something else refreshed. That is
// how `/model not-a-vendor/not-a-model` came to say nothing at all: the error
// was raised, noted, and never drawn. Refreshing here fixes every such path at
// once, and refreshFollow keeps a scrolled-up reader where they are.
func (m *Model) note(content string) {
	m.messages = append(m.messages, Message{Role: "system", Content: content, Timestamp: time.Now()})
	if m.viewport.Height > 0 {
		m.refreshFollow()
	}
}

// hasProviderPrefix: a ":" before the first "/" — "groq:qwen/qwen3.8-27b"
// yes, "z-ai/glm-5.3-flash:batch" no.
func hasProviderPrefix(id string) bool {
	c, sl := strings.Index(id, ":"), strings.Index(id, "/")
	return c > 0 && (sl < 0 || c < sl)
}
