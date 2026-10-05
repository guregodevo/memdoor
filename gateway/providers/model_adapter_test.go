package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

var probedTools = []ToolShape{
	{Name: "read_file", Fields: []string{"path"}},
	{Name: "grep", Fields: []string{"pattern"}},
	{Name: "apply_patch", Fields: []string{"input"}},
}

// THE CALL GEMINI ACTUALLY MADE (probed 2026-09-27): grep with
// {"query":"cache.go"} when the turn offered read_file(path) and grep(pattern).
// A model that cannot be repaired cannot be integrated, which is the whole
// point of the adapters (Greg: "gemini should work").
func TestGeminiCallIsRepaired(t *testing.T) {
	a := AdapterFor("google/gemini-3.8-flash")
	if a.Name() != "envelope" {
		t.Fatalf("one generic adapter, not one per vendor: got %q", a.Name())
	}
	name, args, changed := a.FixToolCall("grep", []byte(`{"query":"cache.go"}`), probedTools)
	if !changed {
		t.Fatal("the measured call must be repaired")
	}
	if name != "read_file" {
		t.Errorf("a filename means the file tool, got %q", name)
	}
	var got map[string]string
	if err := json.Unmarshal(args, &got); err != nil {
		t.Fatalf("repaired arguments must be JSON: %v", err)
	}
	if got["path"] != "cache.go" {
		t.Errorf("the value must survive under the schema's name: %v", got)
	}
}

// An argument named wrongly but for the right tool is renamed, not re-routed.
func TestGeminiRenamesWithoutReRouting(t *testing.T) {
	a := AdapterFor("google/gemini-3.8-flash")
	name, args, changed := a.FixToolCall("grep", []byte(`{"query":"func .*Store"}`), probedTools)
	if !changed || name != "grep" {
		t.Fatalf("a pattern belongs to grep: %q changed=%v", name, changed)
	}
	var got map[string]string
	_ = json.Unmarshal(args, &got)
	if got["pattern"] != "func .*Store" {
		t.Errorf("query → pattern: %v", got)
	}
}

// NEGATIVE PATHS. A repair that fires when nothing is wrong is worse than no
// repair: it would rewrite calls from the nine models that honour the schema.
func TestAdaptersLeaveGoodCallsAlone(t *testing.T) {
	for _, model := range []string{"google/gemini-3.8-flash", "z-ai/glm-5.3", "anthropic/claude-sonnet-5"} {
		a := AdapterFor(model)
		for _, call := range []struct{ name, args string }{
			{"read_file", `{"path":"cache.go"}`},
			{"grep", `{"pattern":"Store"}`},
			{"apply_patch", `{"input":"*** Begin Patch\n*** Update File: a.go\n"}`},
		} {
			name, args, changed := a.FixToolCall(call.name, []byte(call.args), probedTools)
			if changed || name != call.name || string(args) != call.args {
				t.Errorf("%s: a correct %s call was rewritten to %s %s", model, call.name, name, args)
			}
		}
	}
	// Unparseable arguments, or a tool nobody offered: no guessing.
	g := AdapterFor("google/gemini-3.8-flash")
	if _, _, changed := g.FixToolCall("grep", []byte(`not json`), probedTools); changed {
		t.Error("arguments that do not parse must be left for the turn to report")
	}
	if _, _, changed := g.FixToolCall("invented_tool", []byte(`{"query":"x"}`), probedTools); changed {
		t.Error("a tool that was never offered must not be repaired into one")
	}
}

// The shapes come from the schema that was actually sent, required field first.
func TestToolShapesReadTheSchema(t *testing.T) {
	var tool chatTool
	tool.Function.Name = "read_file"
	tool.Function.Parameters = map[string]any{
		"properties": map[string]any{"path": map[string]any{}, "all": map[string]any{}},
		"required":   []any{"path"},
	}
	shapes := toolShapes([]chatTool{tool})
	if len(shapes) != 1 || shapes[0].Name != "read_file" || shapes[0].Fields[0] != "path" {
		t.Fatalf("required field first: %+v", shapes)
	}
	if len(shapes[0].Fields) != 2 {
		t.Errorf("every declared field is known: %+v", shapes[0].Fields)
	}
}

