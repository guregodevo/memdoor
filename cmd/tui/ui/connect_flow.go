package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// /connect — a provider from inside the window (Greg, 2026-10-02: "i dont
// see the /connect … omp does it with a slash command right?"). omp's
// /login is a provider list, then "Paste your X API key", then "✓ logged in
// to X". Here: pick the kind (1-5, as the ask_user_question picker), the
// base URL prefilled, the token typed into the input line but drawn as
// dots, a model only where a gateway may list none — then the gateway
// probes (its list, one call) and the ✓ line says what it found. Esc at
// any step cancels. The shell's `memdoor connect` is the same request.

// ConnectKind is one entry of the kind picker; the shell command shows the
// same list.
type ConnectKind struct {
	ID, Name, API, Base, Hint string
	AsksModel                 bool   // a gateway or custom endpoint may list no model
	KeyVar                    string // the variable the key is usually under, prefilled on the token step; "" when not obvious (a company gateway names its own)
}

var ConnectKinds = []ConnectKind{
	{"gateway", "Company AI gateway", "", "https://ai-gateway.example.com/ai", "the base before /v1; a personal or service-account token", true, ""},
	// Sign in with ChatGPT (2026-10-10): a Plus or Pro plan's allowance, no
	// key; the browser step runs on the gateway (gateway/providers_chatgpt.go).
	{"chatgpt", "ChatGPT plan (Plus/Pro)", "chatgpt", "https://api.openai.com/v1", "sign in with ChatGPT in the browser: your plan's allowance, no API key", false, ""},
	{"openrouter", "OpenRouter", "openrouter", "https://openrouter.ai/api/v1", "an API key from openrouter.ai/keys: every model, one key", false, "OPEN_ROUTER_API_KEY"},
	{"anthropic", "Anthropic", "anthropic", "https://api.anthropic.com", "an API key from console.anthropic.com", false, "ANTHROPIC_API_KEY"},
	{"openai", "OpenAI", "chat", "https://api.openai.com/v1", "an API key from platform.openai.com", false, "OPENAI_API_KEY"},
	{"gemini", "Google Gemini", "chat", "https://generativelanguage.googleapis.com/v1beta/openai", "an API key from Google AI Studio", false, "GEMINI_API_KEY"},
	{"xai", "xAI Grok", "chat", "https://api.x.ai/v1", "an API key from console.x.ai", false, "XAI_API_KEY"},
	{"baseten", "Baseten", "chat", "https://inference.baseten.co/v1", "an API key from app.baseten.co", false, "BASETEN_API_KEY"},
	{"groq", "Groq", "chat", "https://api.groq.com/openai/v1", "an API key from console.groq.com", false, "GROQ_API_KEY"},
	{"deepseek", "DeepSeek", "chat", "https://api.deepseek.com/v1", "an API key from platform.deepseek.com", false, "DEEPSEEK_API_KEY"},
	{"typesafe", "TypeSafe (Jev, decisions)", "decisions", "https://api.typesafe.ai", "a key from typesafe.ai: the decision model, with any chat provider", false, "TYPESAFE_API_KEY"},
	{"custom", "Any OpenAI-compatible endpoint", "", "https://host/v1", "a base URL and its key", true, ""},
}

// ConnectRequest and ConnectResult mirror the gateway's
// /api/providers/connect (gateway/providers_connect.go).
type ConnectRequest struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	API   string `json:"api"`
	Base  string `json:"base"`
	Key   string `json:"key"`
	Model string `json:"model"`
	Probe bool   `json:"probe"`
}

type ConnectResult struct {
	OK     bool     `json:"ok"`
	ID     string   `json:"id"`
	API    string   `json:"api"`
	Models int      `json:"models"`
	Sample []string `json:"sample"`
	Tested string   `json:"tested"`
	Answer string   `json:"answer"`
	Error  string   `json:"error"`
	Advice string   `json:"advice"`
	Saved  bool     `json:"saved"`
}

// ConnectOps is the one call /connect makes, and the browser sign-in's
// four (Sign in with ChatGPT: start, wait, a pasted link, cancel), the
// same shape as the MCP sign-in's.
type ConnectOps struct {
	Connect func(req ConnectRequest) (ConnectResult, error)
	// KeySource is where a provider's key already comes from on the gateway
	// ("env ANTHROPIC_API_KEY", "providers.json", or ""), for the prefill.
	KeySource func(id string) string
	// LoginStart begins a browser sign-in for a kind: its id, the link,
	// whether a browser opened and whether the link was copied.
	LoginStart func(kind string) (id, url string, opened, copied bool, err error)
	// LoginWait waits up to ~25 s: done with the probe's result, or not yet.
	LoginWait func(id string) (done bool, res ConnectResult, err error)
	// LoginPaste hands the sign-in a redirect URL pasted in the window.
	LoginPaste  func(id, value string) error
	LoginCancel func(id string)
}

