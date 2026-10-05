package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// CAN THIS MODEL RUN THE LOOP? (Greg, 2026-09-27: "the coding agent should work
// with any model!? we should test all of model with our provider. Maybe we
// should detect it", and the OpenRouter ranking: someone arriving from it will
// pin whatever they already pay for.)
//
// The catalogue says a model advertises tools. It does not say the model emits a
// call with arguments that parse, picks the right tool of several, carries on
// after a tool result, or writes a patch our own parser accepts. Those are what
// a turn needs, so those are what this probes — on the person's own key, against
// the model they are about to pin, for a few hundred tokens.
//
// Three calls: one to see a tool call, one to see the turn continue after a
// result, one to see a patch. Results land in ~/.memdoor/model-checks.json so
// the answer survives the process and can be shown when someone pins a model.

// modelCheck is one model's report.
type modelCheck struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	ToolCall  bool      `json:"tool_call"`  // emitted a native tool call at all
	RightTool bool      `json:"right_tool"` // chose the one the request needed
	ValidArgs bool      `json:"valid_args"` // arguments parsed, with the field asked for
	RoundTrip bool      `json:"round_trip"` // answered in text after a tool result
	Patch     bool      `json:"patch"`      // wrote a patch our parser accepts
	// Followed: told to load a skill FIRST when asked for a workflow, did it
	// reach for that skill before anything else? "yes", "no", or "" for a
	// report from before this probe existed (2026-10-02: gpt-oss-120b on
	// Groq did a digest inline, Qwen 27B loaded the skill and lost the
	// thread; the coder prompt routes workflow asks exactly this way).
	Followed  string `json:"followed,omitempty"`
	Ms        int64  `json:"ms"` // the three calls together
	InTokens  int64  `json:"in_tokens"`
	OutTokens int64  `json:"out_tokens"`
	Note      string `json:"note,omitempty"` // the first thing that went wrong
}

// Runs reports whether this model can hold a coder turn: a call, the right one,
// arguments that parse, and the turn continuing afterwards. The patch is
// reported separately — a model can be useful for reading and answering without
// it, and the harness has a text-tag fallback for writing.
func (c modelCheck) Runs() bool {
	// Not RightTool: choosing grep before read_file is a strategy, and the turn
	// carries on either way. What a turn cannot survive is a call whose
	// arguments do not match the schema, or silence after a result.
	return c.ToolCall && c.ValidArgs && c.RoundTrip
}

