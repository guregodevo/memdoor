package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// Plan mode is read-only: a tool that changes or runs something is refused
// with the way out, a tool that only reads runs, and the same calls run in
// the default mode. Before 2026-10-24 the mode gated nothing but the
// exit_plan_mode acknowledgement.
func TestPlanModeRefusesWhatChangesAndLetsReadsThrough(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	writes, reads := 0, 0
	ar.tools = append(ar.tools,
		tools.ToolDefinition{Name: "mcp__linear__create_issue", Description: "makes an issue",
			Function: func(json.RawMessage) (string, error) { writes++; return "issue made", nil }},
		tools.ToolDefinition{Name: "lookup_thing", Description: "reads a thing",
			Function: func(json.RawMessage) (string, error) { reads++; return "found", nil }},
	)
	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	plan := context.WithValue(context.Background(), sharedctx.PermissionModeKey, permissionModePlan)
	inputs := map[string]string{
		"mcp__linear__create_issue": `{"title":"bug"}`,
		"write_file":                `{"path":"a.txt","content":"x"}`,
		"bash":                      `{"command":"ls"}`,
		"sessions_spawn":            `{"task":"look around"}`,
		"lookup_thing":              `{"q":"x"}`,
	}
	call := func(ctx context.Context, name string) ToolExecutionInfo {
		info, _ := ar.executeTool(ctx, &llm.ToolUseBlock{ID: "tu", Name: name, Input: json.RawMessage(inputs[name])}, "run-1", session)
		return info
	}

	for _, name := range []string{"mcp__linear__create_issue", "write_file", "bash", "sessions_spawn"} {
		info := call(plan, name)
		if !strings.HasPrefix(info.Error, "plan mode: "+name) || !strings.Contains(info.Error, "exit_plan_mode") {
			t.Fatalf("%s in plan mode: %q", name, info.Error)
		}
	}
	if writes != 0 {
		t.Fatalf("a refused tool ran %d times", writes)
	}
	if info := call(plan, "lookup_thing"); info.Error != "" || reads != 1 {
		t.Fatalf("a read in plan mode: %q (ran %d)", info.Error, reads)
	}
	if info := call(context.Background(), "mcp__linear__create_issue"); info.Error != "" || writes != 1 {
		t.Fatalf("default mode: %q (ran %d)", info.Error, writes)
	}
}
