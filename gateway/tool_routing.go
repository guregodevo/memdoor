package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
)

// Tool routing: the decision-model contributor to the turn's toolsAllow, the
// way openclaw-jev-router fills OpenClaw's before_prompt_build hook. One choice
// question per turn — which tool FAMILY does this request need — over the
// agent's family descriptions; the chosen family plus the agent's alwaysAllow
// core becomes the turn's submitted tools. It fails open: routing off, no
// families for the agent, a long prompt, a timeout, an unavailable decision or
// a confidence under the threshold all leave the tools unchanged.
//
//	decision_tool_routing  on — enable (read per turn, no restart)
//	tool_families          optional JSON overriding the built-in families:
//	                       {"<agent>": {"alwaysAllow": [...], "families": {"<id>": {"description": "...", "tools": [...]}}}}

const (
	settingToolRouting  = "decision_tool_routing"
	settingToolFamilies = "tool_families"

	toolRoutingThreshold = 0.75
	toolRoutingTimeout   = 2500 * time.Millisecond
	toolRoutingMaxPrompt = 12000
)

type toolFamily struct {
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

type agentToolRouting struct {
	AlwaysAllow []string              `json:"alwaysAllow"`
	Families    map[string]toolFamily `json:"families"`
}

// builtinToolRouting covers the agents with the widest palettes. Other agents
// route only when tool_families configures them.
//
// The coder's palette costs 20,916 bytes of schema (~5,200 tokens) submitted
// on EVERY call of a turn, measured 2026-09-27: on a 12-call edit turn that is
// 37% of the turn's input tokens, and it is the largest single cost decisions
// can remove. Each family below is the full set its kind of work needs — a
// tool the turn needs but cannot see gets called with guessed arguments
// (2026-09-25), so families err wide:
// change 16,758 bytes (−20%), answer 13,075 (−37%), investigate 17,522 (−16%),
// schedule 11,139 (−47%). Greg, 2026-09-27: "no risk no quality degradation" —
// so each family carries more than its own core, and the cut is smaller than
// it could be.
var builtinToolRouting = map[string]agentToolRouting{
	"coder": {
		// The core of every coder turn: run things, read a file it was given,
		// EDIT one, pick up the saved plan, load a skill, look at an image.
		// apply_patch is core, never a family's: the prompt tells the coder it
		// implements by calling apply_patch, and a turn routed "answer" lost
		// it — "review last changes" (2026-09-28) reviewed and could not fix
		// what it found. Greg: "apply_patch is always there by default".
		// locate too (2026-09-28, "it always take time to read all files and
		// search relevant files"): finding where the code lives is the first
		// step of every kind of work, and a turn without it reads its way there.
		// recall: a stub names its call only when compaction has run; routing
		// that hid it would leave the model to re-run the call instead.
		// workflow is always there too (2026-10-02): a research sentence was
		// routed to a toolbox without it and the coder ran `memdoor workflow`
		// through bash — a shell run draws nothing in the window.
		AlwaysAllow: []string{"bash", "read_file", "apply_patch", "locate", "skill", "notes", "recall", "workflow"},
		Families: map[string]toolFamily{
			"change": {
				Description: "Write, fix or change code, configuration or documentation in the project, then build, test or run it.",
				// A family must carry every tool its kind of work might reach
				// for, or the turn guesses the arguments instead.
				// sessions_spawn (2026-10-05, Greg: "give the coder sessions_spawn"):
				// a well-defined subtask runs in parallel in its own session and
				// its result wakes this conversation.
				Tools: []string{"grep", "jgrep", "jread", "glob", "todo_write", "todo_read", "ask_user_question", "workflow", "sessions_spawn"},
			},
			"answer": {
				Description: "Find, explain or review code that already exists, without changing any file.",
				Tools:       []string{"grep", "jgrep", "jread", "glob"},
			},
			"investigate": {
				Description: "Work out why something failed or behaved oddly, from logs, runtime output or an error — or wait on something still running (CI, a deploy, a long build) and check back later.",
				// the web: an error message or a library's behaviour this machine
				// has no answer for
				Tools: []string{"grep", "jgrep", "jread", "glob", "ask_user_question", "web_search", "web_fetch", "cron", "workflow", "sessions_spawn"},
			},
			"research": {
				Description: "Look something up on the web: current facts, news, documentation or a library this machine does not have.",
				Tools:       []string{"web_search", "web_fetch", "grep", "jgrep", "jread", "glob"},
			},
		},
	},
}

// namedTools are the palette's tools the request names by their exact name
// and the chosen family left out: "Use the cron tool: every 15s …" was routed
// to the change family, the coder found no cron tool, and spent thirty reads
// and a throwaway job looking for one through bash (live 2026-10-05). A tool
// the person named is in the toolbox, whatever kind of work the rest of the
// sentence is. An exact name, not a guess at intent.
func namedTools(prompt string, cfg agentToolRouting, allow []string) []string {
	var named []string
	seen := map[string]bool{}
	for _, t := range allow {
		seen[t] = true
	}
	for _, fam := range cfg.Families {
		for _, t := range fam.Tools {
			if seen[t] {
				continue
			}
			if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(t) + `\b`).MatchString(prompt) {
				named = append(named, t)
				seen[t] = true
			}
		}
	}
	sort.Strings(named)
	return named
}

