package cmd

import (
	"errors"
	"strings"

	"memdoor/cmd/tui/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// tuiWiring is everything a page needs at construction, closed over the
// authed client: the ops, the poster, the remote relay and the phone's own
// view of the conversation. The window (runTUI) and the phone door with no
// window (`memdoor remote`, remote.go) build the same thing, so a phone
// reaching a conversation a script started sees exactly the TUI it would at
// a terminal (Greg, 2026-10-12: "there should be a remote command to return
// in stdout the link", "from a resume id").
type tuiWiring struct {
	c            *Client
	wsID         string
	wsAddr       string
	plan         string
	decisionsOff bool
	curDir       func() string
	posterFor    func(channelID, channelName string) func(agent, w, text, mode string) error
	fs           ui.FS // the project's reads when it is on the gateway's machine; nil = this machine
}

func newTUIWiring(c *Client, wsID, wsAddr string, curDir func() string,
	posterFor func(channelID, channelName string) func(agent, w, text, mode string) error) *tuiWiring {
	return &tuiWiring{
		c: c, wsID: wsID, wsAddr: wsAddr,
		plan:         windowPlan(), // once, for the input's hint (ui/suggest.go)
		decisionsOff: !gatewayDecisionsOn(c),
		curDir:       curDir, posterFor: posterFor,
	}
}

// The phone's TUI is one conversation: it opens no second page.
func newPhonePage(string, int) (ui.Model, error) {
	return ui.Model{}, errors.New("a second page is for the terminal on your computer")
}

// remoteFor is a conversation's relay session: a phone's turn is posted
// exactly as a typed one, through the page's poster, in the TUI's always-auto
// mode; its terminal view is this same TUI on this conversation, opened the
// way a resumed one is (tui_remote_tty.go).
func (w *tuiWiring) remoteFor(agent, chID, chName string) remoteSession {
	post := w.posterFor(chID, chName)
	c, wsID := w.c, w.wsID
	remote := remoteSession{
		Gateway: gatewayAddr, Token: c.token, Workspace: wsID, ChannelID: chID,
		Account: brokerToken(),
		Post:    func(text string) error { return post(agent, wsID, text, "acceptEdits") },
		History: func() []ui.Message { return fetchTranscript(c, chID, remoteHistoryLimit) },
	}
	remote.TTY = func() tea.Model {
		view := w.baseConfig(agent, chID, chName, 0)
		view.Resume = ui.Resume{History: fetchTranscript(c, chID, 30)}
		view.Ops.Remote = tuiRemote(remote)
		return ui.NewApp(ui.NewPage(view), newPhonePage)
	}
	return remote
}

// pageConfig is a window's page: the base config plus /remote and /share; a
// conversation that already has a link picks its relay back up.
func (w *tuiWiring) pageConfig(agent, chID, chName string, page int) ui.PageConfig {
	remote := w.remoteFor(agent, chID, chName)
	cfg := w.baseConfig(agent, chID, chName, page)
	cfg.Ops.Remote = tuiRemote(remote)
	c := w.c
	cfg.Ops.Share = tuiShare(shareSession{
		Account: brokerToken(), ChannelID: chID,
		Transcript: func() []ui.Message { return fetchTranscript(c, chID, shareTranscriptLimit) },
	})
	resumeRemote(remote)
	return cfg
}

// baseConfig is the page without its remote and share ops.
func (w *tuiWiring) baseConfig(agent, chID, chName string, page int) ui.PageConfig {
	c, wsID, curDir := w.c, w.wsID, w.curDir
	return ui.PageConfig{
		GatewayAddr: w.wsAddr, Workspace: wsID, ChannelID: chID, Token: c.token, Plan: w.plan, DecisionsOff: w.decisionsOff,
		Agent: agent, Poster: w.posterFor(chID, chName), Page: page, Paged: true, FS: w.fs,
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
}

// tuiPoster posts a turn through the platform message path, scoped to the
// workspace, so the run loads the agent's seeded tools and A2A; the
// websocket is kept only for the run's streamed events. The coder works on
// the directory the window was launched from (curDir, read per turn: /dir
// re-roots a session), sent with every turn. Each poster is bound to one
// page's channel.
func tuiPoster(c *Client, wsID string, curDir func() string) func(channelID, channelName string) func(agent, w, text, mode string) error {
	return func(channelID, channelName string) func(agent, w, text, mode string) error {
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
}

// wsURLOf is the gateway's websocket: the http(s) address as ws(s), /ws.
func wsURLOf(gateway string) string {
	ws := strings.Replace(gateway, "http://", "ws://", 1)
	ws = strings.Replace(ws, "https://", "wss://", 1)
	return ws + "/ws"
}
