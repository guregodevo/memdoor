package gateway

import (
	"errors"
	"testing"

	"memdoor/gateway/infra"
	"memdoor/pkg/llm"
)

func TestInterruptedAnswerIsRetainedWithoutLifecycleSuccess(t *testing.T) {
	sp, _ := boundaryStore(t)
	ar := &AgentRuntime{persistence: sp, events: infra.NewEventEmitter()}
	session := &Session{ID: "interrupted", Metadata: map[string]interface{}{}}
	response := &AgentResponse{}
	completions := 0
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == infra.EventStreamLifecycle && ev.Data["event"] == "complete" {
			completions++
			if ev.Data["success"] != false || ev.Data["interrupted"] != true {
				t.Errorf("interruption emitted success: %+v", ev.Data)
			}
		}
	})
	partial := &llm.Message{Role: "assistant", StopReason: llm.StopReasonInterrupted,
		Content: []llm.ContentBlockUnion{{Type: "text", Text: "partial answer"}}}
	ar.keepInterruptedAnswer("run", session, []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("question"))},
		partial, response, errors.New("response interrupted"))
	messages, err := sp.LoadMessages(session.ID)
	if err != nil || len(messages) != 2 || response.Text != "partial answer" || completions != 1 {
		t.Fatalf("partial answer not retained: messages=%v text=%q completions=%d err=%v", messages, response.Text, completions, err)
	}
}
