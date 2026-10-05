package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// /mcp — the person's MCP servers, in one panel (Greg, 2026-10-01: "a nice
// ui experience for adding mcp and auth"). One overlay, four screens:
//
//   list     every server: ● connected (its tools), ◐ needs sign-in, ✗ failed
//            (the reason), ○ off, ? waiting for this project's trust
//   add      one box: paste a URL, a command line or a .mcp.json snippet;
//            the transport follows from it, nobody is asked
//   where    save for this project (.mcp.json) or for every project
//   sign-in  the browser opens; the link is shown and copied; a pasted
//            address finishes it from another machine; esc cancels
//
// The typed forms (/mcp add …, /mcp login x, /mcp test x, /mcp on|off x,
// /mcp remove x, /mcp trust) do the same without the panel.

// MCPServer is a server as the panel shows it (cmd/cli fills it from
// /api/mcp).
type MCPServer struct {
	Name      string
	Scope     string // "project" or "user"
	Transport string // "stdio" or "http"
	Target    string
	State     string
	Error     string
	Tools     []string
	Server    string
	// Saved is an add's answer: false means it was not kept, so the box
	// opens again with the reason instead of the panel showing a dead row.
	Saved bool
}

// MCPPanel is the panel's data: the servers, and whether this project's
// own servers may start.
type MCPPanel struct {
	Servers []MCPServer
	Trusted bool
}

// MCPOps are the panel's calls (injected by cmd/cli, which runs them for
// the directory the TUI was launched from).
type MCPOps struct {
	List        func() (MCPPanel, error)
	Add         func(input string, everywhere bool) ([]MCPServer, error)
	Action      func(action, name string) (MCPServer, error) // test, on, off, remove, logout, trust
	LoginStart  func(name string) (id, authURL string, opened, copied bool, err error)
	LoginWait   func(id string) (done bool, st MCPServer, err error)
	LoginPaste  func(id, value string) error
	LoginCancel func(id string)
	Prompts     func() ([]MCPPrompt, error)
	Search      func(q string) ([]MCPRegistryServer, error) // the official MCP registry
	GetPrompt   func(server, name string, args map[string]string) (string, error)
}

// MCPRegistryServer is a server found in the official MCP registry.
type MCPRegistryServer struct {
	Name, Suggested, Description, Version, Via string
	Needs                                      []string
	Snippet                                    string // what /mcp add takes
}

// MCPPrompt is a server's prompt, offered as /server:prompt.
type MCPPrompt struct {
	Server      string
	Name        string
	Description string
	Args        []MCPPromptArg
}

// MCPPromptArg is one argument a prompt takes.
type MCPPromptArg struct {
	Name        string
	Description string
	Required    bool
}

// Command is how the prompt is typed.
func (p MCPPrompt) Command() string { return "/" + p.Server + ":" + p.Name }

// usage is the prompt's command with its arguments.
func (p MCPPrompt) usage() string {
	u := p.Command()
	for _, a := range p.Args {
		if a.Required {
			u += " <" + a.Name + ">"
		} else {
			u += " [" + a.Name + "]"
		}
	}
	return u
}

type mcpPromptsMsg struct{ prompts []MCPPrompt }

type mcpPromptTextMsg struct {
	cmd, text string
	err       error
}

// mcpLoadPrompts fetches the servers' prompts so they are in the / list.
func (m *Model) mcpLoadPrompts() tea.Cmd {
	if m.mcp.Prompts == nil {
		return nil
	}
	op := m.mcp.Prompts
	return func() tea.Msg {
		p, err := op()
		if err != nil {
			return nil
		}
		return mcpPromptsMsg{prompts: p}
	}
}

// isMCPPrompt reports whether a slash command is a server's prompt.
func (m *Model) isMCPPrompt(cmd string) bool { _, ok := m.mcpPrompts[cmd]; return ok }

// knownMCPServer answers whether the panel has seen a server of this name,
// so a prompt that no longer resolves can say which server is missing.
func (m *Model) knownMCPServer(name string) bool {
	if m.mcpSeen[name] {
		return true
	}
	for cmd := range m.mcpPrompts {
		if server, _, ok := strings.Cut(strings.TrimPrefix(cmd, "/"), ":"); ok && server == name {
			return true
		}
	}
	return false
}

