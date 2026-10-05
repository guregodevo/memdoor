package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// model_ops: construction, dependency setters, chips, and the turn-dispatch commands.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// NewModel creates a new TUI model.
//
// The conversation is scoped to a workspace: the websocket subscribes by
// workspace + channel (the gateway builds the session key), and poster sends
// each turn scoped to it. A nil poster falls back to the raw ws chat (tests).
func NewModel(gatewayAddr, workspace, channelID, token string, poster func(agent, workspace, text, mode string) error) Model {
	return NewPage(PageConfig{GatewayAddr: gatewayAddr, Workspace: workspace, ChannelID: channelID, Token: token, Poster: poster, Agent: FirstAgent()})
}

// Ops are the operations a page reaches the outside world with: the authed
// HTTP client lives in cmd/cli, so cmd/cli hands them in. A nil op means
// "not in this build" and the command says so. Everything a page needs is
// here at construction — no setters afterwards, so a page is consistent
// from the moment it exists (Greg, 2026-09-26: "fail fast constructor",
// "at build time its state is consistent").
type Ops struct {
	Usage          func() (string, error)                         // /usage
	Remote         func(arg string) (string, error)               // /remote, /remote off
	Share          func(arg string) (string, error)               // /share, /unshare
	Fresh          func(agent string) error                       // /fresh, /clear: wipe the agent's memory of this conversation
	Compact        func(agent, focus string) (string, error)      // /compact [focus]: summarize the conversation now
	Handoff        func(agent string) (string, error)             // /handoff [request]: summarize, then start over from it
	Route          func(arg string) (string, error)               // /model (typed forms)
	ModelCatalog   func(q string) ([]CatalogModel, error)         // /model-search: the catalogue
	ModelHosts     func(id string) ([]ModelHost, error)           // a model's hosts
	ModelProviders func() ([]ProviderSummary, error)              // a bare /model-search: the providers and their counts
	PinModel       func(id, sortBy, order string) (string, error) // pin from the picker
	Effort         func(level string) (string, error)             // Shift+Tab: reasoning effort, auto | low | medium | high
	Status         func() Status                                  // the footer
	Update         func() (string, error)                         // /update — install the published build
	MCP            MCPOps                                         // /mcp — the person's MCP servers (mcp_panel.go)
	Workflow       WorkflowOps                                    // /workflow — DAG runs on mario (workflow_panel.go)
	Connect        ConnectOps                                     // /connect — add a provider (connect_flow.go)
}

// Resume is a conversation being rejoined: its title and the transcript so
// far, oldest first.
type Resume struct {
	Title   string
	History []Message
}

// PageConfig is everything a page is built from.
type PageConfig struct {
	GatewayAddr string
	Workspace   string // the conversation's scope: the session key and every turn carry it
	Plan        string // "pro", "free" or "" (unknown): which hint the empty input shows
	// DecisionsOff: no decision model judges this gateway's turns; the empty
	// input says how to connect one (suggest.go).
	DecisionsOff bool
	ChannelID    string
	Token        string
	Agent        string
	Poster       func(agent, workspace, text, mode string) error
	Ops          Ops
	Resume       Resume
	// Page is this page's index in the App (pages.go); Paged says the page
	// runs inside one, so the websocket's messages are tagged with it.
	Page  int
	Paged bool
}

