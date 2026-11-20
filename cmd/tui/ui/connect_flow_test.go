package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func press(t tea.KeyType, r ...rune) tea.KeyMsg { return tea.KeyMsg{Type: t, Runes: r} }

// /connect in the window: pick a kind, the base prefilled, the token typed
// but drawn as dots, one request to the gateway, the ✓ line in the
// conversation; Esc cancels at any step.
func TestConnectFlowPicksTypesProbesAndSays(t *testing.T) {
	var got ConnectRequest
	m := &Model{width: 100, input: textarea.New()}
	m.connectOps = ConnectOps{KeySource: func(id string) string {
		if id == "anthropic" {
			return "env ANTHROPIC_API_KEY"
		}
		return ""
	}, Connect: func(req ConnectRequest) (ConnectResult, error) {
		got = req
		return ConnectResult{OK: true, ID: req.ID, API: "anthropic", Models: 7, Tested: "claude-haiku-4-5-20251001", Answer: "ok", Saved: true, Sample: []string{"claude-haiku-4-5-20251001"}}, nil
	}}
	if cmd := m.connectCommand(nil); cmd != nil || m.connect == nil || m.connect.step != connectStepKind {
		t.Fatal("/connect opens the kind picker")
	}
	if v := m.renderConnect(lipglossPlain(), lipglossPlain()); !strings.Contains(v, "which kind") || !strings.Contains(v, "1. Sign in with OpenRouter") || !strings.Contains(v, "2. ChatGPT") || !strings.Contains(v, "3. Local runtime") || !strings.Contains(v, "4. OpenRouter") || !strings.Contains(v, "5. AI/ML API") || !strings.Contains(v, "6. EUrouter") || !strings.Contains(v, "7. Vercel AI Gateway") || !strings.Contains(v, "17. Anthropic") {
		t.Fatalf("picker: %q", v)
	}
	// Anthropic is the last chat provider (2026-11-20), past the digits:
	// sixteen downs and Enter.
	for i := 0; i < 16; i++ {
		m.connectKey(press(tea.KeyDown))
	}
	if h, _ := m.connectKey(press(tea.KeyEnter)); !h || m.connect.kind.ID != "anthropic" || m.connect.step != connectStepBase || m.input.Value() != "https://api.anthropic.com" {
		t.Fatalf("the 17th row picks Anthropic and prefills its base: %+v %q", m.connect, m.input.Value())
	}
	if h, _ := m.connectKey(press(tea.KeyEnter)); !h || m.connect.step != connectStepKey || m.connect.base != "https://api.anthropic.com" {
		t.Fatalf("enter keeps the base: %+v", m.connect)
	}
	if m.input.Value() != "ANTHROPIC_API_KEY" || !m.connect.keySet {
		t.Fatalf("prefilled from the gateway's environment: %q %v", m.input.Value(), m.connect.keySet)
	}
	if v := m.renderConnect(lipglossPlain(), lipglossPlain()); !strings.Contains(v, "> ANTHROPIC_API_KEY") || !strings.Contains(v, "is set on the gateway") {
		t.Fatalf("the prefilled name is shown and explained: %q", v)
	}
	m.input.SetValue("sk-ant-typed-secret")
	if v := m.renderConnect(lipglossPlain(), lipglossPlain()); strings.Contains(v, "sk-ant") || !strings.Contains(v, "•••••") {
		t.Fatalf("a typed value is drawn as dots: %q", v)
	}
	m.input.SetValue("ANTHROPIC_API_KEY")
	if h, _ := m.connectKey(press(tea.KeyRunes, 'x')); h || m.input.Value() != "" {
		t.Fatalf("a letter on a text step goes to the input line, in place of the prefilled name: %q", m.input.Value())
	}
	m.input.SetValue("ANTHROPIC_API_KEY")
	h, cmd := m.connectKey(press(tea.KeyEnter))
	if !h || cmd == nil || m.connect.step != connectStepProbing {
		t.Fatalf("enter on the key sends (Anthropic asks no model): %+v", m.connect)
	}
	msg := cmd()
	r, ok := msg.(connectResultMsg)
	if !ok || got.ID != "anthropic" || got.API != "anthropic" || got.Key != "ANTHROPIC_API_KEY" || got.Base != "https://api.anthropic.com" {
		t.Fatalf("the request: %+v", got)
	}
	m.connectDone(r)
	last := m.messages[len(m.messages)-1].Content
	if m.connect != nil || !strings.Contains(last, "✓ **anthropic** connected") || !strings.Contains(last, "7 models") || !strings.Contains(last, "/model <id>") {
		t.Fatalf("the ✓ line: %q", last)
	}

	// A gateway asks a model; a refusal shows the advice; Esc cancels.
	m.connectCommand([]string{"gateway"})
	if m.connect.step != connectStepBase || m.connect.kind.ID != "gateway" {
		t.Fatalf("/connect gateway skips the picker: %+v", m.connect)
	}
	m.input.SetValue("https://ai-gateway.corp.example/ai")
	m.connectKey(press(tea.KeyEnter))
	if m.input.Value() != "" {
		t.Fatalf("a company gateway's variable is not obvious: nothing prefilled, got %q", m.input.Value())
	}
	m.input.SetValue("pat")
	if h, _ := m.connectKey(press(tea.KeyEnter)); !h || m.connect.step != connectStepModel {
		t.Fatalf("a gateway asks a model: %+v", m.connect)
	}
	if h, _ := m.connectKey(press(tea.KeyEsc)); !h || m.connect != nil || !strings.Contains(m.messages[len(m.messages)-1].Content, "cancelled") {
		t.Fatal("esc cancels")
	}
	m.connectDone(connectResultMsg{req: ConnectRequest{ID: "corp"}, res: ConnectResult{Error: "HTTP 401", Advice: "check the token"}})
	if last := m.messages[len(m.messages)-1].Content; !strings.Contains(last, "✗ **corp** not connected: HTTP 401") || !strings.Contains(last, "check the token") {
		t.Fatalf("refusal with advice: %q", last)
	}
}