// startTurn sends task as the person's message: shown, routed to its agent,
// the turn started (a /fresh request, a filled MCP prompt).
func (m *Model) startTurn(task string) tea.Cmd {
	m.messages = append(m.messages, Message{Role: "user", Content: task, Timestamp: time.Now()})
	m.viewport.SetContent(m.renderMessages())
	m.gotoBottom()
	m.interrupted = false // a new turn — accept its events again
	agent, body := routeByMention(m.codeAgent, task)
	cmd := m.dispatch(agent, body)
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	return tea.Batch(cmd, m.tickThinking())
}

// mcpPromptFor is the MCP prompt a slash command names, if it is one.
func (m *Model) mcpPromptFor(cmd string) (MCPPrompt, bool) {
	p, ok := m.mcpPrompts[cmd]
	return p, ok
}

// runMCPPrompt fills a prompt and sends it as the turn: arguments in order,
// or name=value.
func (m *Model) runMCPPrompt(p MCPPrompt, words []string) tea.Cmd {
	args := map[string]string{}
	var positional []string
	for _, w := range words {
		if k, v, ok := strings.Cut(w, "="); ok && p.hasArg(k) {
			args[k] = v
			continue
		}
		positional = append(positional, w)
	}
	for _, a := range p.Args {
		if _, set := args[a.Name]; set || len(positional) == 0 {
			continue
		}
		args[a.Name], positional = positional[0], positional[1:]
	}
	if len(positional) > 0 && len(p.Args) > 0 {
		last := p.Args[len(p.Args)-1].Name
		args[last] = strings.TrimSpace(args[last] + " " + strings.Join(positional, " "))
	}
	for _, a := range p.Args {
		if a.Required && args[a.Name] == "" {
			desc := ""
			if a.Description != "" {
				desc = " — " + a.Name + ": " + a.Description
			}
			m.note(fmt.Sprintf("**%s** needs %s%s", p.usage(), a.Name, desc))
			return nil
		}
	}
	m.note(fmt.Sprintf("Filling **%s** from %s …", p.Command(), p.Server))
	op, cmd := m.mcp.GetPrompt, p.Command()
	return func() tea.Msg {
		t, err := op(p.Server, p.Name, args)
		return mcpPromptTextMsg{cmd: cmd, text: t, err: err}
	}
}

func (p MCPPrompt) hasArg(name string) bool {
	for _, a := range p.Args {
		if a.Name == name {
			return true
		}
	}
	return false
}

const (
	mcpList = iota
	mcpAdd
	mcpWhere
	mcpSignIn
	mcpResults
)

type mcpPanelState struct {
	screen    int
	panel     MCPPanel
	index     int
	loading   string
	err       string
	input     string // the add box, or the pasted address while signing in
	pending   string // the add box's text, while asking where
	label     string // what is being added, as shown while asking where
	found     []MCPRegistryServer
	query     string
	searching bool   // the add box searches the registry instead
	where     int    // 0 this project, 1 every project
	confirm   string // a remove waiting for its second x
	signin    struct {
		id, url, name  string
		opened, copied bool
		started        time.Time
	}
}

type (
	mcpPanelMsg struct {
		panel MCPPanel
		err   error
		open  bool // open the panel on it (startup trust question)
	}
	mcpAddMsg struct {
		servers []MCPServer
		err     error
	}
	mcpActionMsg struct {
		action, name string
		st           MCPServer
		err          error
	}
	mcpLoginStartMsg struct {
		name, id, url  string
		opened, copied bool
		err            error
	}
	mcpLoginMsg struct {
		id   string
		done bool
		st   MCPServer
		err  error
	}
	mcpTickMsg   struct{}
	mcpSearchMsg struct {
		query string
		found []MCPRegistryServer
		err   error
	}
)

