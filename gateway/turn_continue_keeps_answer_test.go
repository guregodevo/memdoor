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

	"memdoor/gateway/providers"
	"memdoor/pkg/decision"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// A TURN THE JUDGE SENDS BACK KEEPS ITS ANSWER. Live 2026-10-04 ("it cuts the
// window again"): a read-only review ended on a full answer, the decision
// model judged the turn unfinished (nothing changed), and the continue path
// cut that answer from the reply. The window had already streamed it, the
// final text no longer started with it, and the window replaced the whole
// review with the receipt line; the answer was not in the session either.
// Mutation checks: put back the TrimSuffix on response.Text in the continue
// branch and the answer is gone from the reply; append the retry's message
// without withLeadingText and it is gone from the saved conversation.
func TestATurnSentBackKeepsItsAnswer(t *testing.T) {
	// The finished answer misread as an announcement of work (the live case:
	// a review ending on its "next action"), and the done check judging a
	// read-only turn unfinished. Either sends the turn back; neither may cut
	// what the window already showed.
	t.Run("misread as an announcement", func(t *testing.T) {
		keepsItsAnswer(t, map[string]float64{"announces": 0.95})
	})
	t.Run("judged unfinished", func(t *testing.T) {
		keepsItsAnswer(t, map[string]float64{"unfinished": 0.95})
	})
}

// byQuestion answers each decision question with its own probability, 0.05
// for any it does not name.
type byQuestion map[string]float64

func (b byQuestion) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	answers := map[string]decision.Answer{}
	for id := range req.Questions {
		p, ok := b[id]
		if !ok {
			p = 0.05
		}
		answers[id] = decision.Answer{Kind: decision.KindBoolean, ProbabilityTrue: p}
	}
	return decision.Result{Status: decision.StatusOK, Answers: answers}
}

func keepsItsAnswer(t *testing.T, judge map[string]float64) {
	t.Setenv("HOME", t.TempDir())
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	ar.tools = append(ar.tools, tools.ToolDefinition{Name: "peek", Description: "reads a thing",
		Function: func(json.RawMessage) (string, error) { return "all five fixed", nil }})
	ar.SetTurnVerdict(&turnVerdict{svc: byQuestion(judge), read: func(string) string { return "" }})

	const answer = "REVIEW-ANSWER: all five are fixed. Verdict: clean."
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		switch n {
		case 1, 3: // a read; the third is the forced call after the verdict
			send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant",
				"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("c%d", n), "type": "function",
					"function": map[string]any{"name": "peek", "arguments": `{}`}}}}}}})
			send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}}})
		case 2:
			send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": answer}, "finish_reason": "stop"}}})
		default:
			send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Noted."}, "finish_reason": "stop"}}})
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "openrouter", API: providers.APIOpenRouter, Base: srv.URL, Key: "sk-test"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: providers.ByokEngineName, Endpoint: srv.URL + "/chat/completions",
		Model: "z-ai/glm-5.3-flash", APIKey: "sk-test", Byok: true, CtxLen: 262_144}); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, []string{"read_file", "peek"})
	ctx = context.WithValue(ctx, sharedctx.WorkdirKey, t.TempDir())
	ctx = context.WithValue(ctx, "buddy_agent_name", "coder") //nolint:staticcheck // the key the agent runtime reads
	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	resp, err := ar.ProcessMessage(ctx, "review the last commit", session, "run-keep", "")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	n := calls
	mu.Unlock()
	if n < 3 {
		t.Fatalf("the judge must have sent the turn back (a forced round): %d model calls", n)
	}
	if !strings.Contains(resp.Text, answer) {
		t.Fatalf("the answer the window already showed must stay in the reply:\n%s", resp.Text)
	}
	// The conversation keeps it too, or the next turn ("fix those") asks
	// about an answer the model no longer has.
	if ar.persistence == nil {
		t.Skip("no persistence in this runtime")
	}
	saved, err := ar.persistence.LoadRecentMessages(transcriptKeyOf(session), wholeTranscript)
	if err != nil {
		t.Fatal(err)
	}
	kept := false
	for _, m := range saved {
		for _, b := range m.Content {
			if b.OfText != nil && strings.Contains(b.OfText.Text, answer) {
				kept = true
			}
		}
	}
	if !kept {
		t.Fatalf("the saved conversation must keep the answer (%d messages)", len(saved))
	}
}