// NewPage is a page for one agent (pages.go): its conversation, its look,
// its commands, complete at construction. A zero PageConfig is a working
// page with good defaults: the launch mode's agent, no ops (each command
// then says it is not in this build), a fresh conversation.
func NewPage(cfg PageConfig) Model {
	if cfg.Agent == "" {
		cfg.Agent = FirstAgent()
	}
	// Create textarea for input
	ti := textarea.New()
	ti.Placeholder = ""
	ti.Prompt = "> "
	// Plain input like Claude CLI: no shaded current-line highlight.
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ti.Focus()
	ti.CharLimit = 4000
	ti.SetWidth(100)
	ti.SetHeight(1) // Single line to avoid placeholder duplication
	ti.ShowLineNumbers = false

	// Create viewport for messages
	vp := viewport.New(100, 20)
	vp.KeyMap = transcriptKeys()
	vp.SetContent("")

	// Create WebSocket adapter (uses common client library). The session key
	// prefix names the agent the TUI talks to — the companion is the
	// conversational front door (grounded recall + capture + delegate). "main"
	// is NOT a seeded buddy and fails with "buddy not found".
	ws := NewWebSocketAdapter(cfg.GatewayAddr, cfg.Workspace, cfg.ChannelID, cfg.Token)

	// The commands this session's agent can actually act on.
	slashCommands := loadSlashCommands(cfg.Agent)

	pickerStart := 0
	m := Model{
		// blocks memoises rendered messages so streaming does not rebuild the
		// whole transcript per token (render_cache.go).
		blocks:         blockCache{},
		gatewayAddr:    cfg.GatewayAddr,
		token:          cfg.Token,
		pickerStart:    &pickerStart,
		ws:             ws,
		sessionKey:     cfg.Workspace,
		workspace:      cfg.Workspace,
		plan:           cfg.Plan,
		decisionsOff:   cfg.DecisionsOff,
		channelID:      cfg.ChannelID,
		codeAgent:      cfg.Agent,
		poster:         cfg.Poster,
		usage:          cfg.Ops.Usage,
		remote:         cfg.Ops.Remote,
		share:          cfg.Ops.Share,
		fresh:          cfg.Ops.Fresh,
		compact:        cfg.Ops.Compact,
		handoff:        cfg.Ops.Handoff,
		route:          cfg.Ops.Route,
		modelCatalog:   cfg.Ops.ModelCatalog,
		modelHosts:     cfg.Ops.ModelHosts,
		modelProviders: cfg.Ops.ModelProviders,
		pinModel:       cfg.Ops.PinModel,
		effortOp:       cfg.Ops.Effort,
		statusOp:       cfg.Ops.Status,
		updateOp:       cfg.Ops.Update,
		mcp:            cfg.Ops.MCP,
		workflow:       cfg.Ops.Workflow,
		connectOps:     cfg.Ops.Connect,
		title:          cfg.Resume.Title,
		page:           cfg.Page,
		paged:          cfg.Paged,
		messages:       welcomeMessages(cfg.Agent),
		input:          ti,
		viewport:       vp,
		// Nothing is known before the gateway's first context event. The old
		// 200_000 default was the compaction ceiling (ctxmgmt.CompactionCeiling)
		// displayed as if it were the model's window — /context said 200.0k
		// before any turn ran. Zero means "no report yet" everywhere it renders.
		contextTokens:     0,
		contextLimit:      0,
		contextPercent:    0.0,
		activeTools:       make(map[string]time.Time),
		activeSubagents:   make(map[string]string),
		slashCommands:     slashCommands,
		showAutocomplete:  false,
		autocompleteIndex: 0,
		filteredCommands:  []string{},
	}
	if len(cfg.Resume.History) > 0 {
		// After the welcome, not before it: the transcript reads oldest-first
		// from the top of the session, and the welcome is the top of the session.
		m.messages = append(m.messages, cfg.Resume.History...)
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
	}
	return m
}

// welcomeHint is the first line of a new session.
func welcomeHint() string {
	return "Ask for a change and it reads what it needs, patches, builds and runs the tests before it answers. **@** mentions a file or folder · **/model** chooses which model answers · **/remote** opens this on your phone · **/help** for the rest."
}

// ONE LINE, NOT TWO (Greg, 2026-09-27: "less is more"). The second message
// repeated the mode badge that is already in the header and printed the channel
// UUID, which identifies the session to the gateway and tells the reader
// nothing.
func welcomeMessages(agent string) []Message {
	return []Message{
		{
			Role: "system",
			// The first line a person reads names the coding loop.
			Content:   welcomeHint(),
			Timestamp: time.Now(),
		},
	}
}