func lipglossPlain() lipgloss.Style { return lipgloss.NewStyle() }

// A local runtime (Ollama, llama.cpp, janis, LM Studio) has no token: /connect
// local goes from the base URL to the model question and sends keyless.
func TestConnectALocalRuntimeAsksNoKey(t *testing.T) {
	var got ConnectRequest
	m := &Model{width: 100, input: textarea.New()}
	m.connectOps = ConnectOps{Connect: func(req ConnectRequest) (ConnectResult, error) {
		got = req
		return ConnectResult{OK: true, ID: req.ID, API: "chat", Models: 3, Tested: "qwen3:8b", Answer: "ok", Saved: true}, nil
	}}
	m.connectCommand([]string{"local"})
	if m.connect == nil || m.connect.kind.ID != "local" || m.connect.step != connectStepBase || m.input.Value() != "" {
		t.Fatalf("/connect local starts at the base, blank (the gateway finds a runtime): %+v %q", m.connect, m.input.Value())
	}
	if h, _ := m.connectKey(press(tea.KeyEnter)); !h || m.connect.step != connectStepModel {
		t.Fatalf("enter on the base skips the token step: %+v", m.connect)
	}
	h, cmd := m.connectKey(press(tea.KeyEnter))
	if !h || cmd == nil || m.connect.step != connectStepProbing {
		t.Fatalf("a blank model sends: %+v", m.connect)
	}
	cmd()
	if !got.Keyless || got.Key != "" || got.ID != "local" || got.API != "chat" || got.Base != "" {
		t.Fatalf("the request says keyless and carries no token: %+v", got)
	}
}

// The sign-in screen names the provider being signed in to (live
// 2026-10-31: the OpenRouter link sat under "Sign in with ChatGPT").
func TestSignInScreenNamesItsProvider(t *testing.T) {
	for _, k := range ConnectKinds {
		if !connectKindSignsIn(k) {
			continue
		}
		m := &Model{width: 120, input: textarea.New()}
		m.connect = &connectFlow{step: connectStepBrowser, kind: k, loginURL: "https://example.test/auth"}
		v := m.renderConnect(lipglossPlain(), lipglossPlain())
		want, other := "Sign in with ChatGPT", "OpenRouter"
		if k.ID == KindOpenRouterLogin {
			want, other = "Sign in with OpenRouter", "ChatGPT"
		}
		if !strings.Contains(v, want) || strings.Contains(v, other) {
			t.Fatalf("%s: the screen must say %q and not %q:\n%s", k.ID, want, other, v)
		}
	}
}

