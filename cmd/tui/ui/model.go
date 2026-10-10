package ui

import (
	"memdoor/pkg/workflow"
	"os"
	"strings"
	"time"

	"memdoor/tools"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the TUI state
type Model struct {
	gatewayAddr   string
	token         string            // bearer token for the /ws handshake (empty = rely on loopback)
	ws            *WebSocketAdapter // Now using common client via adapter
	connected     bool
	everConnected bool // distinguishes first connect from a RECONNECT after a drop
	sessionKey    string
	// The conversation lives in a workspace's channel. poster posts the
	// user's turn through POST /api/messages ("@agent …"), so the run loads
	// the agent's tools; the websocket only receives the run's streamed
	// events. Injected by cmd/cli (which owns the authed HTTP client). A nil
	// poster falls back to the raw ws chat (tests).
	workspace string
	channelID string
	poster    func(agent, workspace, text, mode string) error
	// Network-effect ops (injected by cmd/cli, which owns the authed client).
	// Model ops (injected by cmd/cli).
	statusOp           func() Status                                  // footer status: where you are, what is thinking, what it costs
	usage              func() (string, error)                         // /usage — what this workspace has used
	remote             func(arg string) (string, error)               // /remote — this conversation on another device
	share              func(arg string) (string, error)               // /share, /unshare — a read-only link to this conversation
	fresh              func(agent string) error                       // /fresh, /clear — wipe the agent's memory of this conversation
	compact            func(agent, focus string) (string, error)      // /compact [focus] — summarize the conversation now
	handoff            func(agent string) (string, error)             // /handoff [request] — summarize, then start over from it
	route              func(arg string) (string, error)               // /model — the ladder, a pin, or auto
	modelCatalog       func(q string) ([]CatalogModel, error)         // /model-search — the catalogue (route_picker.go)
	modelProviders     func() ([]ProviderSummary, error)              // a bare /model-search: providers and counts
	modelHosts         func(id string) ([]ModelHost, error)           // a model's hosts, to prefer or reorder
	pinModel           func(id, sortBy, order string) (string, error) // pin a model with the picker's preference
	effortOp           func(level string) (string, error)             // Shift+Tab: set the reasoning effort
	routePicker        *routePicker                                   // the model / hosts picker, when open
	mcp                MCPOps                                         // /mcp — the person's MCP servers (mcp_panel.go)
	workflow           WorkflowOps                                    // /workflow — DAG runs on mario (workflow_panel.go)
	workflowPanel      *workflowPanelState                            // the /workflow panel, when open
	workflowRunning    string                                         // this window's run while it runs or waits: Esc stops it
	workflowLast       map[string]WorkflowRun                         // the last state drawn per run; the clock redraws from it
	mcpPanel           *mcpPanelState                                 // the /mcp panel, when open
	filesPanel         *filesPanelState                               // the /files panel (ctrl+f), when open
	mcpPrompts         map[string]MCPPrompt                           // the servers' prompts, by /server:prompt
	mcpSeen            map[string]bool                                // every MCP server name the panel has reported
	updateOp           func() (string, error)                         // /update — install the published build (update.go)
	restartAfterUpdate bool                                           // the new binary is in place: re-exec into this conversation
	status             Status                                         // last status, rendered in the footer
	title              string                                         // what this conversation is about — the first thing you asked
	pickerStart        *int                                           // first model-picker option currently drawn
	newBelow           int                                            // messages that arrived while scrolled up (jump-bar counter)
	// lastCounted is how many messages had been counted into newBelow, so a
	// streamed reply arriving in many chunks counts once per MESSAGE.
	// blocks memoises each message's rendered form so streaming does not
	// re-render the whole transcript per token. See render_cache.go.
	blocks blockCache
	// printedThrough is how many leading messages have been flushed to real
	// terminal scrollback (scrollback.go). Everything below it is frozen: the
	// viewport renders only messages[printedThrough:].
	printedThrough int
	lastCounted    int
	messages       []Message
	input          textarea.Model
	viewport       viewport.Model
	width          int
	height         int
	contextTokens  int
	contextLimit   int
	contextParts   contextParts // /context: what the window holds
	contextPercent float64
	ready          bool
	err            error
	program        *tea.Program
	// thinkSplit routes streamed <think> content to the thinking block rather
	// than the answer, tracking markers across chunk boundaries.
	thinkSplit thinkStreamSplitter
	// jsonFilter hides a tool call written as bare JSON while it streams; the
	// gateway parses it into a real frame, so showing the raw object too is
	// duplication the reader has to scroll past.
	jsonFilter jsonCallFilter
	// pendingCall names the tool whose call is currently being written but not
	// yet complete — the reason the transcript has stopped moving. Empty when
	// nothing is held; pendingCallUnnamed when a call is being written and its
	// name has not arrived yet.
	pendingCall string
	// pendingCallBytes is how much of that call has arrived so far. It grows
	// while the model works, which is what tells a streaming call from a stuck
	// one — the static label read identically for both ("it keeps saying
	// writing a bash tool call", 2026-08-31).
	pendingCallBytes  int
	isThinking        bool                         // Track if assistant is currently thinking
	turnRunning       bool                         // a turn was sent and has not ended (run complete, failure, esc)
	roundClosed       bool                         // the last round's text is complete; the next stream starts a paragraph
	interrupted       bool                         // Esc pressed: drop the in-flight run's remaining events
	queued            []string                     // turns typed while the agent was busy; sent batched when it's free (Claude-CLI style)
	thinkingStartTime time.Time                    // When thinking started
	lastEventAt       time.Time                    // Last run event received; drives the spinner watchdog (never hang forever)
	activeTools       map[string]time.Time         // Tool name -> start time (for running tools)
	activeSubagents   map[string]string            // Session ID -> task description
	subagentWork      map[string]subagentWorkState // Session ID -> what a SPAWNED run is doing (mirrored; see subagentWorkState)
	compacting        bool                         // agent is trimming/summarizing its context window
	compactedTokens   [2]int                       // [before, after] from the last compaction
	activityStartTime time.Time                    // When current activity started (for duration)
	todos             []TodoItem                   // Current todo list
	slashCommands     []string                     // Available slash commands
	showAutocomplete  bool                         // Show autocomplete dropdown
	showFileMentions  bool                         // @file picker open (mentions.go)
	fileMatches       []string                     // files matching the @query
	fileIndex         int                          // highlighted row of the @file picker
	fileList          []string                     // files under the working directory, for @mentions
	fileListAt        time.Time                    // when fileList was read
	mentionStart      int                          // where the @ being completed sits in the input
	expandTools       bool                         // ctrl+o: show tool frames untruncated
	autocompleteIndex int                          // Selected autocomplete index
	filteredCommands  []string                     // Filtered commands for autocomplete
	codeAgent         string                       // agent a coding turn is dispatched to (the coder)
	answeredBy        string                       // the model that answered the last turn, for the footer
	// lastTitle is the terminal title already set, so a tick that changes
	// nothing does not rewrite it (window_title.go).
	lastTitle string
	// blurred is true once the terminal reports the window lost focus, and
	// doneUnseen marks a turn that finished while nobody was looking — both
	// for the end-of-turn signal (window_title.go).
	blurred    bool
	doneUnseen bool
	// spinFrame and spinning drive the running mark in the title (window_title.go).
	spinFrame int
	spinning  bool
	// plan is the account's plan as the window was opened — \"pro\", \"free\", or
	// \"\" when it could not be read — for the input's hint (suggest.go).
	plan            string
	decisionsOff    bool             // no decision model: the hint says how to connect one
	background      bool             // a page not showing (pages.go): nothing printed to scrollback
	page            int              // this page's index in the App (pages.go)
	paged           bool             // the page runs inside an App: websocket messages are tagged
	pendingQuestion *pendingQuestion // interactive ask_user_question awaiting the user's choice
	connect         *connectFlow     // /connect in progress (connect_flow.go)
	connectOps      ConnectOps       // the one call /connect makes
}

// subagentWorkState is one spawned run's current activity: which agent, what
// tool it is inside (empty between calls), and how long that has been true.
//
// A spawned run emits on its OWN session and its own run id, and events are
// broadcast per session — so the screen that asked for the work saw nothing
// at all while it happened (live 2026-09-16: a long tool ran for minutes
// under a spinner still showing the parent's last act).
// The gateway mirrors the child's activity onto the requester's session
// (agent_runtime_progress.go); this is where the screen keeps it.
type subagentWorkState struct {
	agent   string
	tool    string
	seconds int
}

// pendingQuestion is an interactive ask_user_question the agent is blocked on.
type pendingQuestion struct {
	id       string
	question string
	options  []string
	index    int // highlighted option
}

// questionMsg is delivered when the agent asks an interactive question.
type questionMsg struct {
	id       string
	question string
	options  []string
}

// planProposalMsg is delivered when the agent calls exit_plan_mode — the plan text
// to render as a proposal (instead of the raw tool JSON leaking into the chat).
type planProposalMsg struct {
	plan string
}

// FirstAgent is the agent of the page the TUI opens on (pages.go): the coder.
func FirstAgent() string { return "coder" }

// permissionModeString is the server's permission_mode value. The coder ALWAYS
// runs in auto mode — it writes and runs without asking; the ONLY interruption
// point is the ask_user_question picker. The old default/acceptEdits/plan cycle
// (Claude-CLI style) is gone: with the babysit loop, honesty guards, and the
// question tool, modes only added a ritual between the user and the work.
func (m Model) permissionModeString() string {
	return "acceptEdits"
}

// modeLabel is the header badge. Always-auto: the coder works without asking;
// questions interrupt via the picker.
func (m Model) modeLabel() string {
	return "⏵⏵ auto"
}

// Message represents a chat message
type Message struct {
	Role       string // "user", "assistant", "tool_call", "tool_result"
	Content    string
	Timestamp  time.Time
	ToolID     string // tool_use id — identifies WHICH call this frame is
	ToolName   string // For tool execution messages
	ToolInput  string // Tool input parameters (JSON)
	ToolOutput string // Tool result/output
	ToolError  string // Tool error if any
	IsThinking bool   // For "thinking" state
	// Streaming is true while deltas are still arriving for this message.
	// A partially-received reply is rendered PLAIN: markdown re-wrapping
	// incomplete text on every delta is what shredded it on screen.
	Streaming bool
	// Streamed marks a tool that emitted live output deltas (bash, or any tool
	// registered as a streamingExecutor). It gates the Elapsed/Took counters, so
	// they show for streaming tools and not for instant ones. LiveOutput
	// accumulates the live tail WHILE it runs (cleared on completion, when the
	// final ToolOutput takes over); ToolTook is the wall time, set on completion.
	Streamed   bool
	LiveOutput string
	ToolTook   time.Duration
	// ToolDone is set when the result lands. The spinner used to mean "no
	// output and no error yet", so a tool that finished with nothing to say
	// — `touch a_file`, a write that answers "" — kept a spinner for the rest
	// of the session, frozen, reading as a tool that never came back.
	ToolDone bool
}

// toolSettled answers whether this frame is finished: its result landed, or it
// carries something to show (an interrupt settles a frame with an error, and a
// gateway too old to be matched by id still fills the output).
func (m Message) toolSettled() bool {
	return m.ToolDone || m.ToolOutput != "" || m.ToolError != ""
}

// loadSlashCommands loads the commands for THIS SESSION'S AGENT, plus custom
// ones found in .agents/commands/ (and the legacy .claude/commands/).
//
// The list follows the agent, not the window (ADR-0009): a command that
// means nothing to the session's agent is noise in front of the person. Same binary, same backend, same rule — what differs is who is
// answering (2026-09-05: "remove slash command we don't need anymore… or
// update them for this service").
func loadSlashCommands(agent string) []string {
	commands := []string{
		"/help",    // Built-in
		"/compact", // Built-in
		"/handoff", // summarize, then start over from the summary
		"/context", // Built-in
		"/doctor",  // Built-in
		"/agents",  // Built-in
	}
	commands = append(commands,
		"/go",               // run the planner's plan with the coder
		"/init",             // write the project's AGENTS.md
		"/mcp", "/workflow", // the MCP servers the coder can use: add, sign in, state
	)
	commands = append(commands,
		"/update",       // install the published build
		"/model",        // which model answers this conversation; pin one
		"/model-search", // find a model in the catalogue to pin
		"/connect",      // add a provider: the company gateway, Anthropic, OpenAI, Gemini, any endpoint
		"/files",        // the project's files, what this conversation touched first (ctrl+f)
		"/usage",        // what this workspace has used this month
		"/remote",       // open this conversation on another device (link + QR)
		"/fresh",        // a clean session: the agent forgets this conversation; with a request, sends it
		"/share",        // a read-only link to this conversation, end-to-end encrypted
		"/unshare",      // delete this conversation's shared links
		"/dir",          // re-root the session on another directory
		"/copy",         // copy the last interaction to the clipboard
		"/clear",
		"/new",
		"/exit",
	)

	// Load custom commands — .agents/commands/ first, legacy .claude/commands/
	// second (deduplicated so a name present in both is offered once).
	seen := map[string]bool{}
	for _, dir := range []string{".agents/commands", ".claude/commands"} {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					// Extract command name (remove .md extension)
					cmdName := "/" + strings.TrimSuffix(entry.Name(), ".md")
					if !seen[cmdName] {
						seen[cmdName] = true
						commands = append(commands, cmdName)
					}
				}
			}
		}
	}

	// Skills are slash commands under the explicit "skill:" namespace (gateway
	// expands "/skill:name" — see gateway slash-skills). The prefix keeps them
	// from colliding with built-ins/custom commands and makes the dropdown
	// self-documenting; includes skills the coder wrote itself.
	wd, _ := os.Getwd()
	for _, name := range ListedSkills(tools.AvailableSkillNames(wd)) {
		commands = append(commands, "/skill:"+name)
	}
	// The project's workflows, the same way: "/workflow:release" runs one —
	// the dropdown completes it (Greg, 2026-10-02: "this workflow slash
	// command should be autocomplete with existing workflows").
	for _, name := range workflowNames(wd) {
		commands = append(commands, "/workflow:"+name)
	}

	return commands
}

// workflowNames lists the workflows a project declares: the directories
// under .memdoor/workflows/ that hold task YAML, sorted.
func workflowNames(dir string) []string { return workflow.Names(dir) }

// filterCommands filters slash commands based on input
func filterCommands(commands []string, input string) []string {
	if input == "/" {
		// Show all commands if just "/"
		return commands
	}

	var filtered []string
	lowerInput := strings.ToLower(input)
	for _, cmd := range commands {
		if strings.HasPrefix(strings.ToLower(cmd), lowerInput) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}
