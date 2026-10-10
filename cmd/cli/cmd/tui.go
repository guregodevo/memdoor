package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/tools"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// enableTmuxPassthrough turns on per-pane allow-passthrough so the TUI's OSC 8
// hyperlinks (wrapped in tmux DCS passthrough) actually reach the outer terminal
// and become clickable. Best-effort and no-op outside tmux.
func enableTmuxPassthrough() {
	if os.Getenv("TMUX") == "" {
		return
	}
	_ = exec.Command("tmux", "set", "-p", "allow-passthrough", "on").Run()
}

// The agent the TUI converses with, and the channel its conversation lives in.
// The companion is the product's grounded-memory front door; it loads its
// seeded tool palette + forced grounding + A2A when reached via the message path.
const (
	tuiAgent   = "companion"
	tuiChannel = "general"
)

// tuiOptions is how the TUI is launched. A fresh conversation leaves
// resumeChannel empty; `memdoor resume` names the channel to rejoin, which is
// the whole difference between the two.
type tuiOptions struct {
	resumeChannel   string
	resumeChannelID string
	resumeTitle     string
	workspace       string
	worktreePath    string
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --worktree: the session runs in its own git worktree (worktree.go),
		// and the window says where the work is when it closes.
		if cmd.Flags().Changed("worktree") {
			name, _ := cmd.Flags().GetString("worktree")
			// `--worktree fix-login` (a space, not "=") arrives as an argument:
			// a flag whose value is optional does not take the next word. Both
			// spellings mean the same thing to the person typing them.
			if name == "auto" && len(args) == 1 {
				name = args[0]
			}
			wt, err := startWorktree(name)
			if err != nil {
				return err
			}
			fmt.Printf("Working in worktree %s (branch %s)\n", wt.Path, wt.Branch)
			runErr := runTUI(tuiOptions{worktreePath: wt.Path})
			fmt.Println(wt.finish())
			return runErr
		}
		if ch, _ := cmd.Flags().GetString("channel"); ch != "" {
			return runTUI(tuiOptions{resumeChannel: ch})
		}
		return runTUI(tuiOptions{})
	},
}

