package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

func workflowApprovalRuntime() *AgentRuntime {
	return &AgentRuntime{
		events:    infra.NewEventEmitter(false),
		questions: newQuestionBroker(),
	}
}

func TestWorkflowApprovalDecisions(t *testing.T) {
	for _, tc := range []struct {
		name, mode, action, answer string
		asked, allowed             bool
	}{
		{"deny run", "changes", "run", approveNo, true, false},
		{"allow run", "changes", "run", approveYes, true, true},
		{"deny start alias", "changes", "start", approveNo, true, false},
		{"deny resume", "changes", "resume", approveNo, true, false},
		{"allow resume", "changes", "resume", approveYes, true, true},
		{"deny continue alias", "changes", "continue", approveNo, true, false},
		{"off permits run", "off", "run", "", false, true},
		{"off permits resume", "off", "resume", "", false, true},
		{"gate denied even off", "off", "approve", approveNo, true, false},
		{"gate allowed explicitly", "off", "approve", approveYes, true, true},
		{"gate denied in changes", "changes", "approve", approveNo, true, false},
		{"gate allowed in changes", "changes", "approve", approveYes, true, true},
		{"reject blanket run answer", "changes", "run", approveAlways, true, false},
		{"reject blanket gate answer", "off", "approve", approveAlways, true, false},
		{"reject unrecognized answer", "changes", "run", "yes", true, false},
		{"list stays available", "changes", "list", "", false, true},
		{"status stays available", "changes", "status", "", false, true},
		{"stop stays available", "changes", "stop", "", false, true},
		{"cancel stays available", "changes", "cancel", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MEMDOOR_APPROVE", tc.mode)
			ar := workflowApprovalRuntime()
			// Legacy or manually inserted grants must not authorize a workflow.
			ar.approvals.grant("session", "workflow")
			asked := 0
			ar.events.OnEvent(func(ev infra.AgentEvent) {
				if ev.Stream != "question" {
					return
				}
				asked++
				options, ok := ev.Data["options"].([]string)
				if !ok || len(options) != 2 || options[0] != approveYes || options[1] != approveNo || ev.Data["default"] != approveNo {
					t.Error("workflow approval must offer only Yes and No, defaulting to No")
				}
				question, _ := ev.Data["question"].(string)
				if !strings.Contains(question, "execute commands") || !strings.Contains(question, "change files") {
					t.Errorf("approval omits delegated effects: %q", question)
				}
				ar.questions.answer(ev.Data["question_id"].(string), tc.answer)
			})
			input, err := json.Marshal(map[string]string{
				"action": tc.action, "name": "release", "run_id": "r1", "task": "review",
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = ar.askApproval(ctx, &llm.ToolUseBlock{Name: "workflow", Input: input}, "run", &Session{ID: "session"}, logs.New("Agent"))
			if (asked > 0) != tc.asked || (err == nil) != tc.allowed {
				t.Fatalf("asked=%d err=%v; want asked=%v allowed=%v", asked, err, tc.asked, tc.allowed)
			}
			if len(ar.questions.pending) != 0 {
				t.Fatal("approval left a pending question")
			}
		})
	}
}

func TestWorkflowApprovalIsFreshForEveryInvocation(t *testing.T) {
	t.Setenv("MEMDOOR_APPROVE", "changes")
	ar := workflowApprovalRuntime()
	asked := 0
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == "question" {
			asked++
			ar.questions.answer(ev.Data["question_id"].(string), approveYes)
		}
	})
	for _, action := range []string{"run", "run", "resume", "approve", "approve"} {
		input, _ := json.Marshal(map[string]string{"action": action, "name": "release", "run_id": "r1", "task": "review"})
		if err := ar.askApproval(context.Background(), &llm.ToolUseBlock{Name: "workflow", Input: input}, "run", &Session{ID: "session"}, logs.New("Agent")); err != nil {
			t.Fatal(err)
		}
	}
	if asked != 5 || ar.approvals.granted("session", "workflow") {
		t.Fatalf("workflow authority leaked across calls: asked=%d", asked)
	}
}

func TestWorkflowApprovalUnansweredOrCanceledCannotProceed(t *testing.T) {
	for _, action := range []string{"run", "resume", "approve"} {
		t.Run(action, func(t *testing.T) {
			t.Setenv("MEMDOOR_APPROVE", "changes")
			ar := workflowApprovalRuntime()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			input, _ := json.Marshal(map[string]string{"action": action, "name": "release", "run_id": "r1", "task": "review"})
			err := ar.askApproval(ctx, &llm.ToolUseBlock{Name: "workflow", Input: input}, "run", &Session{ID: "session"}, logs.New("Agent"))
			if err == nil || len(ar.questions.pending) != 0 {
				t.Fatalf("unanswered approval proceeded or leaked: %v", err)
			}
		})
	}
}

func TestWorkflowApprovalCancellationWinsOverAnAnswer(t *testing.T) {
	t.Setenv("MEMDOOR_APPROVE", "changes")
	ar := workflowApprovalRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == "question" {
			ar.questions.answer(ev.Data["question_id"].(string), approveYes)
			cancel()
		}
	})
	err := ar.askApproval(ctx, &llm.ToolUseBlock{Name: "workflow", Input: json.RawMessage(`{"action":"run","name":"release"}`)}, "run", &Session{ID: "session"}, logs.New("Agent"))
	if err == nil {
		t.Fatal("canceled approval accepted a buffered Yes")
	}
}

func TestWorkflowApprovalUsesWorkspacePolicyAndEnvironmentOverride(t *testing.T) {
	ar := workflowApprovalRuntime()
	ar.workspaceSetting = func(string) string { return "changes" }
	t.Setenv("MEMDOOR_APPROVE", "")
	asked := 0
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == "question" {
			asked++
			ar.questions.answer(ev.Data["question_id"].(string), approveNo)
		}
	})
	call := &llm.ToolUseBlock{Name: "workflow", Input: json.RawMessage(`{"action":"run","name":"release"}`)}
	if err := ar.askApproval(context.Background(), call, "run", &Session{ID: "session"}, logs.New("Agent")); err == nil || asked != 1 {
		t.Fatal("workspace approval mode was not enforced")
	}
	t.Setenv("MEMDOOR_APPROVE", "off")
	if err := ar.askApproval(context.Background(), call, "run", &Session{ID: "session"}, logs.New("Agent")); err != nil || asked != 1 {
		t.Fatal("environment did not override workspace execution policy")
	}
}

func TestWorkflowApprovalDoesNotChangeDirectToolGrants(t *testing.T) {
	t.Setenv("MEMDOOR_APPROVE", "changes")
	ar := workflowApprovalRuntime()
	asked := 0
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == "question" {
			asked++
			ar.questions.answer(ev.Data["question_id"].(string), approveAlways)
		}
	})
	call := &llm.ToolUseBlock{Name: "bash", Input: json.RawMessage(`{"command":"go test ./..."}`)}
	for i := 0; i < 2; i++ {
		if err := ar.askApproval(context.Background(), call, "run", &Session{ID: "session"}, logs.New("Agent")); err != nil {
			t.Fatal(err)
		}
	}
	if asked != 1 || !ar.approvals.granted("session", "bash") {
		t.Fatal("existing direct tool session grant changed")
	}
}
