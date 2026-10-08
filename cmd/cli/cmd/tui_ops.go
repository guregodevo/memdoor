package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/pkg/savings"
	"memdoor/pkg/shared"
)

// Network-effect operations the TUI exposes as /clone and /publish. They live
// in package cmd so they reuse the same authed Client + clone/publish flow the
// CLI uses (no duplication), and return a result string + new slug the TUI
// renders as a chat message instead of printing to stdout.

func tuiPageStatus(launchDir, workspace, channelID, agent string) ui.Status {
	st := ui.Status{Branch: gitBranch(launchDir)}
	if workspace != "" && channelID != "" && agent != "" {
		if rv, err := readRoute(workspace, channelID, agent); err == nil {
			st.Rung, st.Rungs, st.Reason, st.Pinned, st.Effort = rv.Rung, len(rv.Rungs), rv.Reason, rv.Pinned, rv.Effort
			st.EffortUsed, st.EffortFrom = rv.EffortUsed, rv.EffortFrom
			st.Provider = rv.Provider
			if rv.Model != "" {
				st.AgentModels = map[string]string{agent: rv.Model}
			}
		}
	}

	// What is thinking: the brain the gateway reports, else the local pin.
	c := NewClient()
	r := readBrain(c)
	st.Gate = r.Gate
	st.Model = r.Model
	if st.AgentModels == nil {
		st.AgentModels = r.AgentModels
	}
	return st
}