const (
	connectStepKind = iota
	connectStepID
	connectStepBase
	connectStepKey
	connectStepModel
	connectStepProbing
	connectStepBrowser // a sign-in in the browser; the window waits
)

// connectFlow is the window's state while /connect runs.
type connectFlow struct {
	step   int
	index  int // the kind picker's cursor
	kind   ConnectKind
	id     string
	base   string
	key    string
	model  string
	keyVar string // prefilled on the token step: the kind's variable, or the one the gateway has
	keySet bool   // the gateway already holds it
	// The browser sign-in, once started.
	loginID  string
	loginURL string
	opened   bool
	copied   bool
}

type connectResultMsg struct {
	req ConnectRequest
	res ConnectResult
	err error
}

// connectLoginStartedMsg is the browser sign-in's start: the link to open.
type connectLoginStartedMsg struct {
	id, url        string
	opened, copied bool
	err            error
}

// connectLoginWaitMsg is one wait on the sign-in: done, or ask again.
type connectLoginWaitMsg struct {
	id   string
	done bool
	res  ConnectResult
	err  error
}

// connectCommand is "/connect [kind]".
func (m *Model) connectCommand(args []string) tea.Cmd {
	if m.connectOps.Connect == nil {
		m.note("Connecting a provider isn't available in this build.")
		return nil
	}
	f := &connectFlow{}
	if len(args) > 0 {
		for _, k := range ConnectKinds {
			if k.ID == strings.ToLower(args[0]) {
				f.kind = k
				f.step = connectStepBase
				if k.ID == "custom" {
					f.step = connectStepID
				}
			}
		}
		if f.kind.ID == "" {
			m.note(fmt.Sprintf("No provider kind **%s** — /connect lists them.", args[0]))
			return nil
		}
	}
	m.connect = f
	m.input.Reset()
	if f.kind.ID != "" && connectKindSignsIn(f.kind) {
		return m.connectLoginStart()
	}
	m.connectPrefill()
	return nil
}

// connectKindSignsIn says whether a kind is a browser sign-in rather than a
// pasted key.
func connectKindSignsIn(k ConnectKind) bool { return k.API == "chatgpt" }

// connectLoginStart begins the browser sign-in on the gateway.
func (m *Model) connectLoginStart() tea.Cmd {
	f := m.connect
	f.step = connectStepBrowser
	if m.connectOps.LoginStart == nil {
		m.connect = nil
		m.note("Signing in with " + f.kind.Name + " isn't available in this build.")
		return nil
	}
	kind, start := f.kind.ID, m.connectOps.LoginStart
	return func() tea.Msg {
		id, u, opened, copied, err := start(kind)
		return connectLoginStartedMsg{id: id, url: u, opened: opened, copied: copied, err: err}
	}
}

// connectLoginStarted shows the link and starts waiting.
func (m *Model) connectLoginStarted(msg connectLoginStartedMsg) tea.Cmd {
	f := m.connect
	if f == nil {
		return nil
	}
	if msg.err != nil {
		m.connect = nil
		m.note("✗ the sign-in could not start: " + msg.err.Error())
		return nil
	}
	f.loginID, f.loginURL, f.opened, f.copied = msg.id, msg.url, msg.opened, msg.copied
	return m.connectLoginWait()
}

func (m *Model) connectLoginWait() tea.Cmd {
	f := m.connect
	if f == nil || m.connectOps.LoginWait == nil {
		return nil
	}
	id, wait := f.loginID, m.connectOps.LoginWait
	return func() tea.Msg {
		done, res, err := wait(id)
		return connectLoginWaitMsg{id: id, done: done, res: res, err: err}
	}
}

// connectLoginWaited is one answer of the wait: the result, or ask again.
func (m *Model) connectLoginWaited(msg connectLoginWaitMsg) tea.Cmd {
	f := m.connect
	if f == nil || f.loginID != msg.id {
		return nil
	}
	if msg.err != nil {
		m.connect = nil
		m.input.Reset()
		m.note("✗ not signed in: " + msg.err.Error())
		return nil
	}
	if !msg.done {
		return m.connectLoginWait()
	}
	m.connectDone(connectResultMsg{req: ConnectRequest{ID: f.kind.ID, Name: f.kind.Name, API: f.kind.API}, res: msg.res})
	return nil
}

