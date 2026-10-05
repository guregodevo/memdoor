package gateway

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// notTools are snake_case words that appear in a prompt and are prose, not
// tool names. Kept explicit and small: the burden is on a NEW tool-shaped word
// to justify itself, which is what makes this test catch drift instead of
// needing to be updated every time a tool is added.
var notTools = map[string]bool{
	"in_progress": true, // todo_write's status value
	"tool_call":   true, // the wire format's tag, not a tool
}

// A seeded agent's prompt must not advertise a tool it cannot call.
//
// Nothing catches that at build time — a prompt is a string — and the cost is
// not cosmetic. The model is invited to use the tool, tries, and gets an error,
// burning a round trip each time; before the retire-on-repeat fix it did so
// indefinitely.
//
// Found 2026-08-29 on a live turn: the coder's prompt offered memory tools
// ("your durable memory across sessions") and none of them was in its palette.
// It named them in two further places besides the
// tools list, which is exactly why this is a test and not a careful read —
// the first fix by hand missed both.
func TestCoderPromptOnlyAdvertisesCallableTools(t *testing.T) {
	var palette []string
	if err := json.Unmarshal([]byte(coderToolPalette), &palette); err != nil {
		t.Fatalf("the palette must be valid JSON — it goes straight into the buddies row: %v", err)
	}
	callable := map[string]bool{}
	for _, name := range palette {
		callable[name] = true
	}

	snake := regexp.MustCompile(`\b[a-z]+_[a-z_]+\b`)
	found := snake.FindAllString(coderSystemPrompt, -1)
	if len(found) == 0 {
		t.Fatal("premise: the prompt contains no tool-shaped words at all — the pattern is wrong, not the prompt")
	}

	seen := map[string]bool{}
	var advertised []string
	for _, w := range found {
		if seen[w] || callable[w] || notTools[w] {
			continue
		}
		seen[w] = true
		advertised = append(advertised, w)
	}
	if len(advertised) > 0 {
		t.Errorf("the coder's prompt names tools it cannot call: %v\n"+
			"the model will try them and fail every time — add them to coderToolPalette, "+
			"stop naming them, or list them in notTools if they are prose",
			advertised)
	}
}

// The palette must be callable in the first place: a name with a typo is a tool
// the agent silently does not have.
func TestCoderPaletteIsWellFormed(t *testing.T) {
	var palette []string
	if err := json.Unmarshal([]byte(coderToolPalette), &palette); err != nil {
		t.Fatal(err)
	}
	if len(palette) == 0 {
		t.Fatal("an empty palette gives the coder no tools at all")
	}
	for _, n := range palette {
		if strings.TrimSpace(n) != n || n == "" {
			t.Errorf("malformed tool name %q", n)
		}
	}
	// The two that must always be there: it edits and it verifies.
	for _, required := range []string{"apply_patch", "bash"} {
		if !strings.Contains(coderToolPalette, `"`+required+`"`) {
			t.Errorf("the coder cannot work without %q", required)
		}
	}
}

// The mirror case reports rather than fails: a callable tool the prompt never
// names is one the model is unlikely to reach for, but that can be deliberate.
func TestCoderPaletteToolsAreMentioned(t *testing.T) {
	var palette []string
	_ = json.Unmarshal([]byte(coderToolPalette), &palette)
	var silent []string
	for _, name := range palette {
		if !strings.Contains(coderSystemPrompt, name) {
			silent = append(silent, name)
		}
	}
	if len(silent) > 0 {
		t.Logf("callable but never named in the prompt (may be deliberate): %v", silent)
	}
}
