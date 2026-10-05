package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// A parent's tool calls and a sub-session's read alike unless the line says
// which conversation and run it belongs to (roadmap: "don't guess"). --data
// shows both; --session and --run ask the gateway for only those.
func TestLogsQueryShowsAndFiltersTheSession(t *testing.T) {
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	t.Setenv("HOME", t.TempDir())
	var asked url.Values
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{"events": []map[string]any{{
			"timestamp": "2026-09-29T14:00:00Z", "level": "INFO", "component": "Agent",
			"message": "Tool call started: bash",
			"session": "agent:coder:subagent:abc", "run_id": "run-7-coder",
			"data": map[string]any{"tool": "bash"},
		}}})
	}))
	defer gw.Close()
	gatewayAddr = gw.URL

	logsSession, logsRun, logsShowData = "agent:coder:subagent:abc", "run-7-coder", true
	t.Cleanup(func() { logsSession, logsRun, logsShowData = "", "", false })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	out := os.Stdout
	os.Stdout = w
	runErr := logsQueryCmd.RunE(logsQueryCmd, nil)
	os.Stdout = out
	_ = w.Close()
	printed, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatal(runErr)
	}

	if asked.Get("session") != "agent:coder:subagent:abc" || asked.Get("run") != "run-7-coder" {
		t.Errorf("the filters must reach the gateway: %v", asked)
	}
	for _, want := range []string{"session=agent:coder:subagent:abc", "run_id=run-7-coder"} {
		if !strings.Contains(string(printed), want) {
			t.Errorf("--data must show %q:\n%s", want, printed)
		}
	}
}
