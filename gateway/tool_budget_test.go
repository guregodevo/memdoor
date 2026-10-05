package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every tool definition rides on every request of a turn, cached or not: the
// coder's are held to a budget, so a new tool or a longer description is a
// choice, not a drift (2026-09-29: 4,285 → 3,374 tokens by dropping jlogs and
// cron and hiding fields the model never sets).
func TestTheCodersToolboxStaysInBudget(t *testing.T) {
	const budgetTokens = 3_600
	t.Setenv("HOME", t.TempDir())
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}
	var palette []string
	if err := json.Unmarshal([]byte(coderToolPalette), &palette); err != nil {
		t.Fatal(err)
	}
	in := map[string]bool{}
	for _, p := range palette {
		in[p] = true
	}
	total := 0
	for _, d := range ar.GetTools() {
		if !in[d.Name] {
			continue
		}
		b, _ := json.Marshal(map[string]any{"name": d.Name, "description": d.Description, "input_schema": d.InputSchema})
		total += len(b) / 4
		// Fields the harness sets are never offered to the model.
		props, _ := json.Marshal(d.InputSchema)
		for _, harness := range []string{`"conversation"`, `"rules_file"`, `"transcript"`} {
			if strings.Contains(string(props), harness) {
				t.Errorf("%s offers the harness field %s", d.Name, harness)
			}
		}
	}
	if total > budgetTokens {
		t.Fatalf("the coder's tools are ~%d tokens on every request, over the %d budget", total, budgetTokens)
	}
}
