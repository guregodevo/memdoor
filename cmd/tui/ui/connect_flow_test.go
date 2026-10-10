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
	if v := m.renderConnect(lipglossPlain(), lipglossPlain()); !strings.Contains(v, "which kind") || !strings.Contains(v, "2. ChatGPT") || !strings.Contains(v, "3. OpenRouter") || !strings.Contains(v, "4. Anthropic") {
		t.Fatalf("picker: %q", v)
	}
	if h, _ := m.connectKey(press(tea.KeyRunes, '4')); !h || m.connect.kind.ID != "anthropic" || m.connect.step != connectStepBase || m.input.Value() != "https://api.anthropic.com" {
		t.Fatalf("4 picks Anthropic and prefills its base: %+v %q", m.connect, m.input.Value())
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