// checkModel probes one model. The gateway's own client serves it, so this works
// the same on a person's key and on a seat.
// The return is NAMED so the deferred timing lands in it: with a plain local
// the value is copied at the return and every report read 0 ms (caught the
// first time it ran, 2026-09-27).
func (s *Server) checkModel(ctx context.Context, id string) (res modelCheck) {
	res = modelCheck{ID: id, At: time.Now()}
	start := time.Now()
	defer func() { res.Ms = time.Since(start).Milliseconds() }()

	ctx = context.WithValue(ctx, sharedctx.ModelKey, id)
	ctx = context.WithValue(ctx, "buddy_agent_name", "coder") //nolint:staticcheck // the key the agent runtime reads
	client, err := s.clientFactory.GetClientFor(ctx, "coder")
	if err != nil {
		res.Note = "no client for " + id + ": " + err.Error()
		return res
	}

	readDesc := "Read a file and return its contents"
	grepDesc := "Search the project for a pattern"
	patchDesc := "Create or edit files with a Codex-format patch"
	readFile := llm.ToolUnionParam{OfTool: &llm.ToolParam{Name: "read_file", Description: &readDesc,
		InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"path": map[string]any{"type": "string"}}}}}
	grep := llm.ToolUnionParam{OfTool: &llm.ToolParam{Name: "grep", Description: &grepDesc,
		InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"pattern": map[string]any{"type": "string"}}}}}
	patch := llm.ToolUnionParam{OfTool: &llm.ToolParam{Name: "apply_patch", Description: &patchDesc,
		InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"input": map[string]any{"type": "string"}}}}}

	// 1. A call, the right one, with arguments that parse.
	first, err := client.Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 512,
		System:    []llm.TextBlockParam{{Type: "text", Text: "You are a coding agent. Use a tool rather than answering from memory."}},
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("Read the file cache.go and tell me what it contains. Use a tool."))},
		Tools:     []llm.ToolUnionParam{readFile, grep},
	})
	if err != nil {
		res.Note = "first call failed: " + shortErr(err)
		return res
	}
	res.InTokens, res.OutTokens = first.Usage.InputTokens, first.Usage.OutputTokens
	var callID, callName, callArgs string
	for _, b := range first.Content {
		if b.Type == "tool_use" {
			callID, callName, callArgs = b.ID, b.Name, string(b.Input)
			break
		}
	}
	if callName == "" {
		res.Note = "answered without calling a tool"
		return res
	}
	res.ToolCall = true
	// RightTool is which tool it reached for; ValidArgs is whether it honoured
	// THAT tool's schema. Keeping them apart matters: z-ai/glm-5.3-flash chose
	// grep with a perfectly valid pattern, and an earlier version of this probe
	// called that broken arguments (2026-09-27). Choosing a different tool is a
	// strategy; ignoring the schema is an incompatibility, and only the second
	// one stops a turn.
	res.RightTool = callName == "read_file"
	res.ValidArgs = argsMatchSchema(callName, callArgs)
	if !res.ValidArgs && res.Note == "" {
		res.Note = callName + " called with arguments its schema does not declare: " + truncate(callArgs, 80)
	}

	// 2. The turn continues after a tool result.
	second, err := client.Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 512,
		Messages: []llm.MessageParam{
			llm.NewUserMessage(llm.NewTextBlock("Read the file cache.go and tell me what it contains. Use a tool.")),
			llm.NewAssistantMessage(llm.NewToolUseBlock(callID, callArgs, callName)),
			llm.NewUserMessage(llm.NewToolResultBlock(callID, "package cache\n\n// Store keeps values in memory.\ntype Store struct{}\n", false)),
		},
		Tools: []llm.ToolUnionParam{readFile, grep},
	})
	if err != nil {
		res.Note = "did not continue after a tool result: " + shortErr(err)
		return res
	}
	res.InTokens += second.Usage.InputTokens
	res.OutTokens += second.Usage.OutputTokens
	// A TURN CONTINUES EITHER WAY. Answering in text and calling the next tool
	// are both "carried on"; only silence is a failure. Gemini called a second
	// tool here and the first version of this probe marked it a miss, which was
	// the probe being wrong about turns, not the model (2026-09-27).
	for _, b := range second.Content {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			res.RoundTrip = true
			break
		}
		if b.Type == "tool_use" {
			res.RoundTrip = true
			break
		}
	}
	if !res.RoundTrip && res.Note == "" {
		res.Note = "neither answered nor called another tool after the result"
	}

	// 3. A patch our own parser accepts. TWO ATTEMPTS, because one miss can be
	// variance rather than an incompatibility: xiaomi/mimo-v2.6-flash failed
	// this once and passed the next time, and a table that calls a model
	// incapable on one sample is worse than no table (2026-09-27).
	patchParams := llm.MessageNewParams{
		MaxTokens: 700,
		System:    []llm.TextBlockParam{{Type: "text", Text: tools.ApplyPatchDefinition.Description}},
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(
			"In cache.go, which reads:\n\npackage cache\n\ntype Store struct{}\n\nadd a doc comment above type Store saying \"Store keeps values in memory.\" Use apply_patch."))},
		Tools: []llm.ToolUnionParam{patch},
	}
	badPatch, calledPatch, saidText := "", false, ""
	for attempt := 0; attempt < 2 && !res.Patch; attempt++ {
		third, err := client.Messages().New(ctx, patchParams)
		if err != nil {
			break
		}
		res.InTokens += third.Usage.InputTokens
		res.OutTokens += third.Usage.OutputTokens
		for _, b := range third.Content {
			if b.Type == "text" && saidText == "" {
				saidText = truncate(b.Text, 160)
			}
			if b.Type != "tool_use" || b.Name != "apply_patch" {
				continue
			}
			calledPatch = true
			var in struct {
				Input string `json:"input"`
			}
			if json.Unmarshal(b.Input, &in) == nil {
				if tools.PatchParses(in.Input) {
					res.Patch = true
				} else if badPatch == "" {
					// WHY, not just THAT: the shape it sent is what decides
					// whether an adapter can repair it (model_adapter.go).
					badPatch = truncate(in.Input, 160)
				}
			}
			break
		}
	}
	// WHICH failure it is decides whether an adapter can help: a format we
	// could convert, or a model that never called the tool at all.
	if !res.Patch && res.Note == "" {
		switch {
		case badPatch != "":
			res.Note = "patch not in our format, twice: " + badPatch
		case calledPatch:
			res.Note = "called apply_patch twice with arguments that did not parse"
		case saidText != "":
			res.Note = "wrote prose instead of calling apply_patch: " + saidText
		default:
			res.Note = "no apply_patch call and nothing said"
		}
	}
	if !res.Patch && res.Note == "" {
		res.Note = "no patch our parser accepts (the text-tag fallback covers this)"
	}

	// 4. An instruction followed over an instinct. The coder's prompt routes
	// a workflow ask to the "workflow" skill first; a model that does the
	// steps inline instead skips the whole feature. Offered: skill, bash,
	// read_file — the instinct is bash.
	skillDesc := "Load a skill: step-by-step instructions for a kind of task"
	skillTool := llm.ToolUnionParam{OfTool: &llm.ToolParam{Name: "skill", Description: &skillDesc,
		InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}}
	bashDesc := "Run a shell command"
	bashTool := llm.ToolUnionParam{OfTool: &llm.ToolParam{Name: "bash", Description: &bashDesc,
		InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}}
	fourth, err := client.Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 400,
		System:    []llm.TextBlockParam{{Type: "text", Text: "You are a coding agent.\n\nAsked for a WORKFLOW — steps in order, in parallel, or behind an approval — load the \"workflow\" skill FIRST (skill with command \"workflow\") and follow it. Never do the steps yourself."}},
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("I want a digest workflow: list the last 5 commits, then summarize them in three bullets, then have an agent write them into DIGEST.md. Run it."))},
		Tools:     []llm.ToolUnionParam{skillTool, bashTool, readFile},
	})
	res.Followed = "no"
	if err == nil {
		res.InTokens += fourth.Usage.InputTokens
		res.OutTokens += fourth.Usage.OutputTokens
		for _, b := range fourth.Content {
			if b.Type != "tool_use" {
				continue
			}
			var in struct {
				Command string `json:"command"`
			}
			_ = json.Unmarshal(b.Input, &in)
			if b.Name == "skill" && strings.Contains(strings.ToLower(in.Command), "workflow") {
				res.Followed = "yes"
			} else if res.Note == "" {
				res.Note = "told to load the workflow skill first, it called " + b.Name + " instead"
			}
			break
		}
		if res.Followed == "no" && res.Note == "" {
			res.Note = "told to load the workflow skill first, it did not call a tool"
		}
	}
	return res
}

