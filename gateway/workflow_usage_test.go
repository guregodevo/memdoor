package gateway

import (
	"encoding/json"
	"fmt"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/providers"
)

// Each step's tokens and cost are summed from the ledger rows as they are
// written, and the run's state carries them: the window shows them live
// under each step (#94), as a single turn shows its own.
func TestAWorkflowRunSumsEachStepsTokens(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			fmt.Fprint(w, `{"data":[{"id":"qwen3:4b"}]}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ok"}}}})
		done, _ := json.Marshal(map[string]any{"model": "qwen3:4b", "usage": map[string]any{"prompt_tokens": 1200, "completion_tokens": 300, "total_tokens": 1500, "cost": 0.002}, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
		fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", chunk, done)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "local", API: providers.APIChat, Base: srv.URL + "/v1", Keyless: true, Models: []providers.Model{{ID: "qwen3:4b"}}}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	steps := filepath.Join(dir, ".memdoor", "workflows", "usage-test", "steps")
	if err := os.MkdirAll(steps, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, yaml := range map[string]string{
		"plan":  "type: llm\nprompt: Plan the task.\n",
		"judge": "type: llm\nprompt: Judge the result.\nrequires:\n  - table_pattern: plan\n",
	} {
		if err := os.WriteFile(filepath.Join(steps, name+".yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := runServer(t, &Server{clientFactory: providers.NewClientFactory()})
	run, err := s.startWorkflow(dir, "usage-test", "", "", "", "", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(run.cancel)
	// A row of another run's step, written while this one runs, is not this run's.
	providers.RecordMeter(providers.MeterEntry{Session: "workflow:usage-test:other@20990101-000000:plan", InputTokens: 99999})
	deadline := time.Now().Add(10 * time.Second)
	for run.snapshot().State == "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	snap := run.snapshot()
	if snap.State != "done" {
		t.Fatalf("workflow: %+v", snap)
	}
	for _, task := range snap.Tasks {
		if task.InputTokens != 1200 || task.OutputTokens != 300 || task.CostUSD != 0.002 || task.Calls != 1 {
			t.Errorf("%s: %d in, %d out, $%v, %d calls; want 1200, 300, $0.002, 1", task.Name, task.InputTokens, task.OutputTokens, task.CostUSD, task.Calls)
		}
	}
	if snap.InputTokens != 2400 || snap.OutputTokens != 600 {
		t.Errorf("run: %d in, %d out; want 2400, 600", snap.InputTokens, snap.OutputTokens)
	}
	// After the run, a row in its sessions is no longer counted: its observer is gone.
	providers.RecordMeter(providers.MeterEntry{Session: run.taskSession("plan"), InputTokens: 5})
	if got := run.snapshot().InputTokens; got != 2400 {
		t.Errorf("a row after the run ended was counted: %d", got)
	}
}