func runTUI(opts tuiOptions) error {
	{
		_ = os.Setenv("MEMDOOR_GATEWAY", gatewayAddr)
		wsAddr := strings.Replace(gatewayAddr, "http://", "ws://", 1)
		wsAddr = strings.Replace(wsAddr, "https://", "wss://", 1)
		wsAddr += "/ws"

		// The /agents slash command calls tools.AgentsList, which resolves
		// /api/agents via the tools package's gateway base URL — set it here so
		// the command works instead of panicking ("gateway base URL not set").
		if u, perr := url.Parse(gatewayAddr); perr == nil && u.Hostname() != "" {
			if port, aerr := strconv.Atoi(u.Port()); aerr == nil && port != 0 {
				tools.SetGatewayBaseURL(u.Hostname(), port)
			}
		}

		// Authenticate like the CLI: NewClient loads the saved credentials
		// (~/.memdoor/credentials.json) and carries the Bearer token +
		// X-Forwarded-Workspace on every request.
		// A machine that never ran Memdoor is set up here, without a
		// question; a key the shell holds reaches a gateway that started
		// without it (tui_firstrun.go).
		if err := firstRunReady(); err != nil {
			return err
		}
		c := NewClient()
		handShellKeyToGateway(c)

		// The conversation is scoped to the workspace, and the session key the
		// gateway builds carries it.
		ws := workspaceSlug
		if opts.workspace != "" {
			ws = opts.workspace
		}
		wsID := strings.TrimSpace(ws)
		if wsID == "" {
			return fmt.Errorf("no workspace resolved — `memdoor workspace use <slug>` pins this directory")
		}

		// Resolve the channel UUID. The /ws connection subscribes by workspace +
		// channel; the gateway builds the session key (no client-side
		// session-key construction — single source of truth in the domain model).
		// A fresh channel per launch keeps a small model clean (accumulated
		// history makes it mimic prior replies); it falls back to the shared
		// channel if creation isn't permitted. Resuming deliberately opts out:
		// the point is the history.
		channelName, channelID := tuiChannel, ""
		if opts.worktreePath != "" {
			// --worktree reopens the piece of work, so it continues the
			// conversation the worktree already had: the index records each
			// session's directory, and the process chdir'd into the worktree
			// before the turns were recorded. Only a session with turns
			// qualifies — a record named but never sent to has no history.
			for _, r := range sessionsFor(opts.worktreePath) {
				if r.Turns > 0 {
					opts.resumeChannel, opts.resumeChannelID, opts.resumeTitle =
						r.ChannelName, r.ChannelID, r.Title
					break
				}
			}
		}
		switch {
		case opts.resumeChannelID != "":
			// Resuming: the id was recorded when the conversation started.
			channelName, channelID = opts.resumeChannel, opts.resumeChannelID
		default:
			fresh := "tui-" + time.Now().Format("0102-150405")
			if id, cerr := c.createChannel(fresh); cerr == nil && id != "" {
				// Carry the id the create returned. Looking it up again by name
				// does not work: /api/channels pages ORDER BY name, so a channel
				// named for today sorts past the end of the first page and reads
				// as missing.
				channelName, channelID = fresh, id
			}
		}
		if channelID == "" {
			var err error
			if channelID, err = c.resolveChannelID(channelName); err != nil {
				// The session, not the channel, is the usual reason both the
				// create and the lookup came back empty.
				if p := sessionProblem(); p != nil {
					return p
				}
				return fmt.Errorf("TUI needs the %q channel and a logged-in user "+
					"(try 'memdoor auth login-direct'): %w", channelName, err)
			}
		}

		// Post each turn through the platform message path (scoped to the
		// workspace) so the run loads the agent's seeded tools and A2A. The
		// websocket is kept only for receiving the run's streamed events.
		// Claude-CLI semantics: the coder works on the project the TUI was
		// LAUNCHED from — capture cwd once and send it with every turn.
		launchDir, _ := os.Getwd()
		// The session's directory is READ PER TURN, not captured at launch:
		// /dir re-roots a running session (os.Chdir) so a user who launched in
		// the wrong folder points the agent elsewhere instead of relaunching.
		curDir := func() string {
			if d, err := os.Getwd(); err == nil {
				return d
			}
			return launchDir
		}
		// One page per agent (ui/pages.go): each page has its own channel
		// and its own poster bound to it. The first page is the coder's.
		posterFor := func(channelID, channelName string) func(agent, w, text, mode string) error {
			return func(agent, w, text, mode string) error {
				// Note the conversation before sending: the first turn names it, and
				// every turn keeps it at the top of `memdoor resume`.
				recordSession(sessionRecord{
					ChannelID:   channelID,
					ChannelName: channelName,
					Title:       firstLine(text),
					Workspace:   wsID,
					Dir:         curDir(),
				})
				return c.PostExpectOK("/api/messages", map[string]interface{}{
					"channel_id":      channelID,
					"text":            "@" + agent + " " + text,
					"workspace":       wsID,
					"permission_mode": mode, // Claude-style plan/acceptEdits/default (empty = default)
					"workdir":         curDir(),
				})
			}
		}
		plan := windowPlan() // once, for the input's hint (ui/suggest.go)
		decisionsOff := !gatewayDecisionsOn(c)
		enableTmuxPassthrough() // make OSC 8 links clickable inside tmux
		// Everything a page needs, at construction (ui.PageConfig): the ops
		// close over the authed client; the route and status ops over the
		// page's own channel and agent.
		// The phone's TUI is one conversation: it opens no second page.
		newPhonePage := func(string, int) (ui.Model, error) {
			return ui.Model{}, errors.New("a second page is for the terminal on your computer")
		}
		pageConfig := func(agent, chID, chName string, page int) ui.PageConfig {
			// Remote control (tui_remote.go): a phone's turn is posted exactly as
			// a typed one, through this page's poster, in the TUI's always-auto
			// mode; a conversation that already has a link picks its relay back up.
			post := posterFor(chID, chName)
			remote := remoteSession{
				Gateway: gatewayAddr, Token: c.token, Workspace: wsID, ChannelID: chID,
				Account: brokerToken(),
				Post:    func(text string) error { return post(agent, wsID, text, "acceptEdits") },
				History: func() []ui.Message { return fetchTranscript(c, chID, remoteHistoryLimit) },
			}
			cfg := ui.PageConfig{
				GatewayAddr: wsAddr, Workspace: wsID, ChannelID: chID, Token: c.token, Plan: plan, DecisionsOff: decisionsOff,
				Agent: agent, Poster: posterFor(chID, chName), Page: page, Paged: true,
				Ops: ui.Ops{
					// /model: list + pin the model this conversation runs on.
					Usage:          tuiUsage,
					Route:          tuiRoute(wsID, chID, agent),
					ModelCatalog:   tuiModelCatalog,
					ModelHosts:     tuiModelHosts,
					ModelProviders: tuiModelProviders,
					PinModel:       tuiPinModel(wsID, chID, agent),
					Effort:         tuiEffort(wsID, chID, agent),
					MCP:            tuiMCPOps(curDir),
					Workflow:       tuiWorkflowOps(curDir, wsID, chID),
					Connect:        ui.ConnectOps{Connect: tuiConnect, KeySource: providerKeySource, LoginStart: tuiConnectLoginStart, LoginWait: chatgptLoginWait, LoginPaste: chatgptLoginPaste, LoginCancel: chatgptLoginCancel},
					Status: func() ui.Status {
						st := tuiPageStatus(curDir(), wsID, chID, agent)
						st.Update = tuiUpdateNotice()
						return st
					},
					Update: tuiUpdate,
					// /fresh and /clear: the gateway wipes the agent's memory of
					// this conversation (gateway/session_fresh.go).
					Fresh: func(agent string) error {
						return c.PostExpectOK("/api/sessions/fresh", map[string]interface{}{
							"workspace": wsID, "channel_id": chID, "agent": agent,
						})
					},
					Handoff: func(agent string) (string, error) {
						var r struct {
							Handoff string `json:"handoff"`
						}
						if err := c.PostJSON("/api/sessions/handoff", map[string]interface{}{
							"workspace": wsID, "channel_id": chID, "agent": agent,
						}, &r); err != nil {
							return "", err
						}
						return r.Handoff, nil
					},
					Compact: func(agent, focus string) (string, error) {
						var r compactReply
						if err := c.PostJSON("/api/sessions/compact", map[string]interface{}{
							"workspace": wsID, "channel_id": chID, "agent": agent, "focus": focus,
						}, &r); err != nil {
							return "", err
						}
						return r.line(focus), nil
					},
				},
			}
			// The page's terminal view is this same TUI on this conversation,
			// opened the way a resumed one is: a single page, its last
			// exchanges in view (tui_remote_tty.go).
			phone := cfg
			phone.Page = 0
			remote.TTY = func() tea.Model {
				view := phone
				view.Resume = ui.Resume{History: fetchTranscript(c, chID, 30)}
				view.Ops.Remote = tuiRemote(remote)
				return ui.NewApp(ui.NewPage(view), newPhonePage)
			}
			cfg.Ops.Remote = tuiRemote(remote)
			cfg.Ops.Share = tuiShare(shareSession{
				Account: brokerToken(), ChannelID: chID,
				Transcript: func() []ui.Message { return fetchTranscript(c, chID, shareTranscriptLimit) },
			})
			resumeRemote(remote)
			return cfg
		}
		newPage := func(agent string, page int) (ui.Model, error) {
			fresh := "tui-" + agent + "-" + time.Now().Format("0102-150405")
			id, cerr := c.createChannel(fresh)
			if cerr != nil || id == "" {
				return ui.Model{}, fmt.Errorf("no channel for the %s page: %v", agent, cerr)
			}
			return ui.NewPage(pageConfig(agent, id, fresh, page)), nil
		}

		first := pageConfig(ui.FirstAgent(), channelID, channelName, 0)
		if opts.resumeChannelID != "" {
			// A resumed conversation should look like the one you left: its
			// title in the header and the last exchanges in view. The agent's
			// own memory lives server-side; this is the human's half of it.
			first.Resume = ui.Resume{Title: opts.resumeTitle, History: fetchTranscript(c, channelID, 30)}
		}
		model := ui.NewPage(first)
		app := ui.NewApp(model, newPage)
		// Neither the altscreen nor mouse capture, and the two go together.
		// Settled messages are printed to real terminal scrollback
		// (ui/scrollback.go), which tea.Println can only do outside the
		// altscreen; and capturing the mouse would swallow the wheel, so the
		// scrollback we just gained could not be scrolled. Everything that used
		// to need a click has a key or a slash command: the /model picker takes
		// arrows and Enter.
		// Frames are written through the synchronized-output bracket so the
		// terminal applies each repaint atomically — see ui/sync_writer.go.
		// WithReportFocus: the window learns when it loses focus, so the end of a
		// turn rings only when nobody is looking (ui/window_title.go).
		p := tea.NewProgram(app, tea.WithOutput(ui.NewSyncWriter(os.Stdout)), tea.WithReportFocus())
		// Goroutine: bubbletea's msgs channel is unbuffered with no reader until
		// Run() starts, so a direct p.Send here deadlocks before the first render.
		go p.Send(ui.SetProgramMsg{Program: p})

		final, err := p.Run()
		ui.CmuxClear() // no "Running" left beside the workspace after quitting mid-turn
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		if a, ok := final.(ui.App); ok && a.RestartAfterUpdate() {
			// The new binary is in place: this process becomes it, back
			// in the same conversation (update.go).
			if err := reexecIntoConversation(); err != nil {
				fmt.Println("Updated. Start again with: memdoor resume --last")
			}
			return nil
		}
		// The way back in, printed where the eye lands after the window
		// closes (Greg, 2026-10-03: "when the user exits we should see the
		// resume command").
		fmt.Println(resumeLine(channelID))
		return nil
	}
}