// Status is the footer line, in parts — so each can be coloured for what it
// means rather than the whole line sharing one tone.
type Status struct {
	Effort     string // reasoning effort the person chose (Shift+Tab); "" = auto
	EffortUsed string // what the last turn ran at
	EffortFrom string // "chosen", "decision model" or "default"
	Branch     string // git branch of the project, "" outside a repo
	Model      string // the model answering right now
	Provider   string // which provider serves it: openrouter, anthropic, the company gateway … (2026-10-02)

	// AgentModels is which model answers for which agent;
	// the screen shows its own agent's.
	AgentModels map[string]string
	// Route is which rung of the agent's ladder this conversation runs on
	// and why (legible routing, 2026-09-26): "rung 1/4 · first rung · held
	// for this session", "rung 3/4 · pinned by you". Rungs is the ladder's
	// length; 0 means nothing known.
	Rung   int
	Rungs  int
	Reason string
	Pinned bool
	// Update, when set, says a newer build is published ("memdoor v1.305 is
	// out — /update installs it"); the footer shows it until it is done.
	Update string
	// Gate, when set, is why a turn cannot run now ("no model configured —
	// export a provider key …"). Enter shows it instead of sending; the
	// footer refreshes every few seconds until it clears.
	Gate string
}

// line renders the status with a colour per part: where you are, what is
// thinking, and what it costs are three different questions.
func (s Status) line() string {
	var parts []string
	if s.Branch != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colRead)).Render("⎇ "+s.Branch))
	}
	// WHERE THE WORK IS, WHAT THE BRAIN IS DOING, AND WHICH MODEL ANSWERS.
	//
	// The hourly rate and the balance are gone from the window (Greg,
	// 2026-09-05): what it costs is the operator's business. The MODEL came
	// back on 2026-09-26: the product routes each agent to a model and will
	// move between them, so the person sees which one answers — never its
	// upstream hosts.
	//
	// One line for both front ends — the app and the terminal are one
	// product, so there is no mode to branch on here (ADR-0009).
	if s.Model != "" {
		// PROVIDER · MODEL: where the turn goes, always on screen — the check
		// Claude Code's docs make a developer run by hand (/status, "Anthropic
		// base URL"), here in the footer (Greg, 2026-10-02).
		model := s.Model
		if s.Provider != "" {
			model = s.Provider + " · " + model
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Render(model))
	}
	// THE RUNG AND ITS REASON, next to the model: routing is never an
	// opaque auto mode. A one-rung ladder has nothing to say.
	switch {
	case s.Pinned && s.Rung == 0:
		// A model pinned by id, off the ladder: no rung to name.
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colRead)).Render(s.Reason))
	case s.Rungs > 1 && s.Rung > 0:
		rung := fmt.Sprintf("rung %d/%d", s.Rung, s.Rungs)
		if s.Reason != "" {
			rung += " · " + s.Reason
		}
		colour := colDim
		if s.Pinned {
			colour = colRead
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(rung))
	}
	// THE EFFORT, chosen with Shift+Tab: auto lets the decision model (or
	// high) decide per request; a chosen one holds until changed.
	effort, colour := "effort "+effortLabel(s), colDim
	if s.Effort != "" {
		colour = colRead
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(effort+" (shift+tab)"))
	if s.Update != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colWarn)).Render(s.Update))
	}
	if len(parts) == 0 {
		return ""
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(colRule)).Render("  ·  ")
	return strings.Join(parts, sep)
}

// statusTickMsg refreshes the footer status line.
type statusTickMsg struct{}

func (m Model) tickStatus() tea.Cmd {
	return tick(30*time.Second, func(time.Time) tea.Msg { return statusTickMsg{} })
}

// statusTickSoon is the footer's cadence while a gate is up: a brain
// that is starting is watched, not waited for.
func statusTickSoon() tea.Cmd {
	return tick(5*time.Second, func(time.Time) tea.Msg { return statusTickMsg{} })
}

