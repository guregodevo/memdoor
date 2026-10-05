package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The model picker — Memdoor's core surface (Greg, 2026-09-26: "this will
// be the core feature of Memdoor to have efficient model selection"; "very
// UX friendly for sorting and reordering"). Two screens in one overlay:
//
//   /model-search <name>   the catalogue's matches: ↑↓ move, enter opens a
//                          model's hosts, esc closes
//   /model vendor/name     that model's hosts: ↑↓ move, shift+↑↓ drag a host
//                          up or down (your order), s cycles how hosts are
//                          chosen (cheapest · fastest · lowest latency ·
//                          OpenRouter's pick), r resets, enter pins the
//                          model for this conversation, esc closes
//
// The typed forms (/model id throughput, /model id order a,b) still work;
// the picker is what a person uses.

// CatalogModel is a catalogue entry as the seat sees it, real price included
// (Greg, 2026-09-26: "show the real price").
type CatalogModel struct {
	ID       string
	Provider string // which provider lists it (registry.go); "" = the catalogue
	Name     string
	Context  int
	Band     string
	InPerM   float64 // list price per million input tokens, USD
	OutPerM  float64 // per million output tokens
}

// ProviderSummary is one provider as a bare /model-search lists them.
type ProviderSummary struct {
	ID, Name  string
	Connected bool
	Active    bool
	Models    int
}

// ModelHost is one host of a model as the seat sees it.
type ModelHost struct {
	Slug     string
	Name     string
	Quant    string
	Context  int
	Uptime   int
	Band     string
	InPerM   float64
	OutPerM  float64
	Tools    bool
	Excluded bool // never used under the policy
}

// hostSorts are the ways hosts can be chosen, in the order s cycles them:
// the value sent, and its name on screen.
var hostSorts = []struct{ value, label string }{
	{"", "cheapest"},
	{"throughput", "fastest"},
	{"latency", "lowest latency"},
	{"default", "OpenRouter's pick"},
}

const (
	pickerModels = iota
	pickerHosts
	pickerProviders // a bare /model-search: the providers, enter opens one's models
)

type routePicker struct {
	mode          int
	query         string
	providers     []ProviderSummary // pickerProviders rows
	models        []CatalogModel
	modelID       string
	hosts         []ModelHost
	original      []ModelHost // the order fetched, for r
	index         int
	sortIdx       int
	reordered     bool
	loading       string // what is being fetched, shown while it is
	fromProviders bool   // the models list was opened from the providers picker: esc goes back
}

// providersMsg delivers the providers for a bare /model-search.
type providersMsg struct {
	rows []ProviderSummary
	err  error
}

// openProviderPicker is a bare /model-search: pick a provider from a list
// (Greg, 2026-10-03: "the user doesn't know the provider name; it should
// be without inputting text"), then one of its models.
func (m *Model) openProviderPicker() tea.Cmd {
	if m.modelProviders == nil {
		m.note("Search the model catalogue: **/model-search glm** — enter pins a model for this conversation.")
		return nil
	}
	m.routePicker = &routePicker{mode: pickerProviders, loading: "Reading your providers …"}
	op := m.modelProviders
	return func() tea.Msg { rows, err := op(); return providersMsg{rows: rows, err: err} }
}

// catalogMsg and hostsMsg deliver what the ops fetched.
type catalogMsg struct {
	query  string
	models []CatalogModel
	err    error
}
type hostsMsg struct {
	id    string
	hosts []ModelHost
	err   error
}

// openModelSearch fetches the catalogue and opens the picker on it.
func (m *Model) openModelSearch(query string) tea.Cmd {
	if m.modelCatalog == nil {
		m.note("Model search isn't available in this build.")
		return nil
	}
	m.routePicker = &routePicker{mode: pickerModels, query: query, loading: "Searching models for " + query + " …"}
	op := m.modelCatalog
	return func() tea.Msg { list, err := op(query); return catalogMsg{query: query, models: list, err: err} }
}

// openModelHosts fetches a model's hosts and opens the picker on them.
func (m *Model) openModelHosts(id string) tea.Cmd {
	if m.modelHosts == nil {
		m.note("Model hosts aren't available in this build.")
		return nil
	}
	if m.routePicker == nil {
		m.routePicker = &routePicker{}
	}
	m.routePicker.mode, m.routePicker.modelID, m.routePicker.index, m.routePicker.loading = pickerHosts, id, 0, "Reading hosts for "+id+" …"
	op := m.modelHosts
	return func() tea.Msg { hosts, err := op(id); return hostsMsg{id: id, hosts: hosts, err: err} }
}