// THE PATCH LLAMA ACTUALLY WROTE (probed 2026-09-27): a correct Codex patch in
// the reply text, with no tool call at all. Same failure family as Gemini's —
// the right payload in the wrong envelope — so the same adapter lifts it.
func TestAPatchWrittenAsProseIsLifted(t *testing.T) {
	const written = `*** Update File: cache.go
@@ package cache
+// Store keeps values in memory.
 type Store struct{}
*** End Patch`
	a := AdapterFor("meta-llama/llama-4-maverick")
	name, args, ok := a.LiftFromText("Here is the change:\n\n"+written, probedTools)
	if !ok {
		t.Fatal("a patch our parser accepts must become a call")
	}
	if name != "apply_patch" {
		t.Errorf("lifted into the wrong tool: %q", name)
	}
	var got map[string]string
	if err := json.Unmarshal(args, &got); err != nil {
		t.Fatalf("arguments must be JSON: %v", err)
	}
	if !strings.Contains(got["input"], "*** Update File: cache.go") {
		t.Errorf("the patch must arrive intact: %q", got["input"])
	}

	// A fenced block is unwrapped, because models fence code.
	if _, _, ok := a.LiftFromText("```patch\n"+written+"\n```", probedTools); !ok {
		t.Error("a fenced patch must lift too")
	}
}

// NEGATIVE: lifting must never invent a call. Prose about a file is prose, a
// broken patch stays broken, and nothing is lifted when the tool was not
// offered — otherwise a model would be made to edit files it never asked to.
func TestLiftingRefusesEverythingElse(t *testing.T) {
	a := AdapterFor("meta-llama/llama-4-maverick")
	for name, text := range map[string]string{
		"prose":         "I would add a doc comment above type Store in cache.go.",
		"unified diff":  "--- a/cache.go\n+++ b/cache.go\n@@ -1 +1,2 @@\n+// Store keeps values.\n",
		"half a patch":  "*** Update File:",
		"empty":         "   ",
		"code, no diff": "```go\ntype Store struct{}\n```",
	} {
		if _, _, ok := a.LiftFromText(text, probedTools); ok {
			t.Errorf("%s must not become a tool call: %q", name, text)
		}
	}
	// apply_patch not on offer: nothing to lift into.
	readOnly := []ToolShape{{Name: "read_file", Fields: []string{"path"}}}
	patch := "*** Update File: cache.go\n@@ package cache\n+// c\n type Store struct{}\n*** End Patch"
	if _, _, ok := a.LiftFromText(patch, readOnly); ok {
		t.Error("a turn that offered no patch tool must not get a patch call")
	}
}

// THE ARGUMENTS MIMO ACTUALLY SENT (probed 2026-09-27): the declared field
// carrying the whole argument object again, JSON-encoded inside itself. Right
// payload, envelope wrapped twice — the same family as the other two.
func TestDoubleEncodedArgumentsAreUnwrapped(t *testing.T) {
	a := AdapterFor("xiaomi/mimo-v2.6-flash")
	patch := "*** Begin Patch\n*** Update File: cache.go\n@@ package cache\n+// Store keeps values.\n type Store struct{}\n*** End Patch"
	inner, err := json.Marshal(map[string]string{"input": patch})
	if err != nil {
		t.Fatal(err)
	}
	doubled, err := json.Marshal(map[string]string{"input": string(inner)})
	if err != nil {
		t.Fatal(err)
	}
	name, args, changed := a.FixToolCall("apply_patch", doubled, probedTools)
	if !changed || name != "apply_patch" {
		t.Fatalf("the doubled envelope must be unwrapped: %q changed=%v", name, changed)
	}
	var got map[string]string
	if err := json.Unmarshal(args, &got); err != nil {
		t.Fatal(err)
	}
	if got["input"] != patch {
		t.Errorf("the patch must come out intact: %q", got["input"])
	}

	// NEGATIVE: a value that merely starts with a brace is not an envelope, and
	// an inner object about something else is not unwrapped.
	for name, argv := range map[string]string{
		"json content":  `{"input":"{\"not\":\"a patch\"}"}`,
		"brace in text": `{"input":"{ this is just text }"}`,
	} {
		if _, _, changed := a.FixToolCall("apply_patch", []byte(argv), probedTools); changed {
			t.Errorf("%s must be left alone: %s", name, argv)
		}
	}
}
