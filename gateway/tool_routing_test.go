package gateway

import (
	"context"
	"strings"
	"testing"

	"memdoor/pkg/decision"
	"memdoor/tools"
)

func names(ts []tools.ToolDefinition) string {
	var n []string
	for _, t := range ts {
		n = append(n, t.Name)
	}
	return strings.Join(n, ",")
}

func TestTurnToolsAllowRules(t *testing.T) {
	all := []tools.ToolDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	ctx := context.Background()
	if got := names(submittedTools(ctx, all)); got != "a,b,c" {
		t.Fatalf("no narrowing must leave tools unchanged: %s", got)
	}
	ctx = withTurnToolsAllow(ctx, []string{"a", "b"})
	ctx = withTurnToolsAllow(ctx, []string{"b", "c"})
	if got := names(submittedTools(ctx, all)); got != "b" {
		t.Fatalf("contributors must intersect: %s", got)
	}
	if got := submittedTools(withTurnToolsAllow(context.Background(), []string{}), all); len(got) != 0 {
		t.Fatalf("empty list must submit nothing narrowable: %v", names(got))
	}
}

type familyDecider struct {
	choice string
	conf   float64
	down   bool
	last   decision.Request
}

func (f *familyDecider) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.last = req
	if f.down {
		return decision.Unavailable(decision.ReasonDeadline, "slow")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"family": {Kind: decision.KindChoice, Choice: f.choice, Confidence: f.conf},
	}}
}

func TestToolRouterAppliesAndFailsOpen(t *testing.T) {
	settings := map[string]string{settingToolRouting: "on"}
	read := func(k string) string { return settings[k] }
	d := &familyDecider{choice: "research", conf: 0.9}
	r := &toolRouter{svc: d, read: read}
	ctx := context.Background()

	allow := r.route(ctx, "coder", "what does the Go 1.25 release notes say about iterators?")
	if allow == nil || !hasTool(allow, "web_search") || !hasTool(allow, "apply_patch") || hasTool(allow, "todo_write") {
		t.Fatalf("research family + core expected: %v", allow)
	}
	if q := d.last.Questions["family"]; q.Kind != decision.KindChoice || len(q.Options) != 4 {
		t.Fatalf("one choice over the four families: %+v", q)
	}

	for name, tc := range map[string]struct {
		r      *toolRouter
		agent  string
		prompt string
	}{
		"off":            {&toolRouter{svc: d, read: func(string) string { return "off" }}, "coder", "x"},
		"no families":    {r, "chief", "x"},
		"long prompt":    {r, "coder", strings.Repeat("x", toolRoutingMaxPrompt+1)},
		"unavailable":    {&toolRouter{svc: &familyDecider{down: true}, read: read}, "coder", "x"},
		"low confidence": {&toolRouter{svc: &familyDecider{choice: "change", conf: 0.6}, read: read}, "coder", "x"},
		"unknown family": {&toolRouter{svc: &familyDecider{choice: "dance", conf: 0.99}, read: read}, "coder", "x"},
		"nil router":     {nil, "coder", "x"},
	} {
		if got := tc.r.route(ctx, tc.agent, tc.prompt); got != nil {
			t.Errorf("%s: must fail open (nil), got %v", name, got)
		}
	}

	settings[settingToolFamilies] = `{"coder":{"alwaysAllow":["bash"],"families":{"search":{"description":"find code","tools":["jgrep"]},"edit":{"description":"change code","tools":["apply_patch"]}}}}`
	d.choice = "search"
	if got := r.route(ctx, "coder", "where is X"); strings.Join(got, ",") != "bash,jgrep" {
		t.Fatalf("configured families: %v", got)
	}
	delete(settings, settingToolFamilies)

	// The coder's own families: a change turn keeps the whole edit loop, a
	// question turn drops cron and the todo list, and both stay inside the
	// palette. apply_patch is in EVERY turn: a review that finds a bug fixes it.
	d.choice, d.conf = "change", 0.9
	allow = r.route(ctx, "coder", "in throttle.go make the wait ceiling configurable and add a test")
	for _, n := range []string{"apply_patch", "bash", "read_file", "grep", "jgrep", "jread", "glob", "todo_read"} {
		if !hasTool(allow, n) {
			t.Errorf("a change turn needs %q: %v", n, allow)
		}
	}
	if !hasTool(allow, "workflow") {
		t.Errorf("a change turn writes a DAG and runs it: the workflow tool belongs in it (live 2026-10-02, the coder fell back to bash): %v", allow)
	}
	if hasTool(allow, "cron") {
		t.Errorf("a change turn pays for the cron schema it does not need: %v", allow)
	}
	d.choice = "answer"
	allow = r.route(ctx, "coder", "where is the subagent result turned into a message?")
	if hasTool(allow, "cron") || hasTool(allow, "todo_write") {
		t.Errorf("a question turn should not carry the cron or todo schemas: %v", allow)
	}
	if !hasTool(allow, "jgrep") || !hasTool(allow, "jread") || !hasTool(allow, "read_file") {
		t.Errorf("a question turn still needs the judged reads: %v", allow)
	}
	for _, fam := range []string{"answer", "investigate"} {
		d.choice = fam
		allow = r.route(ctx, "coder", "review last changes")
		for _, core := range []string{"apply_patch", "locate"} {
			if !hasTool(allow, core) {
				t.Errorf("%s is always there, the %q family dropped it: %v", core, fam, allow)
			}
		}
	}
}