// scopeLabel is the header's second field: the directory a coding agent is
// working in, which is also what `memdoor resume` lists a conversation by, or
// the workspace when the directory is unknown.
func (m Model) scopeLabel() string {
	if d := m.dirLabel(); d != "" {
		return d
	}
	if m.workspace != "" {
		return m.workspace
	}
	return "—"
}

// dirLabel is the base name of the directory this conversation works in.
// /dir re-roots a session with os.Chdir, so the process directory is always
// the truth about where the next turn will run.
func (m Model) dirLabel() string {
	d, err := os.Getwd()
	if err != nil || d == "" {
		return ""
	}
	return filepath.Base(d)
}

// noteResultMsg carries the outcome of the async ops (/model, /usage)
// that just report a message.
type noteResultMsg struct {
	summary string
	err     error
}

// send delivers the user's turn to the agent. When sendFunc is wired (the
// platform message path — POST /api/messages "@agent …"), it posts there so the
// run loads the agent's seeded tools + grounding + A2A. Otherwise it falls back
// to the raw websocket chat (tests / legacy). The websocket stays connected
// either way to receive the run's streamed tool/assistant events.
// dispatch posts the turn to `agent` OFF the event loop as a tea.Cmd, so a
// slow/unreachable gateway never freezes the UI. The websocket streams the
// run's events back.
// Returns nil only in the legacy ws-fallback path (no injected poster).

// routeByMention lets a turn that opens with "@name " run as THAT agent
// instead of the session's coding agent. The poster prepends "@"+agent to
// every turn, so a typed "@planner outline this" arrived as "@coder @planner
// outline this" and the server routed to the FIRST mention, the coder. Only a
// LEADING mention routes; "ping me @ 5pm" is ordinary text.
// The name is not validated here: an unknown agent is the server's error to
// report, not a reason for the TUI to silently ignore what the user typed.
func routeByMention(defaultAgent, text string) (agent, rest string) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "@") || len(t) < 2 {
		return defaultAgent, text
	}
	i := strings.IndexFunc(t[1:], func(r rune) bool {
		return !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if i <= 0 {
		return defaultAgent, text
	}
	return strings.ToLower(t[1 : i+1]), strings.TrimSpace(t[i+1:])
}

func (m *Model) dispatch(agent, text string) tea.Cmd {
	if m.poster != nil {
		poster, workspace, mode := m.poster, m.workspace, m.permissionModeString()
		return func() tea.Msg { return sendResultMsg{err: poster(agent, workspace, text, mode)} }
	}
	if m.connected {
		// Production path: run the single coding agent (server-side). Always-auto:
		// permissionModeString is the one source of the turn's permission mode.
		_ = m.ws.SendMessage(text, m.permissionModeString())
	}
	return nil
}

// lastAssistantContent returns the most recent assistant message's text (the
// plan, when the last turn produced one), or "" if none.
func (m Model) lastAssistantContent() string {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			return m.messages[i].Content
		}
	}
	return ""
}

// runPlan is the /go entry point (also /run, /approve): hand the plan the last
// turn produced back to the coding agent to implement and verify. Returns nil
// (and notes) if there's no plan to run.
func (m *Model) runPlan() tea.Cmd {
	plan := m.lastAssistantContent()
	if strings.TrimSpace(plan) == "" {
		m.note("No plan to run yet.")
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return nil
	}
	m.interrupted = false
	m.messages = append(m.messages, Message{Role: "system", Content: "Plan accepted → implementing", Timestamp: time.Now()})
	cmd := m.dispatch("coder", "Implement this plan exactly. Write the file(s), build and run to verify, and fix until it works:\n\n"+plan)
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	m.viewport.SetContent(m.renderMessages())
	m.gotoBottom()
	return tea.Batch(cmd, m.tickThinking())
}