// updateRoutePicker handles the picker's messages and keys. handled is
// false when the message is not the picker's.
func (m *Model) updateRoutePicker(msg tea.Msg) (handled bool, cmd tea.Cmd) {
	p := m.routePicker
	switch t := msg.(type) {
	case catalogMsg:
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			m.routePicker = nil
			m.note(t.err.Error())
			return true, nil
		}
		if len(t.models) == 0 {
			m.routePicker = nil
			m.note(fmt.Sprintf("No model matches **%s**.", t.query))
			return true, nil
		}
		p.models, p.index = t.models, 0
		return true, nil
	case providersMsg:
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			m.routePicker = nil
			m.note(t.err.Error())
			return true, nil
		}
		p.providers, p.index = t.rows, 0
		for i, r := range t.rows { // the cursor starts on the first connected one
			if r.Connected {
				p.index = i
				break
			}
		}
		return true, nil
	case hostsMsg:
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			m.routePicker = nil
			m.note(t.err.Error())
			return true, nil
		}
		if len(t.hosts) == 0 {
			// NO HOSTS IS AN ANSWER, NOT A BLANK PICKER (battle test,
			// 2026-09-27). `/model not-a-vendor/not-a-model` opened an empty
			// picker over nothing: no message, the old pin quietly kept, and
			// no way to tell whether it had worked.
			m.routePicker = nil
			m.note(fmt.Sprintf("No model **%s** with hosts that can serve a turn — `/model-search <text>` finds one, `/model auto` goes back to the ladder.", t.id))
			return true, nil
		}
		p.hosts, p.original, p.index, p.sortIdx, p.reordered = t.hosts, append([]ModelHost(nil), t.hosts...), 0, 0, false
		return true, nil
	case tea.KeyMsg:
		if p == nil {
			return false, nil
		}
		return true, m.routePickerKey(t)
	}
	return false, nil
}

func (m *Model) routePickerKey(k tea.KeyMsg) tea.Cmd {
	p := m.routePicker
	n := len(p.models)
	switch p.mode {
	case pickerHosts:
		n = len(p.hosts)
	case pickerProviders:
		n = len(p.providers)
	}
	switch k.Type {
	case tea.KeyEsc:
		if p.mode == pickerModels && p.query != "" && m.modelProviders != nil && p.fromProviders {
			return m.openProviderPicker() // back up a level
		}
		m.routePicker = nil
		return nil
	case tea.KeyUp:
		if p.index > 0 {
			p.index--
		}
		return nil
	case tea.KeyDown:
		if p.index < n-1 {
			p.index++
		}
		return nil
	case tea.KeyShiftUp, tea.KeyShiftDown:
		if p.mode != pickerHosts || n < 2 {
			return nil
		}
		j := p.index - 1
		if k.Type == tea.KeyShiftDown {
			j = p.index + 1
		}
		if j < 0 || j >= n || p.hosts[p.index].Excluded || p.hosts[j].Excluded {
			return nil
		}
		p.hosts[p.index], p.hosts[j] = p.hosts[j], p.hosts[p.index]
		p.index, p.reordered = j, true
		return nil
	case tea.KeyEnter:
		if p.loading != "" || n == 0 {
			return nil
		}
		if p.mode == pickerProviders {
			r := p.providers[p.index]
			if !r.Connected {
				m.routePicker = nil
				m.note(fmt.Sprintf("**%s** is not connected — /connect %s adds it (the key's name is prefilled).", r.Name, r.ID))
				return nil
			}
			cmd := m.openModelSearch(r.ID)
			if m.routePicker != nil {
				m.routePicker.fromProviders = true
				m.routePicker.loading = "Reading " + r.Name + "'s models …"
			}
			return cmd
		}
		if p.mode == pickerModels {
			c := p.models[p.index]
			if c.Provider != "" && c.Provider != "openrouter" {
				// Another provider's model: hosts are OpenRouter's notion;
				// enter pins it under its provider (provider:id).
				m.routePicker = nil
				if m.route == nil {
					return nil
				}
				id := c.Provider + ":" + c.ID
				m.note(fmt.Sprintf("Pinning **%s** …", id))
				op := m.route
				return func() tea.Msg { s, e := op(id); return routeResultMsg{summary: s, err: e} }
			}
			return m.openModelHosts(c.ID)
		}
		return m.pinFromPicker()
	case tea.KeyRunes:
		switch string(k.Runes) {
		case "s", "S":
			if p.mode == pickerHosts {
				p.sortIdx = (p.sortIdx + 1) % len(hostSorts)
			}
		case "r", "R":
			if p.mode == pickerHosts {
				p.hosts, p.sortIdx, p.reordered, p.index = append([]ModelHost(nil), p.original...), 0, false, 0
			}
		}
	}
	return nil
}