// toolRoutingOn: EMPTY MEANS ON, unlike what this used
// to do. A fresh workspace has no settings rows, so `isOn("")` was false and the
// per-turn toolbox — one of the three things the product is sold on — was
// silently off for every new install until somebody set a setting nobody
// mentions (onboarding walk, 2026-09-27). Explicit off still turns it off, and
// without a decision service nothing here runs anyway.
func toolRoutingOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

type toolRouter struct {
	svc  decision.Service
	read func(key string) string
}

func (r *toolRouter) routingFor(agent string) (agentToolRouting, bool) {
	if raw := strings.TrimSpace(r.read(settingToolFamilies)); raw != "" {
		var cfg map[string]agentToolRouting
		if err := json.Unmarshal([]byte(raw), &cfg); err == nil {
			if a, ok := cfg[agent]; ok && len(a.Families) >= 2 {
				return a, true
			}
		} else {
			logs.New("Decisions").Warn("tool_families is not valid JSON; using built-in families", slog.String("error", err.Error()))
		}
	}
	a, ok := builtinToolRouting[agent]
	return a, ok
}

// route returns the turn's allow list, or nil to leave the tools unchanged.
func (r *toolRouter) route(ctx context.Context, agent, prompt string) []string {
	if r == nil || !toolRoutingOn(r.read(settingToolRouting)) || agent == "" || len(prompt) > toolRoutingMaxPrompt {
		return nil
	}
	cfg, ok := r.routingFor(agent)
	cfg = withMCPFamily(ctx, cfg)
	if !ok || len(cfg.Families) < 2 {
		return nil
	}
	ids := make([]string, 0, len(cfg.Families))
	for id := range cfg.Families {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	opts := make([]decision.Option, len(ids))
	for i, id := range ids {
		opts[i] = decision.Option{Label: id, Description: cfg.Families[id].Description}
	}
	q, err := decision.Choice("Which kind of work does this request need?", opts)
	if err != nil {
		return nil
	}
	start := time.Now()
	res := r.svc.Evaluate(ctx, decision.Request{State: prompt, Questions: map[string]decision.Question{"family": q}},
		decision.Options{AgentID: agent, Purpose: "tool-routing", Timeout: toolRoutingTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Tool routing skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return nil
	}
	a := res.Answers["family"]
	fam, known := cfg.Families[a.Choice]
	if !known || a.Confidence < toolRoutingThreshold {
		log.Info("Tool routing skipped", slog.String("agent", agent), slog.String("family", a.Choice),
			slog.Float64("confidence", a.Confidence), slog.String("reason", "low-confidence-or-unknown"))
		return nil
	}
	allow := append(append([]string{}, cfg.AlwaysAllow...), fam.Tools...)
	allow = append(allow, namedTools(prompt, cfg, allow)...)
	log.Info("Tool routing applied", slog.String("agent", agent), slog.String("family", a.Choice),
		slog.Float64("confidence", a.Confidence), slog.Int("tools", len(allow)),
		slog.Int64("ms", time.Since(start).Milliseconds()), slog.String("model", res.Model))
	return allow
}