// fetchTranscript loads the tail of a channel's conversation so a resumed
// session opens where it left off rather than on a blank screen. Best effort:
// a conversation whose history cannot be read still resumes, just empty.
func fetchTranscript(c *Client, channelID string, limit int) []ui.Message {
	var raw json.RawMessage
	if err := c.GetJSON(fmt.Sprintf("/api/channels/%s/messages?limit=%d", channelID, limit), &raw); err != nil {
		return nil
	}
	return parseTranscript(raw)
}

// parseTranscript turns a channel's message payload into transcript lines.
// Split out from the fetch so the SHAPE can be tested: a mismatch between what
// the gateway sends and what this reads is silent — encoding/json leaves
// unmatched fields zero, and the transcript just comes back empty.
func parseTranscript(raw []byte) []ui.Message {
	var result struct {
		Messages []struct {
			AuthorID string `json:"author_id"`
			Content  struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	var out []ui.Message
	for _, m := range result.Messages {
		text := strings.TrimSpace(m.Content.Text)
		if text == "" {
			continue
		}
		role := "assistant"
		if strings.HasPrefix(m.AuthorID, "human:") {
			role = "user"
			// Turns are posted as "@agent <text>" — show what was typed.
			if strings.HasPrefix(text, "@") {
				if i := strings.Index(text, " "); i > 0 {
					text = text[i+1:]
				}
			}
		}
		out = append(out, ui.Message{Role: role, Content: text, Timestamp: time.Now()})
	}
	return out
}

// firstLine is the conversation's title: the first line of the first thing
// asked, trimmed to something that fits a list.
func firstLine(text string) string {
	t := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if len([]rune(t)) > 72 {
		t = string([]rune(t)[:71]) + "…"
	}
	return t
}

func init() {
	tuiCmd.Flags().String("channel", "", "Rejoin an existing channel instead of starting a fresh conversation")
	tuiCmd.Flags().String("worktree", "", "Work in a git worktree of this repository (optionally named); your checkout is left alone. Untracked paths it needs to build (listed in .worktreeinclude) are linked in")
	tuiCmd.Flags().Lookup("worktree").NoOptDefVal = "auto"
	rootCmd.AddCommand(tuiCmd)
	// Suppress errors + usage when the TUI exits cleanly.
	tuiCmd.SilenceErrors = true
	tuiCmd.SilenceUsage = true
}

// windowPlan is the account's plan for the input's hint: "free" on a machine
// that is not signed in — the person an upgrade line is for — the service's
// answer when it is, and "" when the service does not answer within a moment.
// Read once when the window opens, never per tick: a hint is not worth a
// network call every few seconds.
func windowPlan() string {
	if brokerToken() == "" {
		return "free"
	}
	ch := make(chan string, 1)
	go func() {
		st := currentAccountStatus()
		p := strings.ToLower(strings.TrimSpace(st.Plan))
		if p == "" && st.SignedIn && st.Note == "" {
			p = "free"
		}
		ch <- p
	}()
	select {
	case p := <-ch:
		return p
	case <-time.After(3 * time.Second):
		return ""
	}
}
