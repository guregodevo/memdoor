package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	if v := m.renderConnect(lipglossPlain(), lipglossPlain()); !strings.Contains(v, "which kind") || !strings.Contains(v, "1. Sign in with OpenRouter") || !strings.Contains(v, "2. ChatGPT") || !strings.Contains(v, "3. Local runtime") || !strings.Contains(v, "4. OpenRouter") || !strings.Contains(v, "5. Anthropic") {
		t.Fatalf("picker: %q", v)
	}
	if h, _ := m.connectKey(press(tea.KeyRunes, '5')); !h || m.connect.kind.ID != "anthropic" || m.connect.step != connectStepBase || m.input.Value() != "https://api.anthropic.com" {
		t.Fatalf("5 picks Anthropic and prefills its base: %+v %q", m.connect, m.input.Value())
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
	if h, _ := m.connectKey(press(tea.KeyRunes, 'x')); h {
		t.Fatal("a letter on a text step goes to the input line")
	}
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
	if m.connectKey(press(tea.KeyRunes, '7')); m.connect.kind.ID != "anthropic" {
		t.Fatalf("7 is Anthropic after two found keys: %+v", m.connect.kind)
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
