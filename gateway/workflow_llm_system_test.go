package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/providers"
	"memdoor/pkg/metering"
)

// An llm task's `system:` reaches the model as the request's own system
// message, ahead of the prompt as the user message: the split GVS5H's
// harness sends every role in. Without one, the request has no system
// message at all, as before.
func TestAnLLMTaskSystemMessageIsTheRequestsSystemRole(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
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
		done, _ := json.Marshal(map[string]any{"model": "qwen3:4b", "usage": map[string]any{"prompt_tokens": 12, "completion_tokens": 7, "total_tokens": 19, "cost": 0.0012}, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", done)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
		t.Fatal(err)
	}
	c := workflowCompleter{s: runServer(t, &Server{clientFactory: providers.NewClientFactory()})}
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

	t.Run("workflow steps have separate usage records", func(t *testing.T) {
		s := runServer(t, &Server{clientFactory: providers.NewClientFactory()})
		wantSessions := map[string]bool{}
		for range 2 {
			dir := t.TempDir()
			t.Cleanup(s.closeRunRepos) // after the folder's own, so it runs before it
			steps := filepath.Join(dir, ".memdoor", "workflows", "meter-test", "steps")
			if err := os.MkdirAll(steps, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, yaml := range map[string]string{
				"plan":  "type: llm\nsystem: You are the planner.\nprompt: Plan the task.\n",
				"judge": "type: llm\nprompt: Judge the result.\nrequires:\n  - table_pattern: plan\n",
			} {
				if err := os.WriteFile(filepath.Join(steps, name+".yaml"), []byte(yaml), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			run, err := s.startWorkflow(dir, "meter-test", "", "", "", "same-day", "", "", 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(run.cancel)
			deadline := time.Now().Add(10 * time.Second)
			for run.snapshot().State == "running" && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if got := run.snapshot(); got.State != "done" {
				t.Fatalf("workflow: %+v", got)
			}
			wantSessions[run.taskSession("plan")] = true
			wantSessions[run.taskSession("judge")] = true
		}
		if len(bodies) != 6 {
			t.Fatalf("got %d requests, want 6", len(bodies))
		}
		for i, body := range bodies[2:] {
			got := strings.Join(roles(body), " | ")
			if i%2 == 0 && !strings.HasPrefix(got, "system:You are the planner. | user:") {
				t.Errorf("plan request: %s", got)
			}
			if i%2 == 1 && strings.Contains(got, "system:") {
				t.Errorf("judge request has a system message: %s", got)
			}
		}
		ledger, err := metering.NewJSONLLedger("")
		if err != nil {
			t.Fatal(err)
		}
		entries, err := ledger.All()
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 6 || len(wantSessions) != 4 {
			t.Fatalf("got %d ledger entries and %d distinct step sessions, want 6 and 4", len(entries), len(wantSessions))
		}
		for _, entry := range entries[2:] {
			if !wantSessions[entry.Session] {
				t.Errorf("unexpected or duplicate step session %q", entry.Session)
			}
			delete(wantSessions, entry.Session)
			if entry.Model != "qwen3:4b" || entry.InputTokens != 12 || entry.OutputTokens != 7 || entry.CostUSD != 0.0012 {
				t.Errorf("usage was not preserved: %+v", entry)
			}
		}
		if len(wantSessions) != 0 {
			t.Errorf("missing step usage: %v", wantSessions)
		}
	})
}

// top_p and top_k on a task's model line reach the request; left out, the
// request names neither and the host's defaults apply.
func TestAnLLMTaskSendsTopPAndTopK(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
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
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ok"}, "finish_reason": "stop"}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
		t.Fatal(err)
	}
	c := workflowCompleter{s: runServer(t, &Server{clientFactory: providers.NewClientFactory()})}
	for _, line := range []string{"temperature 0.3 top_p 0.95 top_k 20", "temperature 0.3"} {
		if _, err := c.Complete(t.Context(), line, "the problem"); err != nil {
			t.Fatal(err)
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("%d requests", len(bodies))
	}
	if bodies[0]["top_p"] != 0.95 || bodies[0]["top_k"] != float64(20) || bodies[0]["temperature"] != 0.3 {
		t.Errorf("with top_p/top_k: %v %v %v", bodies[0]["top_p"], bodies[0]["top_k"], bodies[0]["temperature"])
	}
	if _, ok := bodies[1]["top_p"]; ok {
		t.Errorf("top_p sent without being asked: %v", bodies[1]["top_p"])
	}
	if _, ok := bodies[1]["top_k"]; ok {
		t.Errorf("top_k sent without being asked: %v", bodies[1]["top_k"])
	}
}
