package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/providers"
)

// An llm task's answer cut off by the call's clock fails the task: the
// client keeps what streamed (a window shows it), but half a review is not
// a review (2026-10-28: a 4B's ten minutes of review passed as one).
func TestAnLLMTaskAnswerCutOffByItsClockFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := llmTaskCallTimeout
	llmTaskCallTimeout = 300 * time.Millisecond
	t.Cleanup(func() { llmTaskCallTimeout = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			fmt.Fprint(w, `{"data":[{"id":"qwen3:4b"}]}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "## Summary\nThe change"}}}})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
		t.Fatal(err)
	}
	c := workflowCompleter{s: &Server{clientFactory: providers.NewClientFactory()}}
	answer, err := c.Complete(t.Context(), "thinking off", "review this")
	if err == nil || !strings.Contains(err.Error(), "cut off") {
		t.Fatalf("answer %q, err %v: a cut-off answer must fail", answer, err)
	}
}