// gitBranch is the current branch of dir, or a short SHA when HEAD is detached.
// Empty when dir isn't in a git repo. Reads .git directly: a subprocess per
// footer tick is a poor trade for one line of text.
func gitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	for d := dir; ; {
		head := filepath.Join(d, ".git", "HEAD")
		if b, err := os.ReadFile(head); err == nil {
			line := strings.TrimSpace(string(b))
			if ref := strings.TrimPrefix(line, "ref: refs/heads/"); ref != line {
				return ref
			}
			if len(line) >= 7 {
				return line[:7] + " (detached)"
			}
			return ""
		}
		// A worktree or submodule has a .git FILE pointing elsewhere.
		if b, err := os.ReadFile(filepath.Join(d, ".git")); err == nil {
			if p := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:")); p != "" {
				if hb, err := os.ReadFile(filepath.Join(p, "HEAD")); err == nil {
					line := strings.TrimSpace(string(hb))
					if ref := strings.TrimPrefix(line, "ref: refs/heads/"); ref != line {
						return ref
					}
				}
			}
			return ""
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// brainRead is the one status call the footer, the gate and `account
// status` make: what the app should SAY about the brain (creator), and whether a turn can run on it.
type brainRead struct {
	// Model is the model that answers an unnamed request; AgentModels which
	// answers for which agent.
	Model       string            `json:"model,omitempty"`
	AgentModels map[string]string `json:"agent_models,omitempty"`
	State       string            `json:"state,omitempty"` // ready | none
	Gate        string            `json:"gate,omitempty"`  // why a turn cannot run now; empty = it can
}

// readBrain reads the gateway once. One request, several answers — the
// footer redraws on a timer and a second call would double that traffic
// for one string.
func readBrain(c *Client) brainRead {
	var st struct {
		Model       string            `json:"model"`
		AgentModels map[string]string `json:"agent_models"`
		Brain       struct {
			State string `json:"state"`
			Note  string `json:"note"`
		} `json:"brain"`
	}
	if err := c.GetJSON("/api/llm/burst", &st); err != nil {
		return brainRead{}
	}
	r := brainRead{State: st.Brain.State}
	r.Model, r.AgentModels = st.Model, st.AgentModels
	r.Gate = brainGate(st.Brain.State)
	return r
}

// brainGate says why a turn cannot run yet, or nothing.
//
// "We should block user until brain is on" (Greg, 2026-09-18): a turn sent
// where no model can answer fails at once and reads as broken software, so
// the gate names the step that connects one — the person's own key, the
// only way to a model (2026-10-04).
func brainGate(state string) string {
	switch state {
	case "none":
		return "no model yet — type /connect and paste a provider's key, or quit, export OPEN_ROUTER_API_KEY=… (or ANTHROPIC_API_KEY, …) and run memdoor tui again — every model runs on your own key"
	}
	return ""
}

// tuiUsage answers /usage: this month's turns from the local meter and what
// the decision model kept out of them (pkg/savings). Greg, 2026-09-27:
// "let's focus on cheaper by efficiency" — this is where a person looks for
// the number that justifies the ten dollars. Only the header differs between
// a seat and the person's own key; tokens, never the seat's money.
func tuiUsage() (string, error) {
	// THE KEY IS ANY PROVIDER'S (Greg, 2026-10-03: "stop with openrouter"):
	// the header names the provider that answers, as `memdoor providers`
	// marks it, never one vendor by default.
	own := ownKeyLabel()
	header := "**" + own + "**"
	// A seat never supplies a key: the plan only names itself.
	if plan := seatPlanLabel(); plan != "" {
		header = "**" + plan + "** · " + strings.ToLower(own[:1]) + own[1:]
	}
	decisions := gatewayDecisionsOn(NewClient())
	if decisions {
		header += " · the decision model is on"
	}
	s, err := savings.ReadMonth(time.Now())
	if err != nil {
		return "", err
	}
	return usageView(header, decisions, readMeterMonth(time.Now()), s), nil
}

// seatPlanLabel is the seat's plan as the broker names it, or "" when no
// seat answers.
func seatPlanLabel() string {
	var me meAnswer
	if brokerCall("GET", "/v1/me", nil, &me) != nil {
		return ""
	}
	return me.Plan
}

// usageView renders a month: the turns and models, then the receipt.
// decisions says a decision model runs on this gateway.
func usageView(header string, decisions bool, m meterMonth, s savings.Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · %s\n\n", header, s.Month)
	if m.Requests == 0 {
		b.WriteString("No model requests metered yet this month.\n")
	} else {
		fmt.Fprintf(&b, "%d model request%s · %s in · %s out", m.Requests, plural(m.Requests), humanTokens(m.In), humanTokens(m.Out))
		// Some ledger lines say how much of the prompt the engine served
		// from its prefix cache (CachedTokens, 0 when not reported). Only
		// count the lines that report it, so engines that stay silent don't
		// read as a total miss.
		if m.Cached > 0 {
			fmt.Fprintf(&b, " · %s of the input cached", humanTokens(m.Cached))
		}
		// Only some ledger lines carry a price (OpenRouter prices its own),
		// so say how many rather than implying the figure covers everything.
		if m.Cost > 0 {
			fmt.Fprintf(&b, " · %s priced on %d of them", money(m.Cost), m.Priced)
		}
		b.WriteString("\n")
		for i, mm := range m.ByModel {
			if i == 5 {
				// The rows cut off still carry requests and tokens; say how
				// much, or the month looks smaller than the header says.
				fmt.Fprintf(&b, "- and %d more model%s · %d request%s · %s in\n",
					len(m.ByModel)-5, plural(len(m.ByModel)-5),
					m.RemainingRequests, plural(m.RemainingRequests), humanTokens(m.RemainingIn))
				break
			}
			fmt.Fprintf(&b, "- **%s** · %d request%s · %s in · %s out", mm.model, mm.requests, plural(mm.requests), humanTokens(mm.in), humanTokens(mm.out))
			// A price on a partly priced row would read as the model's
			// whole cost, which it is not: say which requests carry it.
			if mm.cost > 0 {
				if mm.priced == mm.requests {
					fmt.Fprintf(&b, " · %s", money(mm.cost))
				} else {
					fmt.Fprintf(&b, " · %s on %d priced", money(mm.cost), mm.priced)
				}
			}
			b.WriteString("\n")
		}
	}
	if t := s.TokensSaved(); t > 0 {
		fmt.Fprintf(&b, "\nThe decision model kept **≈%s input tokens** out of those requests: %s of judged reads and %s of tool schemas.",
			humanCount(t), humanBytes(s.JudgedRaw-s.JudgedKept), humanBytes(s.ToolboxSaved))
		if s.EarlyStops > 0 {
			fmt.Fprintf(&b, " %d turn%s ended for want of progress.", s.EarlyStops, plural(s.EarlyStops))
		}
		b.WriteString("\n`memdoor savings` prices it.\n")
	} else {
		if decisions {
			b.WriteString("\nThe decision model has kept nothing out yet this month: it works through jgrep, jread and the per-turn toolbox.\n")
		} else {
			b.WriteString("\nThe decision model is off: `memdoor connect typesafe` turns it on with any provider (or an OpenRouter key).\n")
		}
	}
	return b.String()
}

type meterModel struct {
	model    string
	requests int
	priced   int // requests whose line carried a price on this key
	in       int64
	out      int64
	cost     float64 // what this key paid for them, when priced
}

// meterMonth is a month of ~/.memdoor/meter.jsonl: one line per model
// request, and a turn makes many (one session measured 57 lines in five
// turns), so the count is requests, never turns.
type meterMonth struct {
	Requests int
	Priced   int // requests on the person's own key whose line carried a cost
	In, Out  int64
	Cached   int64 // input tokens reported served from cache, summed over lines that say
	Cost     float64
	// What the rows past the top five carry, so the truncation line can be
	// honest about what it is hiding.
	RemainingRequests int
	RemainingIn       int64
	ByModel           []meterModel
}

// readMeterMonth sums the meter for the month containing when. Model names of
// one or two characters are test fixtures from this repo's own history, not
// models; they are left out rather than shown to someone as a row.
func readMeterMonth(when time.Time) meterMonth {
	var out meterMonth
	raw, err := os.ReadFile(shared.MemdoorHome("meter.jsonl"))
	if err != nil {
		return out
	}
	month := when.Format("2006-01")
	idx := map[string]int{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e struct {
			TS      string  `json:"ts"`
			Model   string  `json:"model"`
			In      int64   `json:"input_tokens"`
			Cached  int64   `json:"cached_tokens"`
			Out     int64   `json:"output_tokens"`
			CostUSD float64 `json:"cost_usd"`
			Engine  string  `json:"engine"`
		}
		if json.Unmarshal([]byte(line), &e) != nil || !strings.HasPrefix(e.TS, month) {
			continue
		}
		// Fixture lines are not requests and belong in no sum, not even the
		// header's count.
		if len(e.Model) < 3 {
			continue
		}
		out.Requests++
		out.In += e.In
		out.Out += e.Out
		if e.Cached > 0 {
			out.Cached += e.Cached
		}
		// Only a line their own key served is theirs to see priced; the
		// broker's lines are the seat's bill.
		if e.CostUSD > 0 && strings.Contains(e.Engine, "openrouter.ai") {
			out.Cost += e.CostUSD
			out.Priced++
		}
		// One model, one row: a name with and without its vendor prefix
		// ("deepseek-v4-flash", "deepseek/…") is one model; the row shows
		// the full name once it has been seen.
		key := strings.ToLower(e.Model[strings.LastIndex(e.Model, "/")+1:])
		if i, ok := idx[key]; ok {
			out.ByModel[i].requests++
			out.ByModel[i].in += e.In
			out.ByModel[i].out += e.Out
			if e.CostUSD > 0 && strings.Contains(e.Engine, "openrouter.ai") {
				out.ByModel[i].cost += e.CostUSD
				out.ByModel[i].priced++
			}
			if strings.Contains(e.Model, "/") {
				out.ByModel[i].model = e.Model
			}
			continue
		}
		idx[key] = len(out.ByModel)
		own := e.CostUSD > 0 && strings.Contains(e.Engine, "openrouter.ai")
		row := meterModel{model: e.Model, requests: 1, in: e.In, out: e.Out}
		if own {
			row.priced = 1
		}
		out.ByModel = append(out.ByModel, row)
	}
	sort.Slice(out.ByModel, func(i, j int) bool { return out.ByModel[i].in > out.ByModel[j].in })
	for i, mm := range out.ByModel {
		if i < 5 {
			continue
		}
		out.RemainingRequests += mm.requests
		out.RemainingIn += mm.in
	}
	return out
}

// humanTokens renders a token count the way people say it: 1.2k, 18.7M.
func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

// routeView mirrors gateway/route_handler.go.
type routeView struct {
	Agent         string   `json:"agent"`
	Rung          int      `json:"rung"`
	Rungs         []string `json:"rungs"`
	RungProviders []string `json:"rung_providers"`
	Model         string   `json:"model"`
	Provider      string   `json:"provider"`
	Reason        string   `json:"reason"`
	Pinned        bool     `json:"pinned"`
	Custom        bool     `json:"custom"` // pinned by id, off the ladder
	Sort          string   `json:"sort,omitempty"`
	Order         string   `json:"order,omitempty"`
	Effort        string   `json:"effort"` // chosen with Shift+Tab; "" = auto
	EffortUsed    string   `json:"effort_used"`
	EffortFrom    string   `json:"effort_from"`
}

func readRoute(workspace, channelID, agent string) (routeView, error) {
	var rv routeView
	err := NewClient().GetJSON("/api/route?workspace="+url.QueryEscape(workspace)+"&channel_id="+url.QueryEscape(channelID)+"&agent="+url.QueryEscape(agent), &rv)
	return rv, err
}

// tuiRoute is /model for one page: no argument shows the ladder with this
// conversation's rung marked; a number pins that rung; "auto" lets go.
// tuiEffort is Shift+Tab: the conversation's reasoning effort (auto hands it
// back to the decision model, else high — gateway/turn_effort.go).
func tuiEffort(workspace, channelID, agent string) func(level string) (string, error) {
	return func(level string) (string, error) {
		if level == "" {
			level = "auto"
		}
		var rv routeView
		err := NewClient().PostJSON("/api/route", map[string]interface{}{"workspace": workspace, "channel_id": channelID, "agent": agent, "effort": level}, &rv)
		return rv.Effort, err
	}
}

func tuiRoute(workspace, channelID, agent string) func(arg string) (string, error) {
	return func(arg string) (string, error) {
		var rv routeView
		var warn string
		// Keywords (auto, price, order …) read lower-cased; a model id keeps
		// its case — Baseten's ids carry it (zai-org/GLM-5.3-Flash), and a
		// lower-cased pin was refused by its host (live 2026-10-02).
		raw := strings.TrimSpace(arg)
		arg = strings.ToLower(raw)
		switch {
		case arg == "":
			var err error
			if rv, err = readRoute(workspace, channelID, agent); err != nil {
				return "", err
			}
		case looksLikeModelID(arg):
			// A model by id, off the ladder (as /model-search lists them),
			// with the person's host preference: "/model z-ai/glm-5.3
			// throughput", "/model z-ai/glm-5.3 order baidu,morph".
			words := strings.Fields(arg)
			words[0] = strings.Fields(raw)[0] // the id as typed
			body := map[string]interface{}{"workspace": workspace, "channel_id": channelID, "agent": agent, "model": words[0]}
			for i := 1; i < len(words); i++ {
				switch w := words[i]; {
				case w == "price" || w == "throughput" || w == "latency" || w == "default":
					body["sort"] = w
				case w == "order" && i+1 < len(words):
					i++
					body["order"] = words[i]
				case strings.HasPrefix(w, "order:"):
					body["order"] = strings.TrimPrefix(w, "order:")
				default:
					return "", fmt.Errorf("after the model id: price | throughput | latency | default, or order host1,host2 (got %q)", w)
				}
			}
			// A PIN THAT CANNOT SERVE MUST SAY SO (battle test, 2026-09-27).
			// `/model not-a-vendor/not-a-model` used to echo and do nothing:
			// no message, the old pin silently kept, and no way to tell
			// whether it had worked. Check the catalogue first, and fail open
			// when the catalogue itself cannot be reached — the person may
			// know about a model our cache does not.
			if canonical, err := modelInCatalogue(bareModelID(words[0])); err != nil {
				return "", err
			} else if canonical != "" && canonical != bareModelID(words[0]) {
				// The catalogue's own spelling of the id (case included).
				words[0] = strings.TrimSuffix(words[0], bareModelID(words[0])) + canonical
				body["model"] = words[0]
			}
			if err := NewClient().PostJSON("/api/route", body, &rv); err != nil {
				return "", err
			}
			// What the stored probe knows about the model just pinned, said
			// before the turn rather than discovered during it
			// (model_pin_warning.go).
			warn = pinWarning(words[0])
		default:
			var tier interface{} = arg
			n, numErr := strconv.Atoi(arg)
			if numErr == nil {
				tier = n
			} else if arg != "auto" {
				return "", fmt.Errorf("/model takes a rung number, a model id (vendor/name) or auto")
			}
			// A PINNED MODEL IS 1 (Greg, 2026-10-03: "why i dont see it as
			// 1"): the list shows it first and the ladder after it, so the
			// numbers typed are the numbers shown — 1 keeps the pin, n>1 is
			// the ladder's rung n-1.
			if numErr == nil {
				if cur, err := readRoute(workspace, channelID, agent); err == nil && cur.Custom {
					if n == 1 {
						rv = cur
						break
					}
					tier = n - 1
				}
			}
			if err := NewClient().PostJSON("/api/route", map[string]interface{}{"workspace": workspace, "channel_id": channelID, "agent": agent, "tier": tier}, &rv); err != nil {
				return "", err
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Models for **%s** — a pin stays for every new conversation until /model auto:\n", agent)
		offset := 0
		if rv.Custom {
			// The pinned model is 1; the ladder follows.
			offset = 1
			pref := ""
			if rv.Sort != "" {
				pref += " · hosts by " + rv.Sort
			}
			if rv.Order != "" {
				pref += " · host order " + rv.Order
			}
			shown := rv.Model
			if rv.Provider != "" {
				shown = rv.Provider + " · " + rv.Model
			}
			fmt.Fprintf(&b, "→ 1  %s   ← %s%s\n", shown, rv.Reason, pref)
		}
		// Each rung names the provider that serves it, as the gateway states
		// per rung (Greg, 2026-10-03: rung 2 is "deepseek/…" served by
		// OpenRouter): "2  openrouter · deepseek/deepseek-v4.1-flash".
		for i, m := range rv.Rungs {
			mark := "  "
			current := !rv.Custom && i+1 == rv.Rung
			if current {
				mark = "→ "
			}
			if i < len(rv.RungProviders) && rv.RungProviders[i] != "" {
				m = rv.RungProviders[i] + " · " + m
			}
			fmt.Fprintf(&b, "%s%d  %s", mark, i+1+offset, m)
			if current {
				fmt.Fprintf(&b, "   ← %s", rv.Reason)
			}
			b.WriteString("\n")
		}
		if warn != "" {
			b.WriteString(warn + "\n")
		}
		if len(rv.Rungs) > 1 || rv.Custom {
			b.WriteString("/model <n> pins a rung · /model <vendor/name> [price|throughput|latency|default] [order host1,host2] pins any model (/model-search finds one, /model-search <vendor/name> lists its hosts) · /model auto lets it go back to the first.")
		} else {
			b.WriteString("This agent has one rung; /model <vendor/name> pins another model (/model-search finds one).")
		}
		// The other providers (Greg, 2026-10-03, after /connect xai showed
		// nothing here): what is connected and how to pin one of its models,
		// and what is not and how to connect it.
		if rows := listProviders(); len(rows) > 0 {
			var connected, locked []string
			for _, p := range rows {
				switch {
				case p.Active, p.API == "decisions": // a decision provider has no model to pin
				case p.Connected:
					connected = append(connected, p.ID)
				default:
					locked = append(locked, p.ID)
				}
			}
			if len(connected) > 0 {
				fmt.Fprintf(&b, "\nAlso connected: %s — /model-search <name> finds their models; /model <provider>:<id> pins one (e.g. /model %s:…).", strings.Join(connected, ", "), connected[0])
			}
			if len(locked) > 0 {
				fmt.Fprintf(&b, "\nNot connected: %s — /connect <kind> adds one.", strings.Join(locked, ", "))
			}
		}
		return b.String(), nil
	}
}

// catalogModel mirrors billingsvc.catalogModel: what the seat may see.
type catalogModel struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	Name     string  `json:"name"`
	Context  int     `json:"context"`
	Band     string  `json:"band"`
	InPerM   float64 `json:"in_per_m"`
	OutPerM  float64 `json:"out_per_m"`
}

// searchModelCatalog asks the gateway for the catalogue (it reads the
// broker's with its own seat, gateway/models_handler.go).
func searchModelCatalog(query string) ([]catalogModel, error) {
	var res struct {
		Models []catalogModel `json:"models"`
	}
	if err := NewClient().GetJSON("/api/models?q="+url.QueryEscape(query), &res); err != nil {
		return nil, err
	}
	return res.Models, nil
}

// renderModelCatalog is the search result as a note or a terminal listing.
func renderModelCatalog(query string, models []catalogModel) string {
	if len(models) == 0 {
		if hint := connectHint(query, listProviders()); hint != "" {
			return fmt.Sprintf("No model matches **%s**. %s", query, hint)
		}
		return fmt.Sprintf("No model matches **%s**.", query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Models matching **%s** (%d) — provider · id · context · $ per million tokens in / out:\n", query, len(models))
	for _, m := range models {
		prov := m.Provider
		if prov == "" {
			prov = "openrouter"
		}
		fmt.Fprintf(&b, "  %-10s %-40s %5dk  $%.2f / $%.2f  %s\n", prov, m.ID, m.Context/1000, m.InPerM, m.OutPerM, m.Name)
	}
	b.WriteString("/model <id> pins one for this conversation; /model <provider>:<id> when two providers list it.")
	return b.String()
}

// providerView mirrors billingsvc.providerView: a model's host as the seat sees it.
type providerView struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Quant    string  `json:"quant"`
	Context  int     `json:"context"`
	Uptime   int     `json:"uptime_pct"`
	Band     string  `json:"band"`
	InPerM   float64 `json:"in_per_m"`
	OutPerM  float64 `json:"out_per_m"`
	Tools    bool    `json:"tools"`
	Excluded bool    `json:"excluded"`
}

// modelProviders lists a model's hosts through the gateway.
func modelProviders(id string) ([]providerView, error) {
	var res struct {
		Providers []providerView `json:"providers"`
	}
	if err := NewClient().GetJSON("/api/models/providers?id="+url.QueryEscape(id), &res); err != nil {
		return nil, err
	}
	return res.Providers, nil
}

// renderModelProviders is the hosts listing: cheapest band first, the
// slug the person can put in an order.
func renderModelProviders(id string, hosts []providerView) string {
	if len(hosts) == 0 {
		return fmt.Sprintf("No host serves **%s** under your provider policy.", id)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hosts for **%s** (%d) — slug · precision · context · uptime · $ per million tokens in / out:\n", id, len(hosts))
	for _, h := range hosts {
		q := h.Quant
		if q == "" || q == "unknown" {
			q = "—"
		}
		note := ""
		if h.Excluded {
			note = "  (never used: policy)"
		} else if !h.Tools {
			note = "  (no tool calls)"
		}
		fmt.Fprintf(&b, "  %-26s %-6s %5dk  %3d%%  $%.2f / $%.2f  %s%s\n", h.Slug, q, h.Context/1000, h.Uptime, h.InPerM, h.OutPerM, h.Name, note)
	}
	fmt.Fprintf(&b, "/model %s price|throughput|latency|default picks how hosts are chosen · /model %s order slug1,slug2 sets your own order.", id, id)
	return b.String()
}

// tuiModelCatalog and tuiModelHosts feed the picker (ui/route_picker.go).
func tuiModelCatalog(q string) ([]ui.CatalogModel, error) {
	models, err := searchModelCatalog(q)
	if err != nil {
		return nil, err
	}
	out := make([]ui.CatalogModel, 0, len(models))
	for _, m := range models {
		out = append(out, ui.CatalogModel{ID: m.ID, Provider: m.Provider, Name: m.Name, Context: m.Context, Band: m.Band, InPerM: m.InPerM, OutPerM: m.OutPerM})
	}
	return out, nil
}

// tuiModelProviders is a bare /model-search: every provider with how many
// models it lists (the grouped /api/models, no query).
func tuiModelProviders() ([]ui.ProviderSummary, error) {
	var res struct {
		Providers []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			API       string `json:"api"`
			Connected bool   `json:"connected"`
			Active    bool   `json:"active"`
			Count     int    `json:"count"`
		} `json:"providers"`
	}
	if err := NewClient().GetJSON("/api/models?limit=0", &res); err != nil {
		return nil, err
	}
	out := make([]ui.ProviderSummary, 0, len(res.Providers))
	for _, p := range res.Providers {
		if p.API == "decisions" {
			continue // it judges; nothing in it to pin
		}
		out = append(out, ui.ProviderSummary{ID: p.ID, Name: p.Name, Connected: p.Connected, Active: p.Active, Models: p.Count})
	}
	return out, nil
}

func tuiModelHosts(id string) ([]ui.ModelHost, error) {
	hosts, err := modelProviders(id)
	if err != nil {
		// SAY IT IN THE PERSON'S TERMS (battle test, 2026-09-27). A mistyped
		// id came back as `API error (502): {"error": "the host listing
		// answered HTTP 404"}`, which is our plumbing, not their problem.
		if unknownModel(err) {
			return nil, fmt.Errorf("no model %q — `/model-search <text>` finds one, `/model auto` goes back to the ladder", id)
		}
		return nil, err
	}
	out := make([]ui.ModelHost, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, ui.ModelHost{Slug: h.Slug, Name: h.Name, Quant: h.Quant, Context: h.Context, Uptime: h.Uptime, Band: h.Band, InPerM: h.InPerM, OutPerM: h.OutPerM, Tools: h.Tools, Excluded: h.Excluded})
	}
	return out, nil
}

// tuiPinModel pins a model with the picker's preference and answers with
// the ladder note.
func tuiPinModel(workspace, channelID, agent string) func(id, sortBy, order string) (string, error) {
	route := tuiRoute(workspace, channelID, agent)
	return func(id, sortBy, order string) (string, error) {
		arg := id
		if sortBy != "" {
			arg += " " + sortBy
		}
		if order != "" {
			arg += " order " + order
		}
		return route(arg)
	}
}

// modelInCatalogue refuses an id the catalogue does not list, and says how to
// find one. An unreachable catalogue is not a refusal: the pin goes ahead.
// looksLikeModelID: anything that is not a rung number or "auto" — vendor/
// model as OpenRouter names them, or a provider's own id such as
// claude-haiku-4-5-20251001 (any connected provider's list is pinnable).
func looksLikeModelID(arg string) bool {
	first := strings.Fields(arg)
	if len(first) == 0 || first[0] == "auto" {
		return false
	}
	_, err := strconv.Atoi(first[0])
	return err != nil
}

// bareModelID drops a "provider:" prefix ("groq:qwen/qwen3.8-27b" — the
// explicit form for an id two providers list) for the catalogue check.
func bareModelID(id string) string {
	if i := strings.Index(id, ":"); i > 0 && !strings.Contains(id[:i], "/") {
		return id[i+1:]
	}
	return id
}

// modelInCatalogue is the catalogue's own spelling of id ("" when there is
// no catalogue to ask), or why no provider lists it.
// modelFamilies is which provider an id's prefix belongs to, for the hint.
var modelFamilies = []struct{ prefix, provider string }{
	{"claude", "anthropic"}, {"gpt", "openai"}, {"o1", "openai"}, {"o3", "openai"}, {"o4", "openai"},
	{"gemini", "gemini"}, {"grok", "xai"}, {"deepseek", "deepseek"},
}

// connectHint says what to connect: the provider an id's family belongs
// to when it is not connected, else the providers that are not.
func connectHint(id string, providers []providerRow) string {
	byID := map[string]providerRow{}
	var locked []string
	for _, p := range providers {
		byID[p.ID] = p
		if !p.Connected && p.ID != "openrouter" {
			locked = append(locked, p.ID)
		}
	}
	low := strings.ToLower(id)
	if i := strings.LastIndex(low, ":"); i > 0 && !strings.Contains(low[:i], "/") {
		if p, ok := byID[low[:i]]; ok && !p.Connected {
			return fmt.Sprintf("%s is not connected — /connect %s adds it.", p.Name, p.ID)
		}
	}
	for _, f := range modelFamilies {
		if strings.HasPrefix(low, f.prefix) || strings.HasPrefix(low, f.provider+"/") {
			if p, ok := byID[f.provider]; ok && !p.Connected {
				return fmt.Sprintf("That looks like a %s model, and %s is not connected — /connect %s adds it (the key's name is prefilled).", p.Name, p.Name, p.ID)
			}
		}
	}
	if len(locked) > 0 {
		return "Not connected yet: " + strings.Join(locked, ", ") + " — /connect <kind> adds one; /model-search <text> searches what is."
	}
	return ""
}

func modelInCatalogue(id string) (string, error) {
	models, err := searchModelCatalog(id)
	if err != nil || len(models) == 0 && strings.TrimSpace(id) == "" {
		return "", nil // no catalogue to check against
	}
	for _, m := range models {
		if strings.EqualFold(m.ID, id) {
			return m.ID, nil
		}
	}
	// The search is a substring match, so an empty result for a full id means
	// nothing in the catalogue resembles it — often because the provider that
	// would list it is not connected: say which, and how (Greg, 2026-10-03:
	// "when not connected suggest /connect").
	if len(models) == 0 {
		if hint := connectHint(id, listProviders()); hint != "" {
			return "", fmt.Errorf("no model %q among your connected providers. %s", id, hint)
		}
		return "", fmt.Errorf("no model %q in the catalogue — /model-search <text> finds one, and /model auto goes back to the ladder", id)
	}
	near := make([]string, 0, 3)
	for _, m := range models {
		if len(near) == 3 {
			break
		}
		near = append(near, m.ID)
	}
	return "", fmt.Errorf("no model %q in the catalogue. Did you mean: %s", id, strings.Join(near, ", "))
}

// unknownModel is the shape of "the catalogue has never heard of this": the
// upstream listing answers 404, which our proxy reports as a bad gateway.
func unknownModel(err error) bool {
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "404") || strings.Contains(e, "not found")
}

// compactReply is the gateway's answer to /compact (gateway CompactResult).
type compactReply struct {
	BeforeTokens   int  `json:"before_tokens"`
	AfterTokens    int  `json:"after_tokens"`
	BeforeMessages int  `json:"before_messages"`
	AfterMessages  int  `json:"after_messages"`
	Summarized     bool `json:"summarized"`
	Written        bool `json:"written"`
}

// line says what /compact did, and whether a focus had a summary to lead.
func (r compactReply) line(focus string) string {
	if r.AfterTokens >= r.BeforeTokens {
		return fmt.Sprintf("Nothing worth compacting: the conversation is %s tokens in %d messages.",
			humanTokens(int64(r.BeforeTokens)), r.BeforeMessages)
	}
	sizes := fmt.Sprintf("%s → %s tokens, %d → %d messages", humanTokens(int64(r.BeforeTokens)),
		humanTokens(int64(r.AfterTokens)), r.BeforeMessages, r.AfterMessages)
	if r.Summarized && r.Written {
		return "Compacted: " + sizes + ". The model summarized the older part; the next turn starts from its summary."
	}
	if r.Summarized {
		return "Compacted: " + sizes + ". The model could not be asked, so a digest of what was asked and answered stands in for the older part."
	}
	line := "Compacted: " + sizes + " — repeated and stale tool output stubbed; the rest is recent enough to keep word for word."
	if strings.TrimSpace(focus) != "" {
		line += " Nothing was summarized, so the focus was not needed."
	}
	return line
}

// ownKeyLabel names the provider answering on the person's key: "Your own
// DeepSeek key", "Your own Baseten key"; "Your own key" when none is marked.
func ownKeyLabel() string {
	for _, p := range listProviders() {
		if p.Active && p.Name != "" {
			return "Your own " + p.Name + " key"
		}
	}
	return "Your own key"
}

// gatewayDecisionsOn asks the gateway whether a decision model judges its
// turns. Unknown (no answer) counts as on: a hint on a guess is a nag.
func gatewayDecisionsOn(c *Client) bool {
	var st struct {
		Decisions *bool `json:"decisions"`
	}
	if err := c.GetJSON("/api/llm/burst", &st); err != nil || st.Decisions == nil {
		return true
	}
	return *st.Decisions
}