func shortErr(err error) string { return truncate(err.Error(), 120) }

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- the store -------------------------------------------------------------

func modelChecksPath() string {
	return shared.MemdoorHome("model-checks.json")
}

// saveModelCheck keeps the newest report per model. One small file: the answer
// has to survive the process to be worth anything when someone pins a model.
func saveModelCheck(c modelCheck) {
	p := modelChecksPath()
	if p == "" {
		return
	}
	all := loadModelChecks()
	all[c.ID] = c
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, b, 0o600)
}

// staleAfter is how long a report is worth trusting. A model is a moving target
// — a vendor changes a default, an endpoint disappears, a quantisation lands —
// and a week is long enough that a person probing weekly spends a few thousand
// tokens a month, short enough that a change is ours to notice rather than a
// customer's. One number, no setting: a knob here would be a knob about money.
const staleAfter = 7 * 24 * time.Hour

// staleChecks is the ids whose newest report has aged past staleAfter, oldest
// first so a probe that is interrupted has done the most good.
func staleChecks(all map[string]modelCheck, now time.Time) []string {
	out := make([]modelCheck, 0, len(all))
	for _, c := range all {
		if c.At.IsZero() || now.Sub(c.At) >= staleAfter {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	ids := make([]string, 0, len(out))
	for _, c := range out {
		ids = append(ids, c.ID)
	}
	return ids
}

func loadModelChecks() map[string]modelCheck {
	out := map[string]modelCheck{}
	p := modelChecksPath()
	if p == "" {
		return out
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// --- the endpoint ----------------------------------------------------------

// handleModelCheck is POST /api/models/check?id=…&ladder=1&stale=1: probe a
// model, every rung of the coder's ladder, or every report that has gone stale,
// and answer with the reports. GET returns what has already been probed, without
// spending anything.
//
// RE-PROBING IS NOT A BACKGROUND JOB. A probe is three or four real calls on the
// person's own key, so nothing here fires on a timer of ours: a silent process
// spending somebody's money to keep our table fresh is not a trade we get to
// make for them. stale=1 is the shape a schedule takes instead — the person's
// own cron or launchd line, asking only for the reports older than staleAfter,
// and spending nothing at all when none of them are.
func (s *Server) handleModelCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(map[string]any{"checks": loadModelChecks()})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "GET or POST"}`, http.StatusMethodNotAllowed)
		return
	}
	ids := []string{}
	if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
		ids = append(ids, id)
	}
	if r.URL.Query().Get("ladder") != "" {
		if re := providers.ActiveRemoteEngine(); re != nil {
			ids = append(ids, re.AgentLadders["coder"]...)
		}
	}
	if r.URL.Query().Get("stale") != "" {
		stale := staleChecks(loadModelChecks(), time.Now())
		if len(stale) == 0 {
			// Nothing to say and nothing spent. A schedule that finds every
			// report fresh has to be cheap, or it will be turned off.
			_ = json.NewEncoder(w).Encode(map[string]any{"checks": []modelCheck{}, "stale": 0})
			return
		}
		ids = append(ids, stale...)
	}
	if len(ids) == 0 {
		http.Error(w, `{"error": "id=vendor/name, ladder=1, or stale=1"}`, http.StatusBadRequest)
		return
	}
	out := make([]modelCheck, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		c := s.checkModel(r.Context(), id)
		saveModelCheck(c)
		out = append(out, c)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"checks": out})
}

// argsMatchSchema is whether a call filled a field the tool it named actually
// declares. The probe offers read_file{path}, grep{pattern} and
// apply_patch{input}; anything else is the model inventing a shape.
func argsMatchSchema(tool, args string) bool {
	fields := map[string][]string{
		"read_file":   {"path"},
		"jread":       {"path"},
		"grep":        {"pattern"},
		"jgrep":       {"pattern"},
		"apply_patch": {"input"},
	}[tool]
	if len(fields) == 0 {
		return false
	}
	var got map[string]any
	if json.Unmarshal([]byte(args), &got) != nil {
		return false
	}
	for _, f := range fields {
		if v, ok := got[f]; ok {
			if s, isStr := v.(string); !isStr || strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}
