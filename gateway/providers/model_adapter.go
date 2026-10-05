package providers

import (
	"encoding/json"
	"log/slog"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/tools"
)

// ANY MODEL SHOULD RUN THE CODING AGENT (Greg, 2026-09-27: "we need our coding
// agent to work with any model", "try to identify common pattern of failure so
// we can generalize the adapter and not have too many coding paths").
//
// The probe (gateway/model_check.go) put twelve models through what a turn
// needs. Every failure that was not a dead model id turned out to be ONE
// pattern: the model produced the right payload in the wrong envelope.
//
//	google/gemini-3.8-flash      called grep{"query":"cache.go"} when the schema
//	                             said read_file{path} — right intent, wrong tool
//	                             and an invented argument name
//	meta-llama/llama-4-maverick  wrote a CORRECT Codex patch as prose instead of
//	                             calling apply_patch — right payload, no envelope
//
// So there is one generic adapter, not one per vendor, with three rules that
// each fire only on unambiguous evidence:
//
//  1. rename — a declared field is missing and the model sent a known alias
//  2. reroute — the value names a file and a file-reading tool was offered
//  3. lift — there is no tool call, and the text is a patch OUR OWN PARSER
//     accepts, so the model plainly meant to call apply_patch
//
// The interface and registry stay so a genuinely vendor-specific quirk can be
// added later as its own implementation. Today one implementation covers every
// failure observed, which is the point: adapters, not a ladder of special cases.
type ModelAdapter interface {
	// Name is what the logs call this adapter.
	Name() string
	// Applies is whether this adapter speaks for that model id.
	Applies(model string) bool
	// FixToolCall repairs a call the model made, given the tools it was offered.
	// It must be a no-op when the call already matches the schema.
	FixToolCall(name string, args []byte, offered []ToolShape) (string, []byte, bool)
	// LiftFromText turns a payload the model wrote as prose into the call it
	// meant, or reports that there is nothing unambiguous to lift.
	LiftFromText(text string, offered []ToolShape) (string, []byte, bool)
}

// ToolShape is what an adapter needs to know about an offered tool: its name
// and the argument names its schema declares, required ones first.
type ToolShape struct {
	Name   string
	Fields []string
}

// adapters is the registry. envelopeAdapter speaks for every model; a
// vendor-specific implementation would go before it.
var adapters = []ModelAdapter{envelopeAdapter{}}

// AdapterFor is the adapter that speaks for a model.
func AdapterFor(model string) ModelAdapter {
	for _, a := range adapters {
		if a.Applies(model) {
			return a
		}
	}
	return noopAdapter{}
}

// noopAdapter changes nothing. Kept as the floor: a path that wants the raw
// model output asks for it explicitly rather than by accident.
type noopAdapter struct{}

func (noopAdapter) Name() string        { return "none" }
func (noopAdapter) Applies(string) bool { return true }
func (noopAdapter) FixToolCall(n string, a []byte, _ []ToolShape) (string, []byte, bool) {
	return n, a, false
}
func (noopAdapter) LiftFromText(string, []ToolShape) (string, []byte, bool) {
	return "", nil, false
}

// envelopeAdapter fixes the envelope without touching the intent.
type envelopeAdapter struct{}

func (envelopeAdapter) Name() string        { return "envelope" }
func (envelopeAdapter) Applies(string) bool { return true }

// fieldAliases are the argument names models reach for instead of the schema's.
// Generic on purpose: the habit is not one vendor's.
var fieldAliases = map[string][]string{
	"path":    {"file", "filename", "file_path", "filepath", "query", "name"},
	"pattern": {"query", "search", "regex", "q", "text"},
	"input":   {"patch", "diff", "content", "text"},
	"task":    {"question", "goal", "prompt", "instruction"},
	"command": {"cmd", "script", "shell"},
	"content": {"text", "body", "data"},
}

func (e envelopeAdapter) FixToolCall(name string, args []byte, offered []ToolShape) (string, []byte, bool) {
	var got map[string]any
	if json.Unmarshal(args, &got) != nil {
		return name, args, false
	}
	shape, ok := shapeOf(name, offered)
	if !ok {
		return name, args, false // a tool nobody offered: no guessing
	}
	// 0. UNWRAP: the declared field carrying the whole argument object again,
	// JSON-encoded inside itself. Measured on xiaomi/mimo-v2.6-flash, which sent
	// input = `{"input": "*** Begin Patch…"}` — the payload was right, the
	// envelope was wrapped twice.
	if inner, ok := unwrapDoubled(got, shape.Fields); ok {
		out, err := json.Marshal(inner)
		if err == nil {
			logs.New("Providers").Info("tool call unwrapped",
				slog.String("adapter", e.Name()), slog.String("tool", name))
			return name, out, true
		}
	}
	if hasAny(got, shape.Fields) {
		return name, args, false // already right: change nothing
	}

	// 1. RENAME: the schema's field, from the alias the model used.
	fixed := map[string]any{}
	for k, v := range got {
		fixed[k] = v
	}
	changed := false
	for _, want := range shape.Fields {
		if _, have := fixed[want]; have {
			continue
		}
		for _, alias := range fieldAliases[want] {
			if v, have := got[alias]; have {
				fixed[want] = v
				delete(fixed, alias)
				changed = true
				break
			}
		}
	}

	// 2. REROUTE: a filename is a file to read, not a pattern to search.
	if v := onlyString(fixed); v != "" && looksLikePath(v) {
		if better, field, ok := fileTool(offered); ok && better != name {
			name, fixed, changed = better, map[string]any{field: v}, true
		}
	}
	if !changed {
		return name, args, false
	}
	out, err := json.Marshal(fixed)
	if err != nil {
		return name, args, false
	}
	logs.New("Providers").Info("tool call repaired",
		slog.String("adapter", e.Name()), slog.String("tool", name),
		slog.String("was", truncateArgs(args)), slog.String("now", truncateArgs(out)))
	return name, out, true
}