// answerQuestion sends the user's choice for the pending interactive question back
// to the (blocked) agent and clears the picker.
func (m *Model) answerQuestion(selected string) tea.Cmd {
	q := m.pendingQuestion
	m.pendingQuestion = nil
	if q == nil {
		return nil
	}
	if m.ws != nil {
		_ = m.ws.SendAnswer(q.id, selected)
	}
	m.messages = append(m.messages, Message{Role: "system", Content: "answered: " + selected, Timestamp: time.Now()})
	m.viewport.SetContent(m.renderMessages())
	m.gotoBottom()
	return nil
}

// sendResultMsg reports a failed post (the run's own events stream separately).
type sendResultMsg struct{ err error }

// flushQueue sends turns the user typed while the agent was busy, BATCHED into a
// single turn (Claude-CLI style), and re-enters the thinking state. Called when
// a turn completes. Returns nil when nothing is queued.
// tick is how this package waits. Production ticks are the real delays; a
// test sets this to fire at once, so a suite that drives dozens of turns
// does not sleep through every spinner and flush timer (the TUI tests spent
// 18s doing that).
var tick = tea.Tick

// cronAnswerMsg is one run of a check the agent scheduled with the cron tool,
// answering into the conversation that asked for it (gateway: postCronAnswer).
type cronAnswerMsg struct {
	jobID string
	text  string
}

// queueFlushMsg asks for queued input to be sent if no turn is running.
type queueFlushMsg struct{}

// flushQueueSoon follows a run-complete: a normal turn's reply arrives just
// after it and flushes the queue itself; a turn that ended without a reply
// has nothing else to send queued input, so this does, a moment later.
func flushQueueSoon() tea.Cmd {
	return tick(1500*time.Millisecond, func(time.Time) tea.Msg { return queueFlushMsg{} })
}

func (m *Model) flushQueue() tea.Cmd {
	if len(m.queued) == 0 {
		return nil
	}
	batch := strings.Join(m.queued, "\n")
	m.queued = nil
	m.interrupted = false
	// Queued turns must go to the SAME agent as a direct send (m.codeAgent) — one
	// agent per session.
	cmd := m.dispatch(m.codeAgent, batch)
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	return tea.Batch(cmd, m.tickThinking())
}

// SetUsageOp wires /usage — what the workspace has used this month. A
// subscription may show usage; it does not show what the usage cost.
// sender is where the websocket delivers: the program, or, for a page inside
// an App, a pageSender that tags each message with the page (pages.go).
func (m Model) sender(p *tea.Program) msgSender {
	if p == nil {
		return nil // a typed nil in the interface would not read as nil
	}
	if m.paged {
		return pageSender{program: p, page: m.page}
	}
	return p
}

// interruptedNote is the line Esc leaves: it names the way out of a session
// that has gone wrong, not only the fact that it stopped.
const interruptedNote = "Interrupted — `/fresh <request>` starts over in a clean session"

// clearedMsg / freshMsg carry the gateway's answer to /clear and /fresh.
type clearedMsg struct{ err error }
type freshMsg struct {
	task string
	err  error
	// handoff: the summary a /handoff started the conversation from.
	handoff string
}

// stopLocally is the screen's half of an interrupt: the turn stops drawing
// and queued prompts are dropped. The gateway's half is the ws cancel (Esc)
// or the fresh endpoint's own cancel (/fresh).
func (m *Model) stopLocally() {
	m.interrupted = true
	m.isThinking = false
	m.turnRunning = false
	m.activeTools = nil
	m.closeOrphanFrames("interrupted")
	m.activeSubagents = nil
	m.subagentWork = nil
	m.queued = nil
}

// effortLabel is the effort in use as the footer says it: the chosen level;
// on auto the level the last turn ran at ("high · auto", "medium · auto
// (Jev)"); "auto" before the first turn.
func effortLabel(s Status) string {
	switch {
	case s.Effort != "":
		return s.Effort
	case s.EffortUsed == "":
		return "auto"
	case s.EffortFrom == "decision model":
		return s.EffortUsed + " · auto (Jev)"
	}
	return s.EffortUsed + " · auto"
}