// connectPrefill puts the step's default in the input line.
func (m *Model) connectPrefill() {
	f := m.connect
	m.input.Reset()
	switch f.step {
	case connectStepBase:
		m.input.SetValue(f.kind.Base)
		m.input.CursorEnd()
	case connectStepKey:
		// The variable NAME, prefilled (Greg, 2026-10-03: "prefill default
		// KEY NAME or value if you find it … unless it's not obvious like for
		// company gateway"): the kind's usual name, or the one the gateway
		// already holds — Enter keeps it. The value itself is never on screen.
		f.keyVar, f.keySet = f.kind.KeyVar, false
		if m.connectOps.KeySource != nil {
			if src, ok := strings.CutPrefix(m.connectOps.KeySource(f.kind.ID), "env "); ok && src != "" {
				f.keyVar, f.keySet = src, true
			}
		}
		if f.keyVar != "" {
			m.input.SetValue(f.keyVar)
			m.input.CursorEnd()
		}
	}
}

// connectKey handles keys while /connect is open. handled is false when the
// key is not the flow's (the text steps let the input line have it).
func (m *Model) connectKey(k tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	f := m.connect
	if f == nil {
		return false, nil
	}
	if k.Type == tea.KeyEsc {
		if f.step == connectStepBrowser && f.loginID != "" && m.connectOps.LoginCancel != nil {
			m.connectOps.LoginCancel(f.loginID)
		}
		m.connect = nil
		m.input.Reset()
		m.note("Connect cancelled.")
		return true, nil
	}
	switch f.step {
	case connectStepBrowser:
		// A pasted redirect URL, for a browser on another machine.
		if k.Type == tea.KeyEnter {
			v := strings.TrimSpace(m.input.Value())
			m.input.Reset()
			if v != "" && f.loginID != "" && m.connectOps.LoginPaste != nil {
				if err := m.connectOps.LoginPaste(f.loginID, v); err != nil {
					m.note("✗ " + err.Error())
				}
			}
			return true, nil
		}
		return false, nil
	case connectStepKind:
		switch {
		case k.Type == tea.KeyUp:
			f.index = (f.index - 1 + len(ConnectKinds)) % len(ConnectKinds)
		case k.Type == tea.KeyDown:
			f.index = (f.index + 1) % len(ConnectKinds)
		case k.Type == tea.KeyEnter:
			return true, m.connectPick(f.index)
		case k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] >= '1' && k.Runes[0] <= '9':
			if n := int(k.Runes[0] - '1'); n < len(ConnectKinds) {
				return true, m.connectPick(n)
			}
		}
		return true, nil
	case connectStepProbing:
		return true, nil
	}
	if k.Type == tea.KeyEnter {
		return true, m.connectEnter(strings.TrimSpace(m.input.Value()))
	}
	return false, nil
}

func (m *Model) connectPick(n int) tea.Cmd {
	f := m.connect
	f.kind = ConnectKinds[n]
	if connectKindSignsIn(f.kind) {
		m.input.Reset()
		return m.connectLoginStart()
	}
	f.step = connectStepBase
	if f.kind.ID == "custom" {
		f.step = connectStepID
	}
	m.connectPrefill()
	return nil
}

// connectEnter takes the input line for the current step and moves on; the
// last step sends the request.
func (m *Model) connectEnter(value string) tea.Cmd {
	f := m.connect
	switch f.step {
	case connectStepID:
		if value == "" {
			return nil
		}
		f.id = strings.ToLower(value)
		f.step = connectStepBase
	case connectStepBase:
		if value == "" {
			value = f.kind.Base
		}
		f.base = value
		f.step = connectStepKey
	case connectStepKey:
		if value == "" {
			return nil
		}
		f.key = value
		if f.kind.AsksModel {
			f.step = connectStepModel
		} else {
			return m.connectSend()
		}
	case connectStepModel:
		f.model = value
		return m.connectSend()
	}
	m.connectPrefill()
	return nil
}

func (m *Model) connectSend() tea.Cmd {
	f := m.connect
	f.step = connectStepProbing
	m.input.Reset()
	id := f.id
	if id == "" {
		id = f.kind.ID
	}
	req := ConnectRequest{ID: id, Name: f.kind.Name, API: f.kind.API, Base: f.base, Key: f.key, Model: f.model}
	op := m.connectOps.Connect
	return func() tea.Msg { res, err := op(req); return connectResultMsg{req: req, res: res, err: err} }
}

