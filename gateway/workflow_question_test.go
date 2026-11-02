package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/infra"
	"memdoor/pkg/llm"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
)

// recordingScreen keeps every frame a window was sent.
type recordingScreen struct {
	id     string
	mu     sync.Mutex
	frames []map[string]interface{}
}

func (r *recordingScreen) GetID() string { return r.id }
func (r *recordingScreen) Send(b []byte) error {
	var f map[string]interface{}
	_ = json.Unmarshal(b, &f)
	r.mu.Lock()
	r.frames = append(r.frames, f)
	r.mu.Unlock()
	return nil
}

func (r *recordingScreen) workflowEvents() []map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []map[string]interface{}
	for _, f := range r.frames {
		if f["type"] != "workflow_event" {
			continue
		}
		data, _ := f["data"].(map[string]interface{})
		if inner, ok := data["data"].(map[string]interface{}); ok {
			data = inner
		}
		out = append(out, data)
	}
	return out
}

// A step's ask_user_question reaches the project's windows as a workflow_event
// with the id the answer goes back by, and the answer closes it everywhere.
func TestAWorkflowStepsQuestionReachesTheWindow(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	sm := broadcast.NewSubscriptionManager(false)
	window := &recordingScreen{id: "win"}
	sm.AddSubscriber(window)
	sm.SubscribeToSession(window.id, shared.NewChannelSessionID("ws-a", "chan"))
	s := &Server{broadcaster: sm, agent: ar}
	run := &workflowRun{ID: "r1", Workflow: "release", Partition: "p1", Dir: "/proj", Workspace: "ws-a"}
	defer s.listen(run)()

	unsub := ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == workflowQuestion {
			go ar.AnswerQuestion(ev.Data["question_id"].(string), "B")
		}
	})
	defer unsub()

	session := &Session{ID: run.taskSession("plan"), Metadata: map[string]interface{}{}}
	info, _ := ar.executeTool(context.WithValue(context.Background(), sharedctx.AttendedKey, true),
		&llm.ToolUseBlock{ID: "tu", Name: "ask_user_question", Input: json.RawMessage(`{"question":"Which?","options":["A","B"]}`)}, "run-1", session)
	if info.Error != "" || info.Output != `{"question":"Which?","selected":"B"}` {
		t.Fatalf("the answer must reach the step: err=%q out=%q", info.Error, info.Output)
	}

	evs := window.workflowEvents()
	if len(evs) != 2 {
		t.Fatalf("want the question and its answer, got %v", evs)
	}
	q, done := evs[0], evs[1]
	opts, _ := q["options"].([]interface{})
	if q["state"] != "question" || q["task"] != "plan" || q["text"] != "Which?" || len(opts) != 2 || q["question_id"] == "" || q["dir"] != "/proj" {
		t.Fatalf("question event: %v", q)
	}
	if done["state"] != "answered" || done["question_id"] != q["question_id"] {
		t.Fatalf("answered event: %v", done)
	}
}

// askAfter asks one question with the timeout at 50ms and answers it after
// delay.
func askAfter(t *testing.T, ctx context.Context, sessionID string, delay time.Duration) ToolExecutionInfo {
	t.Helper()
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	old := questionTimeout
	questionTimeout = 50 * time.Millisecond
	t.Cleanup(func() { questionTimeout = old })
	unsub := ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == workflowQuestion || ev.Stream == "question" {
			id, _ := ev.Data["question_id"].(string)
			go func() { time.Sleep(delay); ar.AnswerQuestion(id, "B") }()
		}
	})
	defer unsub()
	session := &Session{ID: sessionID, Metadata: map[string]interface{}{}}
	info, _ := ar.executeTool(ctx, &llm.ToolUseBlock{ID: "tu", Name: "ask_user_question", Input: json.RawMessage(`{"question":"Which?","options":["A","B"]}`)}, "run-1", session)
	return info
}

// A step of an unattended run gives up at the timeout. A step of a window's run
// waits for the answer.
func TestAWorkflowStepsQuestionWaitsOnlyWhenTheRunIsAttended(t *testing.T) {
	step := (&workflowRun{Workflow: "release", Partition: "p1"}).taskSession("plan")

	unattended := askAfter(t, context.Background(), step, 400*time.Millisecond)
	if unattended.Error != "no answer received within 5 minutes" {
		t.Fatalf("an unattended run's question must time out: err=%q out=%q", unattended.Error, unattended.Output)
	}
	attended := askAfter(t, context.WithValue(context.Background(), sharedctx.AttendedKey, true), step, 400*time.Millisecond)
	if attended.Error != "" || attended.Output != `{"question":"Which?","selected":"B"}` {
		t.Fatalf("an attended run's question must wait for the answer: err=%q out=%q", attended.Error, attended.Output)
	}
}

// A turn outside a workflow keeps the timeout.
func TestAWindowTurnsQuestionKeepsItsTimeout(t *testing.T) {
	info := askAfter(t, context.WithValue(context.Background(), sharedctx.AttendedKey, true), "chan:ws-a:chat", 400*time.Millisecond)
	if info.Error != "no answer received within 5 minutes" {
		t.Fatalf("a turn outside a workflow keeps the timeout: err=%q", info.Error)
	}
}

// A run is attended when a window started it.
func TestARunIsAttendedWhenAWindowStartedIt(t *testing.T) {
	if (&workflowRun{}).attended() {
		t.Fatal("a run with no session (workflow run, cron, CI) is unattended")
	}
	if !(&workflowRun{Session: shared.NewChannelSessionID("ws-a", "chan")}).attended() {
		t.Fatal("a run started from a window's conversation is attended")
	}
}

// The shell, the Action and cron start runs with no session.
func TestARunStartedWithoutASessionIsUnattended(t *testing.T) {
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)
	s := &Server{}
	cli, err := s.startWorkflow(dir, "release", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cli.attended() {
		t.Fatal("a run started without a session must be unattended")
	}
	s.stopWorkflow(cli.ID)
}