// A key already on this machine is listed before the kinds: Enter on it
// connects it, the kinds keep working below it (the mutation check: drop
// f.found and "1. OpenRouter  $MY_OR_KEY" is not drawn).
func TestConnectListsFoundKeysFirst(t *testing.T) {
	picked := -1
	m := &Model{width: 120, input: textarea.New()}
	m.connectOps = ConnectOps{
		Connect: func(req ConnectRequest) (ConnectResult, error) { return ConnectResult{}, nil },
		Found:   func() []string { return []string{"OpenRouter  $MY_OR_KEY", "Anthropic  .env: ANTHROPIC_API_KEY"} },
		ConnectFound: func(i int) (ConnectResult, error) {
			picked = i
			return ConnectResult{OK: true, ID: "openrouter", API: "openrouter", Models: 371, Tested: "x/y", Answer: "ok"}, nil
		},
	}
	m.connectCommand(nil)
	v := m.renderConnect(lipglossPlain(), lipglossPlain())
	for _, want := range []string{"Found on this machine:", "1. OpenRouter  $MY_OR_KEY", "2. Anthropic  .env: ANTHROPIC_API_KEY", "Or a provider:", "3. Sign in with OpenRouter"} {
		if !strings.Contains(v, want) {
			t.Fatalf("picker lacks %q:\n%s", want, v)
		}
	}
	_, cmd := m.connectKey(press(tea.KeyEnter))
	if cmd == nil || m.connect.step != connectStepProbing {
		t.Fatalf("enter on the first found key connects it: %+v", m.connect)
	}
	m.connectDone(cmd().(connectResultMsg))
	if last := m.messages[len(m.messages)-1].Content; picked != 0 || !strings.Contains(last, "✓ **openrouter** connected") {
		t.Fatalf("picked %d, said %q", picked, last)
	}

	m.connectCommand(nil)
	if m.connectKey(press(tea.KeyRunes, '7')); m.connect.kind.ID != "aimlapi" {
		t.Fatalf("7 is AI/ML API after two found keys: %+v", m.connect.kind)
	}
}

// A ChatGPT sign-in's ✓ says the plan answers by default now and which pin
// it moved off the billed OpenAI key (the mutation check: skip the default
// line and "answers by default now" is not drawn).
func TestAChatGPTSignInSaysThePlanAnswersAndThePinMoved(t *testing.T) {
	m := &Model{width: 120, input: textarea.New()}
	m.connectDone(connectResultMsg{req: ConnectRequest{ID: "chatgpt", API: "chatgpt"}, res: ConnectResult{
		OK: true, ID: "chatgpt", Models: 8, Tested: "gpt-6.1-sol", Answer: "ok",
		Default: "gpt-6.1-sol", Notes: []string{"openai:gpt-6-astra → chatgpt:gpt-6-astra"},
	}})
	last := m.messages[len(m.messages)-1].Content
	for _, want := range []string{"✓ **ChatGPT plan** connected", "answers by default now, on gpt-6.1-sol", "off the billed OpenAI key: openai:gpt-6-astra → chatgpt:gpt-6-astra"} {
		if !strings.Contains(last, want) {
			t.Fatalf("lacks %q:\n%s", want, last)
		}
	}
	m.connectDone(connectResultMsg{req: ConnectRequest{ID: "chatgpt", API: "chatgpt"}, res: ConnectResult{OK: true, ID: "chatgpt", Models: 8, Tested: "gpt-6.1-sol", Answer: "ok"}})
	if last := m.messages[len(m.messages)-1].Content; strings.Contains(last, "by default") || strings.Contains(last, "pin moved") {
		t.Fatalf("a company key answering: no default line, no pin line:\n%s", last)
	}
}