// connectDone is the result in the conversation: the ✓ line, or the
// refusal with its advice.
func (m *Model) connectDone(msg connectResultMsg) {
	m.connect = nil
	m.input.Reset()
	switch {
	case msg.err != nil:
		m.note("✗ not connected: " + msg.err.Error())
	case !msg.res.OK:
		s := "✗ **" + msg.req.ID + "** not connected: " + msg.res.Error
		if msg.res.Advice != "" {
			s += "\n  " + msg.res.Advice
		}
		m.note(s)
	case msg.req.API == "chatgpt":
		r := msg.res
		s := fmt.Sprintf("✓ **ChatGPT plan** connected · %d models · tested %s → %q · your plan's allowance answers, no API key", r.Models, r.Tested, r.Answer)
		if len(r.Sample) > 0 {
			s += "\n  e.g. " + strings.Join(r.Sample, ", ")
		}
		s += "\n  /model <id> pins one of its models for this conversation; ChatGPT → Settings → Usage shows the weekly cap per app."
		m.note(s)
	default:
		r := msg.res
		s := fmt.Sprintf("✓ **%s** connected · %s API · %d models · tested %s → %q · kept in ~/.memdoor/providers.json", r.ID, r.API, r.Models, r.Tested, r.Answer)
		if len(r.Sample) > 0 {
			s += "\n  e.g. " + strings.Join(r.Sample, ", ")
		}
		s += "\n  /model <id> pins one of its models for this conversation; /model-search <name> finds them."
		m.note(s)
	}
}

// renderConnect is the footer while /connect runs: the kind list, or the
// step's prompt over the input line — the token drawn as dots.
func (m Model) renderConnect(inputStyle, helpStyle lipgloss.Style) string {
	f := m.connect
	w := m.width - 4
	if w < 20 {
		w = 20
	}
	head := lipgloss.NewStyle().Foreground(lipgloss.Color(colAsk)).Bold(true).Width(w)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Width(w)
	var b strings.Builder
	switch f.step {
	case connectStepKind:
		b.WriteString(head.Render("Connect a provider — which kind?") + "\n")
		for i, k := range ConnectKinds {
			line := fmt.Sprintf("  %d. %-32s %s", i+1, k.Name, k.Hint)
			if i == f.index {
				b.WriteString(head.Render(">"+line[1:]) + "\n")
			} else {
				b.WriteString(dim.Render(line) + "\n")
			}
		}
		b.WriteString(helpStyle.Render("up/down move · enter select · 1-9 pick · esc cancel"))
		return inputStyle.Render(b.String())
	case connectStepID:
		b.WriteString(head.Render(f.kind.Name+" — a short id for it (letters, digits, dashes)") + "\n")
		b.WriteString(m.input.View())
	case connectStepBase:
		b.WriteString(head.Render(f.kind.Name+" — base URL") + "\n")
		b.WriteString(dim.Render(f.kind.Hint) + "\n")
		b.WriteString(m.input.View())
	case connectStepKey:
		b.WriteString(head.Render(f.kind.Name+" — token or key (not shown)") + "\n")
		switch {
		case f.keySet:
			b.WriteString(dim.Render(f.keyVar+" is set on the gateway — enter keeps it; or type the value, another variable's NAME, or !command") + "\n")
		case f.keyVar != "":
			b.WriteString(dim.Render(f.keyVar+" prefilled (not set on the gateway yet) — or type the value, a variable's NAME, or !command that prints it") + "\n")
		default:
			b.WriteString(dim.Render("the value, the NAME of an environment variable the gateway has, or !command that prints it") + "\n")
		}
		typed := strings.TrimSpace(m.input.Value())
		shown := strings.Repeat("•", len([]rune(typed)))
		if typed != "" && typed == f.keyVar {
			shown = typed // a variable's name is not a secret
		}
		b.WriteString("> " + shown)
	case connectStepModel:
		b.WriteString(head.Render(f.kind.Name+" — a model to test with (blank: the first it lists)") + "\n")
		b.WriteString(m.input.View())
	case connectStepProbing:
		b.WriteString(head.Render("Probing " + f.base + " — its model list, then one call …"))
		return inputStyle.Render(b.String())
	case connectStepBrowser:
		if f.loginURL == "" {
			b.WriteString(head.Render("Sign in with ChatGPT — starting …"))
			return inputStyle.Render(b.String())
		}
		b.WriteString(head.Render("Sign in with ChatGPT — approve in the browser") + "\n")
		switch {
		case f.opened:
			b.WriteString(dim.Render("Your browser is opening. If it doesn't, open this link:") + "\n")
		default:
			b.WriteString(dim.Render("Open this link to approve:") + "\n")
		}
		b.WriteString(dim.Render("  "+f.loginURL) + "\n")
		if f.copied {
			b.WriteString(dim.Render("  (copied to the clipboard)") + "\n")
		}
		b.WriteString(dim.Render("Waiting (5 minutes). A browser on another machine: paste the page's address here and press enter.") + "\n")
		b.WriteString(m.input.View())
		b.WriteString("\n" + helpStyle.Render("esc cancel"))
		return inputStyle.Render(b.String())
	}
	b.WriteString("\n" + helpStyle.Render("enter next · esc cancel"))
	return inputStyle.Render(b.String())
}
