package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The picker: search opens the models, enter opens a model's hosts,
// shift+arrows reorder, s cycles how hosts are chosen, enter pins with
// exactly that preference.
func TestModelPickerFlow(t *testing.T) {
	var pinned []string
	hostsAsked := ""
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{
		ModelCatalog: func(q string) ([]CatalogModel, error) {
			return []CatalogModel{{ID: "z-ai/glm-5.3", Name: "GLM 5.3", Context: 1310720, Band: "$$"}, {ID: "z-ai/glm-5.3-flash", Name: "GLM 5.3 Flash", Context: 1310720, Band: "$"}}, nil
		},
		ModelHosts: func(id string) ([]ModelHost, error) {
			hostsAsked = id
			return []ModelHost{{Slug: "baidu", Name: "Baidu", Tools: true, Band: "$$"}, {Slug: "morph", Name: "Morph", Tools: true, Band: "$$"}, {Slug: "together", Name: "Together", Tools: true, Band: "$$$", Excluded: true}}, nil
		},
		PinModel: func(id, sortBy, order string) (string, error) {
			pinned = []string{id, sortBy, order}
			return "pinned", nil
		},
	}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	run := func(msg tea.Msg) tea.Cmd { n, c := m.Update(msg); m = n.(Model); return c }
	cmd := m.openModelSearch("glm")
	run(cmd())
	if m.routePicker == nil || m.routePicker.mode != pickerModels || len(m.routePicker.models) != 2 {
		t.Fatalf("the search opens the models: %+v", m.routePicker)
	}
	if v := m.View(); !strings.Contains(v, "z-ai/glm-5.3-flash") || !strings.Contains(v, "enter pins (OpenRouter: its hosts)") {
		t.Fatalf("the picker is drawn:\n%s", v)
	}
	run(tea.KeyMsg{Type: tea.KeyDown})
	if cmd = run(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("enter on a model fetches its hosts")
	}
	run(cmd())
	if hostsAsked != "z-ai/glm-5.3-flash" || m.routePicker.mode != pickerHosts || len(m.routePicker.hosts) != 3 {
		t.Fatalf("hosts of the chosen model: asked=%q %+v", hostsAsked, m.routePicker)
	}
	// Drag morph above baidu; the excluded host cannot be dragged.
	run(tea.KeyMsg{Type: tea.KeyDown})
	run(tea.KeyMsg{Type: tea.KeyShiftUp})
	if m.routePicker.hosts[0].Slug != "morph" || !m.routePicker.reordered || m.routePicker.index != 0 {
		t.Fatalf("shift+up moves the host up and follows it: %+v", m.routePicker.hosts)
	}
	run(tea.KeyMsg{Type: tea.KeyDown})
	run(tea.KeyMsg{Type: tea.KeyDown})
	run(tea.KeyMsg{Type: tea.KeyShiftUp})
	if m.routePicker.hosts[2].Slug != "together" {
		t.Fatal("a host the policy never uses stays where it is")
	}
	run(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if v := m.View(); !strings.Contains(v, "your order") {
		t.Fatalf("a reordered list is 'your order':\n%s", v)
	}
	if cmd = run(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("enter pins")
	}
	run(cmd())
	if len(pinned) != 3 || pinned[0] != "z-ai/glm-5.3-flash" || pinned[1] != "throughput" || pinned[2] != "morph,baidu" {
		t.Fatalf("pinned with the picker's preference: %v", pinned)
	}
	if m.routePicker != nil {
		t.Fatal("the picker closes on pin")
	}
	// r resets; esc closes without pinning.
	run(m.openModelHosts("z-ai/glm-5.3")())
	run(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	run(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	run(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if m.routePicker.sortIdx != 0 {
		t.Fatal("r resets the preference")
	}
	pinned = nil
	run(tea.KeyMsg{Type: tea.KeyEsc})
	if m.routePicker != nil || pinned != nil {
		t.Fatal("esc closes without pinning")
	}
}

// A bare /model-search is a picker of providers (Greg, 2026-10-03: "the
// user doesn't know the provider name; it should be without inputting
// text"): enter opens a provider's models, esc there goes back, a provider
// not connected says how to connect it.
func TestProviderPickerThenAProvidersModels(t *testing.T) {
	searched := ""
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{
		ModelProviders: func() ([]ProviderSummary, error) {
			return []ProviderSummary{{ID: "anthropic", Name: "Anthropic", Models: 13}, {ID: "deepseek", Name: "DeepSeek", Connected: true, Models: 2}, {ID: "openrouter", Name: "OpenRouter", Connected: true, Active: true, Models: 380}}, nil
		},
		ModelCatalog: func(q string) ([]CatalogModel, error) {
			searched = q
			return []CatalogModel{{ID: "deepseek-flash", Provider: "deepseek", Name: "DeepSeek Flash", Context: 1048576}}, nil
		},
		Route: func(arg string) (string, error) { return "pinned " + arg, nil },
	}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	run := func(msg tea.Msg) tea.Cmd { n, c := m.Update(msg); m = n.(Model); return c }
	cmd := m.openProviderPicker()
	run(cmd())
	if m.routePicker == nil || m.routePicker.mode != pickerProviders || len(m.routePicker.providers) != 3 || m.routePicker.index != 1 {
		t.Fatalf("the providers, cursor on the first connected: %+v", m.routePicker)
	}
	if v := m.View(); !strings.Contains(v, "Your providers") || !strings.Contains(v, "2 models") || !strings.Contains(v, "answering now") {
		t.Fatalf("drawn:\n%s", v)
	}
	// Enter on a provider that is not connected says how.
	run(tea.KeyMsg{Type: tea.KeyUp})
	run(tea.KeyMsg{Type: tea.KeyEnter})
	if m.routePicker != nil || !strings.Contains(m.messages[len(m.messages)-1].Content, "/connect anthropic") {
		t.Fatalf("not connected → /connect: %+v", m.messages[len(m.messages)-1].Content)
	}
	// Enter on a connected one opens its models; enter there pins provider:id.
	run(m.openProviderPicker()())
	if cmd = run(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil || searched != "" {
		t.Fatal("enter opens the provider's models")
	}
	run(cmd())
	if searched != "deepseek" || m.routePicker == nil || m.routePicker.mode != pickerModels || !m.routePicker.fromProviders {
		t.Fatalf("the provider's whole list, from the providers picker: %q %+v", searched, m.routePicker)
	}
	if v := m.View(); !strings.Contains(v, "deepseek's models") || !strings.Contains(v, "esc back to providers") {
		t.Fatalf("drawn:\n%s", v)
	}
	if cmd = run(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("enter pins")
	}
	if r, ok := cmd().(routeResultMsg); !ok || r.summary != "pinned deepseek:deepseek-flash" {
		t.Fatalf("pinned under its provider: %+v", cmd())
	}
}