// The prefilled variable name is a default: a key pasted into the field takes
// its place. It was appended (2026-11-18, a first run on Linux): the field held
// OPENROUTER_API_KEYsk-or-…, drawn as dots, and OpenRouter answered 401.
func TestConnectKeyPasteReplacesThePrefilledName(t *testing.T) {
	var got ConnectRequest
	open := func() *Model {
		m := &Model{width: 100, input: textarea.New()}
		m.input.Focus()
		m.connectOps = ConnectOps{Connect: func(req ConnectRequest) (ConnectResult, error) {
			got = req
			return ConnectResult{OK: true, ID: req.ID}, nil
		}}
		m.connectCommand([]string{"openrouter"})
		if m.connect.step != connectStepBase {
			t.Fatalf("/connect openrouter asks the base: %+v", m.connect)
		}
		m.connectKey(press(tea.KeyEnter))
		if m.connect.step != connectStepKey || m.input.Value() != "OPENROUTER_API_KEY" || m.connect.keySet {
			t.Fatalf("the name prefilled, not set on the gateway: %+v %q", m.connect, m.input.Value())
		}
		return m
	}
	// type sends a key through the window's order: /connect first, then the input line.
	typeKey := func(m *Model, k tea.KeyMsg) {
		if h, _ := m.connectKey(k); !h {
			m.input, _ = m.input.Update(k)
		}
	}
	send := func(m *Model) string {
		_, cmd := m.connectKey(press(tea.KeyEnter))
		if cmd == nil {
			t.Fatalf("enter on the key sends: %+v %q", m.connect, m.input.Value())
		}
		cmd()
		return got.Key
	}

	m := open()
	typeKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sk-or-v1-pasted"), Paste: true})
	if k := send(m); k != "sk-or-v1-pasted" {
		t.Fatalf("a pasted key replaces the name: %q", k)
	}

	m = open()
	for _, r := range "sk-or-v1-typed" {
		typeKey(m, press(tea.KeyRunes, r))
	}
	if k := send(m); k != "sk-or-v1-typed" {
		t.Fatalf("a typed key replaces the name, and the letters after the first add up: %q", k)
	}

	m = open()
	if k := send(m); k != "OPENROUTER_API_KEY" {
		t.Fatalf("enter keeps the name: %q", k)
	}

	m = open()
	typeKey(m, press(tea.KeyBackspace))
	if m.input.Value() != "" {
		t.Fatalf("backspace deletes the prefilled name whole: %q", m.input.Value())
	}

	m = open()
	typeKey(m, press(tea.KeyLeft))
	typeKey(m, press(tea.KeyRunes, 'X'))
	if m.input.Value() != "OPENROUTER_API_KEXY" {
		t.Fatalf("after a cursor key the name is edited, not replaced: %q", m.input.Value())
	}
}

// The kind list keeps its columns: every hint starts where the others do, a
// hint too long for the row goes on under its own column, never back at the
// left edge, and a name too wide for the column has its hint on the next line (2026-11-18: four names were wider than the fixed 32 columns, and
// "its address" wrapped to column 0 as if it were the next row).
func TestConnectKindListKeepsItsColumns(t *testing.T) {
	number := regexp.MustCompile(`^[ >] *[0-9]+\. `)
	withHint := regexp.MustCompile(`^[ >] *[0-9]+\. \S+( \S+)*  +`)
	for _, width := range []int{120, 100} {
		m := &Model{width: width, input: textarea.New()}
		m.connectOps = ConnectOps{Connect: func(ConnectRequest) (ConnectResult, error) { return ConnectResult{}, nil }}
		m.connectCommand(nil)
		v := ansi.Strip(m.renderConnect(lipglossPlain(), lipglossPlain()))
		col, rows := -1, 0
		for _, l := range strings.Split(v, "\n")[1:] {
			l = strings.TrimRight(l, " ")
			if l == "" || strings.Contains(l, "esc cancel") {
				continue
			}
			at := len(l) - len(strings.TrimLeft(l, " "))
			if number.MatchString(l) {
				rows++
				r := withHint.FindString(l)
				if r == "" {
					continue // a name wider than the column: its hint is on the next line
				}
				at = len(r)
			}
			if col == -1 {
				col = at
			}
			if at != col {
				t.Fatalf("width %d: %q starts its hint at %d, the first row at %d:\n%s", width, l, at, col, v)
			}
		}
		if rows != len(ConnectKinds) {
			t.Fatalf("width %d: %d rows, %d kinds:\n%s", width, rows, len(ConnectKinds), v)
		}
	}
}