// mcpCommand runs /mcp and its typed forms.
func (m *Model) mcpCommand(args []string) tea.Cmd {
	if m.mcp.List == nil {
		m.note("MCP isn't available in this build.")
		return nil
	}
	if len(args) == 0 {
		return m.openMCPPanel(mcpList, "")
	}
	switch args[0] {
	case "add":
		if len(args) == 1 {
			return m.openMCPPanel(mcpAdd, "")
		}
		m.mcpPanel = &mcpPanelState{screen: mcpWhere, pending: strings.Join(args[1:], " ")}
		return nil
	case "login":
		if len(args) < 2 {
			m.note("Which server? **/mcp login <name>**")
			return nil
		}
		m.mcpPanel = &mcpPanelState{screen: mcpSignIn}
		return m.mcpStartSignIn(args[1])
	case "test", "on", "off", "remove", "logout":
		if len(args) < 2 {
			m.note(fmt.Sprintf("Which server? **/mcp %s <name>**", args[0]))
			return nil
		}
		return m.mcpRunAction(args[0], args[1])
	case "trust":
		return m.mcpRunAction("trust", "")
	case "search":
		if len(args) < 2 {
			m.note("Search the MCP registry: **/mcp search github**")
			return nil
		}
		return m.mcpSearch(strings.Join(args[1:], " "))
	}
	m.note("**/mcp** opens the panel · **/mcp add** <URL, command or .mcp.json snippet> · **/mcp login|logout|test|on|off|remove** <name> · **/mcp trust**")
	return nil
}

func (m *Model) mcpSearch(q string) tea.Cmd {
	if m.mcp.Search == nil {
		m.note("Registry search isn't available in this build.")
		return nil
	}
	if m.mcpPanel == nil {
		m.mcpPanel = &mcpPanelState{}
	}
	p := m.mcpPanel
	p.screen, p.query, p.index, p.err, p.loading = mcpResults, q, 0, "", "Searching the MCP registry for "+q+" …"
	op := m.mcp.Search
	return func() tea.Msg { f, err := op(q); return mcpSearchMsg{query: q, found: f, err: err} }
}

func (m *Model) openMCPPanel(screen int, input string) tea.Cmd {
	m.mcpPanel = &mcpPanelState{screen: screen, input: input, loading: "Reading your MCP servers …"}
	if screen == mcpAdd {
		m.mcpPanel.loading = ""
		return nil
	}
	op := m.mcp.List
	return func() tea.Msg { p, err := op(); return mcpPanelMsg{panel: p, err: err} }
}

// mcpRefresh re-reads the servers, and with them their prompts: a server
// turned on, tested, added or signed in to after the window opened used to
// keep its prompts out of the / list for the rest of the session, because
// they were fetched once at startup (live 2026-10-01: /everything:simple-prompt
// answered "not connected" while the panel showed its 13 tools).
func (m *Model) mcpRefresh() tea.Cmd {
	op := m.mcp.List
	return tea.Batch(
		func() tea.Msg { p, err := op(); return mcpPanelMsg{panel: p, err: err} },
		m.mcpLoadPrompts(),
	)
}

// mcpStartupCheck asks once, at the start, whether a project's own servers
// may start: they are commands from a repository the person may have just
// cloned.
func (m *Model) mcpStartupCheck() tea.Cmd {
	if m.mcp.List == nil {
		return nil
	}
	op := m.mcp.List
	return func() tea.Msg {
		p, err := op()
		if err != nil || p.Trusted {
			return nil
		}
		return mcpPanelMsg{panel: p, open: true}
	}
}

func (m *Model) mcpRunAction(action, name string) tea.Cmd {
	verb := map[string]string{"test": "Connecting to", "on": "Turning on", "off": "Turning off", "remove": "Removing", "logout": "Signing out of", "trust": "Trusting this project's servers"}[action]
	if m.mcpPanel != nil {
		m.mcpPanel.loading = strings.TrimSpace(verb + " " + name + " …")
	} else {
		m.note(strings.TrimSpace(verb + " " + name + " …"))
	}
	op := m.mcp.Action
	return func() tea.Msg {
		st, err := op(action, name)
		return mcpActionMsg{action: action, name: name, st: st, err: err}
	}
}

func (m *Model) mcpStartSignIn(name string) tea.Cmd {
	p := m.mcpPanel
	p.screen, p.loading, p.err, p.input = mcpSignIn, "Finding where "+name+" signs in …", "", ""
	p.signin.name = name
	op := m.mcp.LoginStart
	return func() tea.Msg {
		id, u, opened, copied, err := op(name)
		return mcpLoginStartMsg{name: name, id: id, url: u, opened: opened, copied: copied, err: err}
	}
}