func TestBuiltinFamiliesCoverTheirPalettes(t *testing.T) {
	for agent, palette := range map[string]string{"coder": coderToolPalette} {
		cfg := builtinToolRouting[agent]
		if len(cfg.Families) < 2 {
			t.Fatalf("%s has no families: routing would never narrow", agent)
		}
		covered := map[string]bool{}
		for _, n := range cfg.AlwaysAllow {
			covered[n] = true
		}
		inPalette := map[string]bool{}
		for _, n := range strings.Split(strings.Trim(palette, "[]"), ",") {
			inPalette[strings.Trim(n, `"`)] = true
		}
		for id, f := range cfg.Families {
			if strings.TrimSpace(f.Description) == "" {
				t.Errorf("%s family %q has no description: the decision model cannot choose it", agent, id)
			}
			for _, n := range f.Tools {
				covered[n] = true
				if !inPalette[n] {
					t.Errorf("%s family %q names %q, which is not in the agent's palette", agent, id, n)
				}
			}
		}
		// "mcp" is not a tool: it admits the person's MCP tools, whose family
		// is added on a turn that has them (withMCPFamily).
		covered[mcpPaletteEntry] = true
		for n := range inPalette {
			if !covered[n] {
				t.Errorf("%s tool %q is in no family and not always allowed: routing would hide it", agent, n)
			}
		}
	}
}

func hasTool(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// A fresh workspace has no settings rows, and the per-turn toolbox must still
// work: it is one of the three things the product is sold on (onboarding walk,
// 2026-09-27). Explicit off is still off.
func TestToolRoutingIsOnByDefault(t *testing.T) {
	for _, on := range []string{"", "on", "1", "true", "anything"} {
		if !toolRoutingOn(on) {
			t.Errorf("%q must leave routing on", on)
		}
	}
	for _, off := range []string{"off", "OFF", "0", "false", "no"} {
		if toolRoutingOn(off) {
			t.Errorf("%q must turn routing off", off)
		}
	}
	// And the router follows: a workspace with no setting still narrows.
	settings := map[string]string{}
	r := &toolRouter{svc: &familyDecider{choice: "change", conf: 0.9}, read: func(k string) string { return settings[k] }}
	if allow := r.route(context.Background(), "coder", "fix the throttle test"); allow == nil {
		t.Error("a fresh workspace must still get a narrowed toolbox")
	}
	settings[settingToolRouting] = "off"
	if allow := r.route(context.Background(), "coder", "fix the throttle test"); allow != nil {
		t.Error("decision_tool_routing=off must be honoured")
	}
}

// The workflow tool is in every toolbox the coder can get (live 2026-10-02:
// a research sentence was routed to one without it, and the coder ran the
// workflow through bash, drawing nothing in the window).
// A tool the request names by its exact name is in the toolbox whatever
// family the router picks: "Use the cron tool …" routed to change left the
// coder without cron, and it spent thirty reads looking for one through bash
// (live 2026-10-05). A sentence that does not name it keeps the lean family.
func TestAToolTheRequestNamesStaysInTheToolbox(t *testing.T) {
	read := func(k string) string { return map[string]string{settingToolRouting: "on"}[k] }
	d := &familyDecider{choice: "change", conf: 0.9}
	r := &toolRouter{svc: d, read: read}
	ctx := context.Background()
	if allow := r.route(ctx, "coder", "Use the cron tool: every 15s, up to 4 times, task: 'run date +%S and answer exactly: still ticking'. Then end your turn."); !hasTool(allow, "cron") {
		t.Errorf("the request names cron; the change family must carry it: %v", allow)
	}
	d.choice = "answer"
	if allow := r.route(ctx, "coder", "Spawn a subagent with sessions_spawn whose task is exactly: run the tests and report."); !hasTool(allow, "sessions_spawn") {
		t.Errorf("the request names sessions_spawn; the answer family must carry it: %v", allow)
	}
	if allow := r.route(ctx, "coder", "where is the subagent result turned into a message?"); hasTool(allow, "cron") || hasTool(allow, "sessions_spawn") {
		t.Errorf("a sentence naming no tool keeps the lean family: %v", allow)
	}
	if allow := r.route(ctx, "coder", "is the cronjob history kept anywhere?"); hasTool(allow, "cron") {
		t.Errorf("cronjob is not the tool's name: %v", allow)
	}
}

func TestTheWorkflowToolIsAlwaysInTheCodersToolbox(t *testing.T) {
	cfg := builtinToolRouting["coder"]
	for id := range cfg.Families {
		allow := append(append([]string{}, cfg.AlwaysAllow...), cfg.Families[id].Tools...)
		if !hasTool(allow, "workflow") {
			t.Errorf("family %s has no workflow tool: %v", id, allow)
		}
	}
}
