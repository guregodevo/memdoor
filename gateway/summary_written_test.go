package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"memdoor/gateway/compaction"
	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
)

// fakeOpenRouter answers every chat request with reply and records what the
// model was asked.
func fakeOpenRouter(t *testing.T, reply string) *[]string {
	t.Helper()
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content any `json:"content"`
			} `json:"messages"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		mu.Lock()
		asked = append(asked, "model="+body.Model+" "+fmt.Sprint(body.Messages))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": reply}}}})
		fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":9}}\n\ndata: [DONE]\n\n", chunk)
	}))
	t.Cleanup(srv.Close)
	// Every request goes through a provider: OpenRouter connected as
	// memdoor connect keeps it, its base this fake.
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "openrouter", API: providers.APIOpenRouter, Base: srv.URL, Key: "sk-test"}); err != nil {
		t.Fatal(err)
	}
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: providers.ByokEngineName, Endpoint: srv.URL + "/chat/completions",
		Model: "z-ai/glm-5.3-flash", APIKey: "sk-test", Byok: true, CtxLen: 262_144}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	return &asked
}

func summaryRuntime(t *testing.T) (*AgentRuntime, *SessionPersistence, string) {
	t.Helper()
	sp, key := boundaryStore(t) // sets HOME
	c, err := compaction.NewCompactor("default", false)
	if err != nil {
		t.Fatal(err)
	}
	return &AgentRuntime{persistence: sp, compactor: c, clientFactory: providers.NewClientFactory()}, sp, key
}

func longConversation(n int) []llm.MessageParam {
	var conv []llm.MessageParam
	for i := 0; i < n; i++ {
		conv = append(conv, user(fmt.Sprintf("q%d %s", i, strings.Repeat("x", 3000))), asst(fmt.Sprintf("a%d", i)))
	}
	return conv
}

// /compact has the answering model write the summary, with the focus in
// what it is asked and at the head of what it wrote.
func TestCompactNowWritesTheSummaryWithTheModel(t *testing.T) {
	ar, sp, key := summaryRuntime(t)
	asked := fakeOpenRouter(t, "Goal: the API. Decided: REST. Next: tests.")
	if err := sp.SaveMessages(key, longConversation(30)); err != nil {
		t.Fatal(err)
	}
	res, err := ar.CompactNow(t.Context(), key, "coder", "keep the API decisions")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Summarized || !res.Written {
		t.Fatalf("a written summary: %+v", res)
	}
	if len(*asked) != 1 || !strings.Contains((*asked)[0], "focus on this: keep the API decisions") {
		t.Fatalf("one call, the focus in it: %v", len(*asked))
	}
	sent, _ := sp.LoadRecentMessages(key, wholeTranscript)
	head := messageText(sent[0])
	for _, want := range []string{"FOCUS (asked for with /compact", "written by the model", "Decided: REST"} {
		if !strings.Contains(head, want) {
			t.Fatalf("missing %q in:\n%.400s", want, head)
		}
	}
}

// /handoff: the model writes the handoff; the next conversation starts from
// it, the old one gone.
func TestHandoffStartsTheNextConversationFromIt(t *testing.T) {
	ar, sp, key := summaryRuntime(t)
	asked := fakeOpenRouter(t, "Goal: X.\nNext steps: 1. write the test.")
	if err := sp.SaveMessages(key, longConversation(4)); err != nil {
		t.Fatal(err)
	}
	handoff, err := ar.Handoff(t.Context(), key, "coder")
	if err != nil || !strings.Contains(handoff, "Next steps") {
		t.Fatalf("%q %v", handoff, err)
	}
	if !strings.Contains((*asked)[0], "Write a handoff") {
		t.Fatal("the model is asked for a handoff")
	}
	if err := sp.DeleteSession(key); err != nil {
		t.Fatal(err)
	}
	if err := ar.StartFrom(key, handoffHead+handoff); err != nil {
		t.Fatal(err)
	}
	sent, _ := sp.LoadRecentMessages(key, wholeTranscript)
	if len(sent) != 1 || !strings.HasPrefix(messageText(sent[0]), "HANDOFF FROM THE PREVIOUS SESSION") {
		t.Fatalf("the next conversation starts from the handoff: %s", texts(sent))
	}
}

// A summary is written by the first, cheapest rung, whatever the
// conversation is pinned to: a pinned expensive model wrote summaries at its
// own price (2026-09-29).
func TestASummaryIsWrittenByTheCheapestRung(t *testing.T) {
	ar, sp, key := summaryRuntime(t)
	asked := fakeOpenRouter(t, "Goal: X.")
	if err := sp.SaveMessages(key, longConversation(30)); err != nil {
		t.Fatal(err)
	}
	pinned := context.WithValue(context.WithValue(t.Context(), sharedctx.ModelKey, "anthropic/claude-opus-5"), sharedctx.TierKey, 2)
	if _, err := ar.CompactNow(pinned, key, "coder", ""); err != nil {
		t.Fatal(err)
	}
	if len(*asked) != 1 || !strings.HasPrefix((*asked)[0], "model=z-ai/glm-5.3-flash ") {
		t.Fatalf("the cheapest rung writes it: %.60s", (*asked)[0])
	}
}