func (m *Model) mcpWait(id string) tea.Cmd {
	op := m.mcp.LoginWait
	return func() tea.Msg { done, st, err := op(id); return mcpLoginMsg{id: id, done: done, st: st, err: err} }
}

func mcpTick() tea.Cmd {
	return tick(time.Second, func(time.Time) tea.Msg { return mcpTickMsg{} })
}

// updateMCPPanel handles the panel's messages and keys; handled is false
// when the message is not the panel's.
func (m *Model) updateMCPPanel(msg tea.Msg) (handled bool, cmd tea.Cmd) {
	p := m.mcpPanel
	switch t := msg.(type) {
	case mcpPanelMsg:
		// Remember the names past the panel's life: a `/server:prompt` typed
		// after it closed can then say which server is not connected.
		if m.mcpSeen == nil {
			m.mcpSeen = map[string]bool{}
		}
		for _, s := range t.panel.Servers {
			m.mcpSeen[s.Name] = true
		}
		if t.open {
			if m.mcpPanel != nil || len(projectServers(t.panel)) == 0 {
				return true, nil
			}
			m.mcpPanel = &mcpPanelState{screen: mcpList, panel: t.panel}
			return true, nil
		}
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			p.err = t.err.Error()
			return true, nil
		}
		p.panel = t.panel
		if p.index >= len(p.panel.Servers) {
			p.index = 0
		}
		return true, nil
	case mcpAddMsg:
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			p.screen, p.input, p.err = mcpAdd, p.pending, t.err.Error()
			return true, nil
		}
		for _, s := range t.servers {
			m.note(mcpResult(s))
		}
		for _, s := range t.servers {
			if !s.Saved {
				p.screen, p.input = mcpAdd, p.pending
				p.err = strings.TrimSpace(s.Error)
				return true, nil
			}
		}
		for _, s := range t.servers {
			if s.State == "needs-sign-in" {
				return true, m.mcpStartSignIn(s.Name)
			}
		}
		p.screen = mcpList
		return true, m.mcpRefresh()
	case mcpActionMsg:
		if t.err != nil {
			if p != nil {
				p.loading, p.err = "", t.err.Error()
			} else {
				m.note("✗ " + t.err.Error())
			}
			return true, nil
		}
		switch t.action {
		case "test":
			m.note(mcpResult(t.st))
		case "remove":
			m.note("Removed **" + t.name + "**.")
		case "logout":
			m.note("Signed out of **" + t.name + "**.")
		case "trust":
			m.note("This project's MCP servers start from the next turn. A change to them asks again.")
		}
		if p == nil {
			return true, nil
		}
		return true, m.mcpRefresh()
	case mcpLoginStartMsg:
		if p == nil {
			if t.id != "" {
				m.mcp.LoginCancel(t.id)
			}
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			p.err = t.err.Error()
			return true, nil
		}
		p.signin.id, p.signin.url, p.signin.opened, p.signin.copied, p.signin.started = t.id, t.url, t.opened, t.copied, time.Now()
		m.note("Sign in to **" + t.name + "**: " + t.url)
		return true, tea.Batch(m.mcpWait(t.id), mcpTick())
	case mcpLoginMsg:
		if p == nil || p.screen != mcpSignIn || p.signin.id != t.id {
			return true, nil
		}
		if t.err != nil {
			p.err = t.err.Error()
			p.signin.id = ""
			return true, nil
		}
		if !t.done {
			return true, m.mcpWait(t.id)
		}
		m.note("✓ Signed in to **" + p.signin.name + "**.")
		m.note(mcpResult(t.st))
		p.screen, p.signin.id = mcpList, ""
		return true, m.mcpRefresh()
	case mcpSearchMsg:
		if p == nil || p.screen != mcpResults || p.query != t.query {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			p.err = t.err.Error()
			return true, nil
		}
		p.found = t.found
		if len(t.found) == 0 {
			p.err = "Nothing in the MCP registry matches " + t.query + "."
		}
		return true, nil
	case mcpPromptsMsg:
		if m.mcpPrompts == nil {
			m.mcpPrompts = map[string]MCPPrompt{}
		}
		for _, pr := range t.prompts {
			if _, known := m.mcpPrompts[pr.Command()]; !known {
				m.slashCommands = append(m.slashCommands, pr.Command())
			}
			m.mcpPrompts[pr.Command()] = pr
		}
		return true, nil
	case mcpPromptTextMsg:
		if t.err != nil {
			m.note("✗ " + t.cmd + ": " + t.err.Error())
			return true, nil
		}
		if strings.TrimSpace(t.text) == "" {
			m.note(t.cmd + " gave an empty prompt.")
			return true, nil
		}
		return true, m.startTurn(t.text)
	case mcpTickMsg:
		if p != nil && p.screen == mcpSignIn && p.signin.id != "" {
			return true, mcpTick()
		}
		return true, nil
	case tea.KeyMsg:
		if p == nil {
			return false, nil
		}
		return true, m.mcpKey(t)
	}
	return false, nil
}