// pinFromPicker pins the model with the picker's preference: the sort
// chosen, and the person's order when they dragged hosts.
func (m *Model) pinFromPicker() tea.Cmd {
	p := m.routePicker
	if m.pinModel == nil {
		m.routePicker = nil
		m.note("Pinning a model isn't available in this build.")
		return nil
	}
	order := ""
	if p.reordered {
		var slugs []string
		for _, h := range p.hosts {
			if !h.Excluded && h.Tools {
				slugs = append(slugs, h.Slug)
			}
		}
		order = strings.Join(slugs, ",")
	}
	id, sortBy := p.modelID, hostSorts[p.sortIdx].value
	m.routePicker = nil
	m.answeredBy = ""
	op := m.pinModel
	return func() tea.Msg { s, e := op(id, sortBy, order); return routeResultMsg{summary: s, err: e} }
}

// renderRoutePicker draws the overlay.
func (m Model) renderRoutePicker() string {
	p := m.routePicker
	if p == nil {
		return ""
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colText)).PaddingLeft(2)
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG)).Bold(true)
	norm := lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	keys := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).PaddingLeft(2)
	var b strings.Builder
	if p.loading != "" {
		b.WriteString(title.Render(p.loading))
		return b.String()
	}
	maxRows := m.height - 12
	if maxRows > 12 {
		maxRows = 12
	}
	if maxRows < 3 {
		maxRows = 3
	}
	var rows []string
	var n int
	if p.mode == pickerProviders {
		b.WriteString(title.Render("Your providers — enter opens one's models; a model's enter pins it for this conversation") + "\n")
		n = len(p.providers)
		for _, r := range p.providers {
			state := fmt.Sprintf("%4d models", r.Models)
			if !r.Connected {
				state = "not connected · enter says how"
			} else if r.Active {
				state += " · answering now"
			}
			rows = append(rows, fmt.Sprintf("%-11s %-22s %s", r.ID, r.Name, state))
		}
	} else if p.mode == pickerModels {
		heading := fmt.Sprintf("Models matching %q", p.query)
		if p.fromProviders {
			heading = fmt.Sprintf("%s's models", p.query)
		}
		b.WriteString(title.Render(heading+" — provider · id · context · $ per million in / out — enter pins (OpenRouter: its hosts)") + "\n")
		n = len(p.models)
		for _, c := range p.models {
			prov := c.Provider
			if prov == "" {
				prov = "openrouter"
			}
			rows = append(rows, fmt.Sprintf("%-10s %-40s %5dk  %s  %s", prov, c.ID, c.Context/1000, priceCell(c.InPerM, c.OutPerM), c.Name))
		}
	} else {
		how := hostSorts[p.sortIdx].label
		if p.reordered {
			how = "your order"
		}
		b.WriteString(title.Render(fmt.Sprintf("Hosts for %s — chosen by %s — $ per million tokens in / out", p.modelID, how)) + "\n")
		n = len(p.hosts)
		for i, h := range p.hosts {
			q := h.Quant
			if q == "" || q == "unknown" {
				q = "—"
			}
			row := fmt.Sprintf("%2d. %-22s %-6s %5dk  %3d%%  %s  %s", i+1, h.Slug, q, h.Context/1000, h.Uptime, priceCell(h.InPerM, h.OutPerM), h.Name)
			if h.Excluded {
				row += "  (never used: policy)"
			} else if !h.Tools {
				row += "  (no tool calls)"
			}
			rows = append(rows, row)
		}
	}
	start := 0
	if n > maxRows {
		start = p.index - maxRows/2
		if start < 0 {
			start = 0
		}
		if start > n-maxRows {
			start = n - maxRows
		}
	}
	end := start + maxRows
	if end > n {
		end = n
	}
	for i := start; i < end; i++ {
		line := rows[i]
		switch {
		case i == p.index:
			line = sel.Render("→ " + line)
		case p.mode == pickerHosts && p.hosts[i].Excluded, p.mode == pickerProviders && !p.providers[i].Connected:
			line = dim.Render("  " + line)
		default:
			line = norm.Render("  " + line)
		}
		b.WriteString("  " + line + "\n")
	}
	if end < n {
		b.WriteString(dim.Render(fmt.Sprintf("  … %d more", n-end)) + "\n")
	}
	if p.mode == pickerProviders {
		b.WriteString(keys.Render("↑↓ move · enter open · esc close"))
	} else if p.mode == pickerModels {
		back := "esc close"
		if p.fromProviders {
			back = "esc back to providers"
		}
		b.WriteString(keys.Render("↑↓ move · enter pins (OpenRouter: its hosts) · " + back))
	} else {
		b.WriteString(keys.Render("↑↓ move · shift+↑↓ reorder · s how hosts are chosen · r reset · enter pin for this conversation · esc close"))
	}
	return b.String()
}

// priceCell is "$0.04 / $0.50" padded to a column: the real list price per
// million tokens, in and out.
func priceCell(in, out float64) string {
	return fmt.Sprintf("%7s / %-7s", "$"+trimZero(in), "$"+trimZero(out))
}

func trimZero(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	return s
}
