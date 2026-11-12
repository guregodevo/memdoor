package gateway

import (
	"encoding/json"
	"fmt"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"memdoor/gateway/providers"
)

// Reasoning is a workflow result only when there is no answer. Neither a
// capped reply nor an empty normal completion triggers a thinking-off retry.
func TestAnLLMTaskKeepsReasoningWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name, text, thinking, finish, want string
		wantErr                            bool
	}{
		{"capped reasoning", "", "  partial plan\n```go\npackage main\n```  ", "length", "partial plan\n```go\npackage main\n```", false},
		{"normal reasoning", "", "  plan  ", "stop", "plan", false},
		{"whitespace answer", " \n ", "plan", "stop", "plan", false},
		{"answer wins", "  final answer  ", "reasoning", "stop", "final answer", false},
		{"capped answer wins", "partial answer", "reasoning", "length", "partial answer", false},
		{"empty normal reply", "", "", "stop", "", true},
		{"empty capped reply", "", "", "length", "", true},
		{"whitespace reply", " \n", " \n", "stop", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("OPEN_ROUTER_API_KEY", "")
			t.Setenv("OPENROUTER_API_KEY", "")
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
					fmt.Fprint(w, `{"data":[{"id":"qwen3:4b"}]}`)
					return
				}
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
					"index": 0, "delta": map[string]any{"content": tc.text, "reasoning_content": tc.thinking},
				}}})
				fmt.Fprintf(w, "data: %s\n\n", chunk)
				done, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
					"index": 0, "delta": map[string]any{}, "finish_reason": tc.finish,
				}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", done)
			}))
			t.Cleanup(srv.Close)
			if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
				t.Fatal(err)
			}
			c := workflowCompleter{s: &Server{clientFactory: providers.NewClientFactory()}}
			answer, err := c.Complete(t.Context(), "thinking high", "make a plan")
			if (err != nil) != tc.wantErr || answer != tc.want {
				t.Fatalf("answer %q, err %v; want %q, error %v", answer, err, tc.want, tc.wantErr)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("%d calls, want exactly one", got)
			}
		})
	}
}

// mark_cutoff on a task's model line starts a reply cut at the cap with
// cutoffMark, an empty one included; a finished reply, or a task without the
// term, is unchanged.
func TestAnLLMTaskMarksAReplyCutAtTheCap(t *testing.T) {
	for _, tc := range []struct {
		name, line, text, thinking, finish, want string
	}{
		{"capped reasoning", "mark_cutoff", "", "partial plan", "length", cutoffMark + "\npartial plan"},
		{"capped answer", "mark_cutoff", "### CODE\nx", "r", "length", cutoffMark + "\n### CODE\nx"},
		{"capped and empty", "mark_cutoff", "", "", "length", cutoffMark},
		{"finished", "mark_cutoff", "### PLAN\nok", "r", "stop", "### PLAN\nok"},
		{"no term", "thinking high", "", "partial plan", "length", "partial plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("OPEN_ROUTER_API_KEY", "")
			t.Setenv("OPENROUTER_API_KEY", "")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
					fmt.Fprint(w, `{"data":[{"id":"qwen3:4b"}]}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
					"index": 0, "delta": map[string]any{"content": tc.text, "reasoning_content": tc.thinking}, "finish_reason": tc.finish,
				}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			}))
			t.Cleanup(srv.Close)
			if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
				t.Fatal(err)
			}
			c := workflowCompleter{s: &Server{clientFactory: providers.NewClientFactory()}}
			answer, err := c.Complete(t.Context(), tc.line, "make a plan")
			if err != nil || answer != tc.want {
				t.Fatalf("answer %q, err %v; want %q", answer, err, tc.want)
			}
		})
	}
}