func projectServers(p MCPPanel) []MCPServer {
	var out []MCPServer
	for _, s := range p.Servers {
		if s.Scope == "project" {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) mcpKey(k tea.KeyMsg) tea.Cmd {
	p := m.mcpPanel
	if k.Type == tea.KeyCtrlC {
		return nil // the app's own ctrl+c handling is above the panel
	}
	switch p.screen {
	case mcpResults:
		switch k.Type {
		case tea.KeyEsc:
			p.screen, p.err = mcpList, ""
			return m.mcpRefresh()
		case tea.KeyUp:
			if p.index > 0 {
				p.index--
			}
		case tea.KeyDown:
			if p.index < len(p.found)-1 {
				p.index++
			}
		case tea.KeyEnter:
			if p.index < len(p.found) {
				f := p.found[p.index]
				p.pending, p.label, p.screen, p.err = f.Snippet, f.Suggested+" ("+f.Name+")", mcpWhere, ""
			}
		}
		return nil
	case mcpAdd:
		switch k.Type {
		case tea.KeyEsc:
			if len(p.panel.Servers) > 0 || p.err != "" {
				p.screen, p.input, p.err = mcpList, "", ""
				return m.mcpRefresh()
			}
			m.mcpPanel = nil
		case tea.KeyEnter:
			if strings.TrimSpace(p.input) == "" {
				return nil
			}
			if p.searching {
				p.searching = false
				return m.mcpSearch(strings.TrimSpace(p.input))
			}
			p.pending, p.label, p.screen, p.err = p.input, p.input, mcpWhere, ""
		case tea.KeyBackspace:
			if r := []rune(p.input); len(r) > 0 {
				p.input = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			p.input += string(k.Runes) // a space key carries its " " rune
		}
		return nil
	case mcpWhere:
		switch k.Type {
		case tea.KeyEsc:
			p.screen, p.input = mcpAdd, p.pending
		case tea.KeyUp, tea.KeyDown, tea.KeyTab:
			p.where = 1 - p.where
		case tea.KeyEnter:
			p.loading = "Connecting …"
			input, everywhere := p.pending, p.where == 1
			op := m.mcp.Add
			return func() tea.Msg { s, err := op(input, everywhere); return mcpAddMsg{servers: s, err: err} }
		case tea.KeyRunes:
			switch string(k.Runes) {
			case "1", "p":
				p.where = 0
			case "2", "e":
				p.where = 1
			}
		}
		return nil
	case mcpSignIn:
		switch k.Type {
		case tea.KeyEsc:
			if p.signin.id != "" {
				m.mcp.LoginCancel(p.signin.id)
				m.note("Sign-in to **" + p.signin.name + "** cancelled. `/mcp login " + p.signin.name + "` starts it again.")
			}
			p.screen, p.signin.id, p.input, p.err = mcpList, "", "", ""
			return m.mcpRefresh()
		case tea.KeyEnter:
			v := strings.TrimSpace(p.input)
			if v == "" || p.signin.id == "" {
				return nil
			}
			p.input = ""
			if err := m.mcp.LoginPaste(p.signin.id, v); err != nil {
				p.err = err.Error()
			}
		case tea.KeyBackspace:
			if r := []rune(p.input); len(r) > 0 {
				p.input = string(r[:len(r)-1])
			}
		case tea.KeyRunes:
			p.input += string(k.Runes)
		}
		return nil
	}

	// the list
	n := len(p.panel.Servers)
	var cur *MCPServer
	if p.index < n {
		cur = &p.panel.Servers[p.index]
	}
	if k.Type != tea.KeyRunes || string(k.Runes) != "x" {
		p.confirm = ""
	}
	switch k.Type {
	case tea.KeyEsc:
		m.mcpPanel = nil
	case tea.KeyUp:
		if p.index > 0 {
			p.index--
		}
	case tea.KeyDown:
		if n == 0 {
			if p.index < len(mcpEmptyChoices)-1 {
				p.index++
			}
		} else if p.index < n-1 {
			p.index++
		}
	case tea.KeyEnter:
		if cur == nil {
			p.screen, p.input, p.err = mcpAdd, "", ""
			p.searching = n == 0 && p.index == 1
			return nil
		}
		if cur.State == "needs-sign-in" {
			return m.mcpStartSignIn(cur.Name)
		}
		if cur.State == "not-trusted" {
			return m.mcpRunAction("trust", "")
		}
		return m.mcpRunAction("test", cur.Name)
	case tea.KeySpace:
		if cur != nil {
			if cur.State == "off" {
				return m.mcpRunAction("on", cur.Name)
			}
			return m.mcpRunAction("off", cur.Name)
		}
	case tea.KeyRunes:
		switch string(k.Runes) {
		case "a":
			p.screen, p.input, p.err, p.searching = mcpAdd, "", "", false
		case "t":
			if cur != nil {
				return m.mcpRunAction("test", cur.Name)
			}
		case "l":
			if cur != nil && cur.Transport == "http" {
				return m.mcpStartSignIn(cur.Name)
			}
		case "o":
			if cur != nil && cur.Transport == "http" {
				return m.mcpRunAction("logout", cur.Name)
			}
		case "y":
			if !p.panel.Trusted {
				return m.mcpRunAction("trust", "")
			}
		case "s":
			p.screen, p.input, p.err, p.searching = mcpAdd, "", "", true
		case "x":
			if cur == nil {
				return nil
			}
			if p.confirm == cur.Name {
				p.confirm = ""
				return m.mcpRunAction("remove", cur.Name)
			}
			p.confirm = cur.Name
		}
	}
	return nil
}

// mcpEmptyChoices is what the panel offers when no server is configured,
// in the order the cursor walks them.
var mcpEmptyChoices = []struct{ what, how string }{
	{"Add a server", "a URL, a command, or a .mcp.json snippet"},
	{"Search the registry", "find one by name"},
}

// mcpResult is a server's state in the transcript, after an add, a test
// or a sign-in.
func mcpResult(s MCPServer) string {
	switch s.State {
	case "connected":
		tools := strings.Join(s.Tools, ", ")
		if len(s.Tools) > 6 {
			tools = strings.Join(s.Tools[:6], ", ") + fmt.Sprintf(", … %d more", len(s.Tools)-6)
		}
		srv := ""
		if s.Server != "" {
			srv = " (" + s.Server + ")"
		}
		return fmt.Sprintf("✓ **%s** connected%s: %d tools — %s. The coder can use them from the next turn.", s.Name, srv, len(s.Tools), tools)
	case "needs-sign-in":
		return fmt.Sprintf("◐ **%s** needs sign-in.", s.Name)
	case "failed": // the reason already says whether it was kept
		return fmt.Sprintf("✗ **%s**: %s", s.Name, s.Error)
	case "not-trusted":
		return fmt.Sprintf("? **%s** is in this project's .mcp.json: it starts after your yes (/mcp trust).", s.Name)
	}
	return fmt.Sprintf("%s **%s**: %s", mcpGlyph(s.State), s.Name, s.State)
}

func mcpGlyph(state string) string {
	switch state {
	case "connected":
		return "●"
	case "needs-sign-in":
		return "◐"
	case "failed":
		return "✗"
	case "off":
		return "○"
	case "not-trusted":
		return "?"
	case "connecting":
		return "◌"
	}
	return "·"
}

// renderMCPPanel draws the overlay.
func (m Model) renderMCPPanel() string {
	p := m.mcpPanel
	if p == nil {
		return ""
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colText)).PaddingLeft(2)
	norm := lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG)).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	keys := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).PaddingLeft(2)
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
	good := lipgloss.NewStyle().Foreground(lipgloss.Color(colOK))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color(colRun))
	box := func(s string) string {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colDim)).Padding(0, 1).Width(boxWidth(m.width)).Render(s)
	}
	var b strings.Builder
	errLine := ""
	if p.err != "" {
		errLine = "\n" + bad.Render("✗ "+p.err)
	}
	if p.loading != "" {
		return title.Render(p.loading)
	}
	switch p.screen {
	case mcpAdd:
		if p.searching {
			b.WriteString(title.Render("Search the MCP registry") + "\n")
			b.WriteString(box(norm.Render("> "+p.input+"▏")+"\n"+
				dim.Render("Words to look for: github, postgres, linear … (registry.modelcontextprotocol.io)")+errLine) + "\n")
			b.WriteString(keys.Render("enter search · esc back"))
			return b.String()
		}
		b.WriteString(title.Render("Add an MCP server") + "\n")
		b.WriteString(box(norm.Render("> "+p.input+"▏")+"\n"+
			dim.Render("A URL (https://mcp.linear.app/mcp), a command (npx -y @modelcontextprotocol/server-github),\nor a .mcp.json snippet. HTTP or local follows from what you paste. Not sure? esc, then s searches the registry.")+errLine) + "\n")
		b.WriteString(keys.Render("enter next · esc back"))
		return b.String()
	case mcpWhere:
		opts := []string{"This project  (.mcp.json — share it with the repo)", "Every project  (~/.memdoor/mcp.json — just you)"}
		label := p.label
		if label == "" {
			label = p.pending
		}
		b.WriteString(title.Render("Save "+truncateMiddle(label, 60)+" for:") + "\n")
		for i, o := range opts {
			if i == p.where {
				b.WriteString("  " + sel.Render("→ "+o) + "\n")
			} else {
				b.WriteString("  " + norm.Render("  "+o) + "\n")
			}
		}
		b.WriteString(keys.Render("↑↓ choose · enter connect · esc back"))
		return b.String()
	case mcpResults:
		b.WriteString(title.Render("MCP registry: "+p.query) + "\n")
		var rows []string
		// The description takes what the box leaves: its width less the
		// border and padding (4), the arrow (2), and the name and via columns.
		descW := boxWidth(m.width) - 4 - 2 - 22 - 1 - 7 - 1
		if descW < 10 {
			descW = 10
		}
		for i, f := range p.found {
			row := fmt.Sprintf("%-22s %-7s %s", truncateMiddle(f.Suggested, 22), f.Via, truncateEnd(f.Description, descW))
			if i == p.index {
				rows = append(rows, sel.Render("→ "+row))
				detail := dim.Render("    " + f.Name + " " + f.Version)
				if len(f.Needs) > 0 {
					detail += "\n" + warn.Render("    needs "+strings.Join(f.Needs, ", ")+" set where the gateway runs")
				}
				rows = append(rows, detail)
			} else {
				rows = append(rows, "  "+norm.Render(row))
			}
		}
		if len(rows) == 0 {
			rows = append(rows, dim.Render("No results."))
		}
		maxRows := m.height - 14
		if maxRows < 6 {
			maxRows = 6
		}
		if len(rows) > maxRows {
			start := p.index - maxRows/2
			if start < 0 {
				start = 0
			}
			if start > len(rows)-maxRows {
				start = len(rows) - maxRows
			}
			rows = rows[start : start+maxRows]
		}
		b.WriteString(box(strings.Join(rows, "\n")+errLine) + "\n")
		b.WriteString(keys.Render("↑↓ move · enter add · esc back"))
		return b.String()
	case mcpSignIn:
		s := p.signin
		lines := []string{lipgloss.NewStyle().Bold(true).Render("Sign in to " + s.name)}
		switch {
		case s.opened:
			lines = append(lines, "Your browser is open: approve Memdoor there.")
		default:
			lines = append(lines, "Open this link to approve Memdoor:")
		}
		if s.url != "" {
			link := truncateMiddle(s.url, 90)
			if s.copied {
				link += dim.Render("  (copied)")
			}
			lines = append(lines, link)
		}
		if s.id != "" {
			el := time.Since(s.started).Round(time.Second)
			lines = append(lines, warn.Render(fmt.Sprintf("◌ Waiting for you to approve… %s", el)))
			lines = append(lines, dim.Render("Browser on another machine? Paste the page's address here:"), norm.Render("> "+p.input+"▏"))
		}
		if p.err != "" {
			// A paste that is not the address gets sent as a code and the
			// provider refuses it ("Invalid authorization code format"); say
			// what the box actually wants.
			lines = append(lines, dim.Render("The address is the one the browser ends on — it carries ?code=…"))
		}
		b.WriteString(box(strings.Join(lines, "\n")+errLine) + "\n")
		b.WriteString(keys.Render("enter submits a pasted address · esc cancels"))
		return b.String()
	}

	// the list
	b.WriteString(title.Render("MCP servers — the coder's extra tools") + "\n")
	if len(p.panel.Servers) == 0 {
		// Nothing to point at, so the cursor points at what to do: the two
		// ways in are rows like any other, and enter takes the one chosen.
		var rows []string
		for i, c := range mcpEmptyChoices {
			row := fmt.Sprintf("%-20s%s", c.what, c.how)
			if i == p.index {
				rows = append(rows, sel.Render("→ "+row))
			} else {
				rows = append(rows, "  "+dim.Render(row))
			}
		}
		b.WriteString(box(norm.Render("No MCP servers yet.")+"\n\n"+strings.Join(rows, "\n")+errLine) + "\n")
		b.WriteString(keys.Render("↑↓ move · enter choose · esc close"))
		return b.String()
	}
	w := 0
	for _, s := range p.panel.Servers {
		if len(s.Name) > w {
			w = len(s.Name)
		}
	}
	var rows []string
	for i, s := range p.panel.Servers {
		what := s.State
		style := norm
		switch s.State {
		case "connected":
			what, style = fmt.Sprintf("%d tools", len(s.Tools)), good
		case "needs-sign-in":
			what, style = "needs sign-in — enter", warn
		case "failed":
			what, style = s.Error, bad
		case "not-trusted":
			what, style = "waiting for your yes — y", warn
		case "idle":
			what = "starts on first use"
		case "off":
			style = dim
		}
		// What the row says takes the room the box leaves after the glyph,
		// the name, the transport and the scope columns (and the arrow).
		whatW := boxWidth(m.width) - 4 - 2 - (2 + w + 2 + 5 + 2 + 7 + 2)
		if whatW < 12 {
			whatW = 12
		}
		row := fmt.Sprintf("%s %-*s  %-5s  %-7s  %s", mcpGlyph(s.State), w, s.Name, s.Transport, s.Scope, truncateEnd(what, whatW))
		if i == p.index {
			rows = append(rows, sel.Render("→ "+row))
		} else {
			rows = append(rows, "  "+style.Render(row))
		}
		if p.confirm == s.Name {
			rows = append(rows, bad.Render("    press x again to remove "+s.Name))
		}
	}
	body := strings.Join(rows, "\n")
	if !p.panel.Trusted && len(projectServers(p.panel)) > 0 {
		body += "\n\n" + warn.Render("This project's .mcp.json would start:")
		for _, s := range projectServers(p.panel) {
			body += "\n  " + dim.Render(s.Name+"  "+truncateMiddle(s.Target, 70))
		}
		body += "\n" + warn.Render("Press y to let them start (a change to them asks again).")
	}
	b.WriteString(box(body+errLine) + "\n")
	b.WriteString(keys.Render("↑↓ move · enter sign in/test · a add · s search the registry · space on/off · t test · o sign out · x remove · esc close"))
	return b.String()
}

// boxWidth is the panel box's width for a terminal width.
func boxWidth(termWidth int) int {
	w := termWidth - 8
	if w > 100 {
		w = 100
	}
	if w < 20 {
		w = 20
	}
	return w
}

// truncateEnd shortens s to n runes, ending in "…".
func truncateEnd(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n || n < 2 {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}

// truncateMiddle shortens s to n runes, keeping both ends.
func truncateMiddle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n || n < 10 {
		return s
	}
	half := (n - 1) / 2
	return string(r[:half]) + "…" + string(r[len(r)-half:])
}
