package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/infra"
	"memdoor/pkg/llm"
	"memdoor/pkg/shared"
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
	info, _ := ar.executeTool(context.Background(),
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