// LiftFromText implements rule 3. The only payload lifted is a patch, because a
// patch says what it is: if this harness's own parser accepts the text, the
// model wrote a patch and forgot the envelope. Nothing else is guessed — prose
// that merely mentions a file is left as prose.
func (e envelopeAdapter) LiftFromText(text string, offered []ToolShape) (string, []byte, bool) {
	if strings.TrimSpace(text) == "" {
		return "", nil, false
	}
	field, ok := patchTool(offered)
	if !ok {
		return "", nil, false
	}
	body := patchBody(text)
	if body == "" || !tools.PatchParses(body) {
		return "", nil, false
	}
	args, err := json.Marshal(map[string]string{field: body})
	if err != nil {
		return "", nil, false
	}
	logs.New("Providers").Info("patch lifted out of the reply into a tool call",
		slog.String("adapter", e.Name()), slog.Int("bytes", len(body)))
	return "apply_patch", args, true
}

// patchBody is the patch inside a reply: the whole text when it is a patch, or
// the part from the first patch marker, with a fenced block unwrapped.
func patchBody(text string) string {
	t := text
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		if strings.Contains(rest, "*** ") {
			t = rest
		}
	}
	for _, marker := range []string{"*** Begin Patch", "*** Update File:", "*** Add File:", "*** Delete File:"} {
		if i := strings.Index(t, marker); i >= 0 {
			return strings.TrimSpace(t[i:])
		}
	}
	return ""
}

func shapeOf(name string, offered []ToolShape) (ToolShape, bool) {
	for _, t := range offered {
		if t.Name == name {
			return t, len(t.Fields) > 0
		}
	}
	return ToolShape{}, false
}

// hasAny is whether the model already filled one of the declared fields.
func hasAny(got map[string]any, fields []string) bool {
	for _, f := range fields {
		v, ok := got[f]
		if !ok {
			continue
		}
		if s, isStr := v.(string); !isStr || strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

// fileTool is the offered tool that reads a file by path, and the field to put
// the path in.
func fileTool(offered []ToolShape) (string, string, bool) {
	for _, t := range offered {
		if t.Name != "read_file" && t.Name != "jread" {
			continue
		}
		for _, f := range t.Fields {
			if f == "path" {
				return t.Name, f, true
			}
		}
	}
	return "", "", false
}

// patchTool is the field apply_patch takes its patch in, when it was offered.
func patchTool(offered []ToolShape) (string, bool) {
	for _, t := range offered {
		if t.Name != "apply_patch" {
			continue
		}
		for _, f := range t.Fields {
			if f == "input" || f == "patch" {
				return f, true
			}
		}
		if len(t.Fields) == 1 {
			return t.Fields[0], true
		}
	}
	return "", false
}

// looksLikePath is whether a value names a file rather than a pattern: one
// token, an extension, no regex punctuation.
func looksLikePath(v string) bool {
	if strings.ContainsAny(v, " *?[]()|\\^$") {
		return false
	}
	base := v
	if i := strings.LastIndex(v, "/"); i >= 0 {
		base = v[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	return dot > 0 && dot < len(base)-1
}

// onlyString is the single string value in args, or "" when there are several.
func onlyString(args map[string]any) string {
	if len(args) != 1 {
		return ""
	}
	for _, v := range args {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func truncateArgs(b []byte) string {
	s := string(b)
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// unwrapDoubled finds a declared field whose string value is itself the JSON
// object the arguments should have been, and returns the inner object. Only
// when the inner object declares the SAME field, so this can never turn an
// ordinary string that happens to look like JSON into something else.
func unwrapDoubled(got map[string]any, fields []string) (map[string]any, bool) {
	for _, f := range fields {
		s, ok := got[f].(string)
		if !ok || !strings.HasPrefix(strings.TrimSpace(s), "{") {
			continue
		}
		var inner map[string]any
		if json.Unmarshal([]byte(s), &inner) != nil {
			continue
		}
		if v, ok := inner[f].(string); ok && strings.TrimSpace(v) != "" {
			return inner, true
		}
	}
	return nil, false
}
