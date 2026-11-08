package gateway

import (
	"context"
	"os"
	"strings"
	"testing"

	"memdoor/gateway/infra"
	"memdoor/pkg/llm"
)

func TestConversationSaveFailureIsVisibleAndRebuildUsesPendingHistory(t *testing.T) {
	sp, session, key := checkpointStore(t)
	ar := &AgentRuntime{persistence: sp, events: infra.NewEventEmitter(false)}
	conversation := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("first"))}
	if err := ar.updateSession(session, conversation); err != nil {
		t.Fatal(err)
	}
	path, _ := sp.resolveSessionStorePath(key)
	if err := os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	warned := false
	ar.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream == infra.EventStreamError {
			warned = ev.RunID == "save-run" && ev.Data["recoverable"] == true && ev.Data["saved"] == false
		}
	})
	conversation = append(conversation, llm.NewAssistantMessage(llm.NewTextBlock("unsaved")))
	err := ar.updateSession(session, conversation, "save-run")
	if err == nil || !strings.Contains(err.Error(), "only in memory") || !warned {
		t.Fatalf("save failure was not visible: warned=%v err=%v", warned, err)
	}
	rebuilt := ar.buildConversation(context.Background(), session)
	if len(rebuilt) != 2 || sessionCursor(session, 10) != 1 {
		t.Fatalf("rebuild discarded unsaved messages or advanced cursor: %v", rebuilt)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".saved", path); err != nil {
		t.Fatal(err)
	}
	if err := ar.updateSession(session, rebuilt); err != nil {
		t.Fatal(err)
	}
	if _, pending := session.pendingConversation(key); pending {
		t.Fatal("recovery did not clear the pending view")
	}
}
