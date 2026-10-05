package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	"memdoor/tools"
)

// Greg, 2026-10-02: a company that approves coding agents forbids "the
// default yolo mode". With MEMDOOR_APPROVE=changes a tool that changes or
// runs something waits for the person's answer through the picker; "no"
// ends the call with a result the model can act on, "always" keeps the tool
// quiet for the session, and off means nothing is asked.
func TestApprovalModeAsksBeforeAChange(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	ran := 0
	ar.tools = append(ar.tools, tools.ToolDefinition{Name: "mcp__linear__create_issue", Description: "makes an issue",
		Function: func(json.RawMessage) (string, error) { ran++; return "issue made", nil }})
	t.Setenv("MEMDOOR_APPROVE", "changes")

	// Answer every approval question with the next scripted answer.
	var mu sync.Mutex
	var asked []string
	answers := []string{approveNo, approveAlways}
	unsub := ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream != "question" {
			return
		}
		mu.Lock()
		asked = append(asked, ev.Data["question"].(string))
		next := answers[0]
		answers = answers[1:]
		mu.Unlock()
		go ar.AnswerQuestion(ev.Data["question_id"].(string), next)
	})
	defer unsub()

	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	call := func() (ToolExecutionInfo, llm.ContentBlockParamUnion) {
		return ar.executeTool(context.Background(),
			&llm.ToolUseBlock{ID: "tu", Name: "mcp__linear__create_issue", Input: json.RawMessage(`{"title":"bug"}`)}, "run-1", session)
	}

	info, result := call() // "No"
	if ran != 0 || !strings.Contains(info.Error, "said no") || result.OfToolResult == nil || !result.OfToolResult.IsError {
		t.Fatalf("a refused change must not run and must say so: ran=%d err=%q", ran, info.Error)
	}
	info, _ = call() // "Yes, always"
	if ran != 1 || info.Error != "" || info.Output != "issue made" {
		t.Fatalf("an allowed change runs: ran=%d err=%q out=%q", ran, info.Error, info.Output)
	}
	info, _ = call() // no question this time
	if ran != 2 || info.Error != "" {
		t.Fatalf("after always, no question: ran=%d err=%q", ran, info.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 2 || !strings.Contains(asked[0], "create_issue on linear") || !strings.HasSuffix(asked[0], "allow?") {
		t.Fatalf("asked: %q", asked)
	}

	// A read never asks, and off asks nothing at all.
	if ar.approvalMode() != approveChanges || !needsApproval("bash") || !needsApproval("apply_patch") || !needsApproval("write_file") || needsApproval("read_file") || needsApproval("grep") {
		t.Fatal("bash, patches, writes and MCP tools need approval; reads do not")
	}
	t.Setenv("MEMDOOR_APPROVE", "off")
	if err := ar.askApproval(context.Background(), &llm.ToolUseBlock{Name: "bash", Input: json.RawMessage(`{"command":"rm -rf x"}`)}, "run", session, logs.New("Agent")); err != nil {
		t.Fatalf("off asks nothing: %v", err)
	}
}

// No one answering is a no: a change does not run because nobody was there.
func TestApprovalUnansweredIsANo(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	t.Setenv("MEMDOOR_APPROVE", "changes")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err = ar.askApproval(ctx, &llm.ToolUseBlock{Name: "bash", Input: json.RawMessage(`{"command":"make deploy"}`)}, "run", &Session{ID: "s"}, logs.New("Agent"))
	if err == nil || !strings.Contains(err.Error(), "was not run") {
		t.Fatalf("unanswered must refuse: %v", err)
	}
}

func TestApprovalSummaryNamesTheChange(t *testing.T) {
	for _, c := range []struct{ tool, input, want string }{
		{"bash", `{"command":"go test ./...\n"}`, "Run: go test ./..."},
		{"apply_patch", `{"input":"*** Begin Patch\n*** Update File: a.go\n+x\n*** Add File: b.go\n*** End Patch"}`, "Change: a.go, b.go"},
		{"write_file", `{"path":"cmd/main.go","content":"x"}`, "Write: cmd/main.go"},
		{"mcp__github__create_pr", `{"title":"t"}`, "Call create_pr on github"},
	} {
		if got := approvalSummary(c.tool, json.RawMessage(c.input)); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: %q, want prefix %q", c.tool, got, c.want)
		}
	}
}
