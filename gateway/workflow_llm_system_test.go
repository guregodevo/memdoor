package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/gateway/providers"
)

// An llm task's `system:` reaches the model as the request's own system
// message, ahead of the prompt as the user message: the split GVS5H's
// harness sends every role in. Without one, the request has no system
// message at all, as before.
func TestAnLLMTaskSystemMessageIsTheRequestsSystemRole(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			fmt.Fprint(w, `{"data":[{"id":"qwen3:4b"}]}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "### PLAN\nok"}}}})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		done, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", done)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
		t.Fatal(err)
	}
	c := workflowCompleter{s: &Server{clientFactory: providers.NewClientFactory()}}
	if answer, err := c.CompleteWithSystem(t.Context(), "", "You are the PRIMARY orchestrator.", "the problem"); err != nil || answer != "### PLAN\nok" {
		t.Fatalf("answer %q, err %v", answer, err)
	}
	if answer, err := c.Complete(t.Context(), "", "the problem"); err != nil || answer != "### PLAN\nok" {
		t.Fatalf("answer %q, err %v", answer, err)
	}
	if len(bodies) != 2 {
		t.Fatalf("%d requests", len(bodies))
	}
	roles := func(b map[string]any) []string {
		var out []string
		for _, m := range b["messages"].([]any) {
			msg := m.(map[string]any)
			out = append(out, fmt.Sprintf("%v:%v", msg["role"], msg["content"]))
		}
		return out
	}
	if got := strings.Join(roles(bodies[0]), " | "); !strings.HasPrefix(got, "system:You are the PRIMARY orchestrator. | user:") || !strings.Contains(got, "the problem") {
		t.Errorf("with system: %s", got)
	}
	if got := strings.Join(roles(bodies[1]), " | "); strings.Contains(got, "system:") {
		t.Errorf("without system, a system message was sent: %s", got)
	}
}
