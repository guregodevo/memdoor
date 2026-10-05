package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// Nothing we SHOW the model may contain another vendor's tool-call dialect.
//
// A model repeatedly reached for Anthropic's format — <invoke>, </anthropic> —
// and locked into repeating it until the token cap (2026-08-30). The obvious
// suspicion was that we had put it there ourselves: the prompt was written with
// Claude's help, and a model imitates what it is shown.
//
// It had not. This pins that: the tool payload is what the model actually sees,
// and if a foreign dialect ever appears in a description or schema the model
// will copy it, so the check belongs in CI rather than in someone's memory.
// (The system prompt and the seeded skills are clean too, checked by grep.)
//
// Everything the model is actually SENT: tool names, descriptions, schemas.
func TestNoForeignDialectInWhatTheModelSees(t *testing.T) {
	var sent strings.Builder
	for _, d := range []ToolDefinition{
		BashDefinition, ReadFileDefinition, ApplyPatchDefinition, GrepDefinition,
		GlobDefinition, TodoWriteDefinition, TodoReadDefinition, LocateDefinition,
		SkillDefinition, AskUserQuestionDefinition,
	} {
		sent.WriteString(d.Name + "\n" + d.Description + "\n")
		b, _ := json.Marshal(d.InputSchema)
		sent.Write(b)
		sent.WriteString("\n")
	}
	payload := strings.ToLower(sent.String())
	for _, bad := range []string{"anthropic", "<invoke", "function_calls", "function_results", "dsml"} {
		if strings.Contains(payload, bad) {
			t.Errorf("the model is being SHOWN %q — it would imitate it", bad)
		}
	}
	t.Logf("scanned %d bytes of tool payload, clean", len(payload))
}
