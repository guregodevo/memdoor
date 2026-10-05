package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
)

func withDecisions(t *testing.T, svc decision.Service) {
	t.Helper()
	prev := decisionService()
	SetDecisionService(svc)
	t.Cleanup(func() { SetDecisionService(prev) })
}

func TestJreadSectionsAndKeepsRelevant(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("func filler() {\n\treturn\n}\n\n")
	}
	b.WriteString("func resumeAfterRestart() {\n\t// the answer\n}\n")
	path := filepath.Join(t.TempDir(), "big.go")
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "resumeAfterRestart") {
			return 0.9
		}
		return 0.05
	}})
	in, _ := json.Marshal(JreadInput{Task: "where is a turn resumed after restart", Path: path})
	out, err := Jread(in)
	if err != nil || !strings.Contains(out, "kept 1 of") || !strings.Contains(out, "resumeAfterRestart") || strings.Contains(out, "filler") {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, sc := range fileSections(strings.Split(b.String(), "\n")) {
		if sc[1]-sc[0]+1 > jreadSectionLines {
			t.Fatalf("section %v longer than %d lines", sc, jreadSectionLines)
		}
	}
}

// No decision model: the read degrades to the unjudged head of the file, said
// so, rather than dropping what it could not judge.
func TestJreadWithoutAModelIsUnjudged(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "func filler%d() {\n\treturn\n}\n\n", i)
	}
	path := filepath.Join(t.TempDir(), "big.go")
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
	withDecisions(t, nil)
	in, _ := json.Marshal(JreadInput{Task: "where is filler3", Path: path})
	out, err := Jread(in)
	if err != nil || !strings.Contains(out, "UNJUDGED") || !strings.Contains(out, "filler0") {
		t.Fatalf("no model must return the unjudged head: %v\n%s", err, out)
	}
}

func TestRenderLogEvent(t *testing.T) {
	e := &logs.Event{Timestamp: time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC), Level: "ERROR", Component: "Agent",
		Message: "Tool execution failed", Data: map[string]any{"tool": "chrome_devtools", "b": 1}}
	if got := renderLogEvent(e); got != "15:00:00.000 ERROR [Agent] Tool execution failed b=1 tool=chrome_devtools" {
		t.Fatalf("%q", got)
	}
}

// Live: jread through Jev on real material.
//
//	OPEN_ROUTER_API_KEY=... JEV_E2E=1 go test ./tools/ -run TestJevToolsLive -v
func TestJevToolsLive(t *testing.T) {
	if os.Getenv("JEV_E2E") != "1" {
		t.Skip("set JEV_E2E=1 and OPEN_ROUTER_API_KEY")
	}
	c, err := systemone.New(systemone.Config{BaseURL: "https://openrouter.ai/api/alpha/decisions", APIKey: os.Getenv("OPEN_ROUTER_API_KEY"), Model: "~typesafe/jev-latest"})
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := decision.NewService(func(string) string { return "systemone" }, c)
	withDecisions(t, svc)

	path, _ := filepath.Abs("../gateway/agent_runtime_process.go")
	raw, _ := os.ReadFile(path)
	start := time.Now()
	in, _ := json.Marshal(JreadInput{Task: "Find where the turn loop gives up after the same tool keeps failing.", Path: path, Keep: 3})
	out, err := Jread(in)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("jread: %d KB file -> %.1f KB in %s\n%s", len(raw)/1024, float64(len(out))/1024, time.Since(start).Round(time.Millisecond), firstLines(out, 4))
	if !strings.Contains(out, "maxRepeatedFail") && !strings.Contains(out, "maxFailsPerTool") {
		t.Error("jread missed the failure-limit section")
	}

}

func firstLines(s string, n int) string {
	ls := strings.Split(s, "\n")
	return strings.Join(ls[:min(n, len(ls))], "\n")
}

func TestRenderLogEventIsCapped(t *testing.T) {
	data := map[string]interface{}{}
	for i := 0; i < 20; i++ {
		data[fmt.Sprintf("k%02d", i)] = strings.Repeat("v", 190)
	}
	e := &logs.Event{Timestamp: time.Now(), Level: "INFO", Component: "LLM", Message: "LLM request " + strings.Repeat("system_head ", 400), Data: data}
	out := renderLogEvent(e)
	if len(out) > logEventLineMax+len("…") || !strings.HasPrefix(out[13:], "INFO [LLM] LLM request") {
		t.Fatalf("len %d: %q", len(out), out[:80])
	}
}

// A judged read that keeps nearly all of a file saves nothing and costs
// fidelity: the caller must get the file whole (2026-09-27, the edit A/B).
func TestJudgedReadWithoutSavingReturnsTheWholeFile(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "func wanted%d() {\n\treturn\n}\n\n", i)
	}
	path := filepath.Join(t.TempDir(), "small.go")
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
	withDecisions(t, &scriptedDecisions{p: func(string) float64 { return 0.9 }})

	if _, err := judgedSections("jread", path, b.String(), "change every function", 12); err == nil {
		t.Fatal("judging that keeps everything must report no saving")
	}
	in, _ := json.Marshal(JreadInput{Task: "change every function", Path: path})
	out, err := Jread(in)
	if err != nil {
		t.Fatalf("jread must answer with the file: %v", err)
	}
	if !strings.Contains(out, "whole") || !strings.Contains(out, "wanted7") || strings.Contains(out, "(p=") {
		t.Fatalf("expected the whole file, not judged sections:\n%s", out)
	}
	// And the judged path still compresses a file long enough to have
	// several sections when the task needs only one of them.
	var long strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&long, "func wanted%d() {\n\treturn\n}\n\n", i)
	}
	if n := len(fileSections(strings.Split(long.String(), "\n"))); n < 3 {
		t.Fatalf("test needs a file of several sections, got %d", n)
	}
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "wanted3()") {
			return 0.95
		}
		return 0.02
	}})
	out, err = judgedSections("jread", path, long.String(), "what does wanted3 do", 12)
	if err != nil || !strings.Contains(out, "wanted3") || strings.Contains(out, "wanted39") {
		t.Fatalf("judged read should keep one section: %v\n%s", err, out)
	}
}
