package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

func shadowFor(t *testing.T) *routeShadow {
	t.Helper()
	ctx := context.WithValue(context.Background(), sharedctx.SessionIDKey, "s1")
	ctx = context.WithValue(ctx, sharedctx.TierKey, 0)
	s := newRouteShadow(ctx, "coder")
	if s == nil {
		t.Fatal("shadow on by default")
	}
	return s
}

// The same failing test run five times is a hard trigger; a green run
// before that forgives everything.
func TestShadowFailStreakFiresAndGreenResets(t *testing.T) {
	s := shadowFor(t)
	fail := ToolExecutionInfo{Output: "--- FAIL: TestX (0.00s)\nFAIL\tm\t0.1s", Error: "exit status 1"}
	edit := ToolExecutionInfo{Output: "ok"}
	for i := 0; i < 2; i++ {
		s.observe("bash", `{"command":"go test ./..."}`, fail)
		s.observe("apply_patch", "patch "+string(rune('a'+i)), edit)
	}
	if s.fired {
		t.Fatalf("two failing runs with edits between are not a loop: %+v", s.firedParts)
	}
	s.observe("bash", `{"command":"go test ./..."}`, ToolExecutionInfo{Output: "ok\tm\t0.1s"})
	if s.resets != 1 || s.lastScore != 0 {
		t.Fatalf("a green run resets: resets=%d score=%d", s.resets, s.lastScore)
	}
	for i := 0; i < 5; i++ {
		s.observe("bash", `{"command":"go test ./..."}`, fail)
	}
	if !s.fired || s.firedParts["fail_streak"] != 3 {
		t.Fatalf("five failing runs with the same failing set fire: fired=%v parts=%v", s.fired, s.firedParts)
	}
}

// A shrinking failing set is progress, not a streak.
func TestShadowShrinkingFailingSetIsProgress(t *testing.T) {
	s := shadowFor(t)
	s.observe("bash", `{"command":"go test ./..."}`, ToolExecutionInfo{Output: "--- FAIL: TestA\n--- FAIL: TestB\n--- FAIL: TestC", Error: "exit status 1"})
	s.observe("apply_patch", "p1", ToolExecutionInfo{Output: "ok"})
	s.observe("bash", `{"command":"go test ./..."}`, ToolExecutionInfo{Output: "--- FAIL: TestA\n--- FAIL: TestB", Error: "exit status 1"})
	s.observe("apply_patch", "p2", ToolExecutionInfo{Output: "ok"})
	s.observe("bash", `{"command":"go test ./..."}`, ToolExecutionInfo{Output: "--- FAIL: TestA", Error: "exit status 1"})
	if s.fired || s.resets != 2 {
		t.Fatalf("each shrink resets: fired=%v resets=%d", s.fired, s.resets)
	}
}

// Identical calls: three score, five fire on their own; an alternating
// pair scores too.
func TestShadowRepeatsAndErrors(t *testing.T) {
	s := shadowFor(t)
	for i := 0; i < 3; i++ {
		s.observe("read_file", `{"path":"a.go"}`, ToolExecutionInfo{Output: "x"})
	}
	if sc, parts, _ := s.score(); sc != 3 || parts["repeat"] != 3 {
		t.Fatalf("three identical calls: %d %v", sc, parts)
	}
	for i := 0; i < 2; i++ {
		s.observe("read_file", `{"path":"a.go"}`, ToolExecutionInfo{Output: "x"})
	}
	if !s.fired || s.firedAt != 5 {
		t.Fatalf("five identical calls are a hard trigger: %v at %d", s.fired, s.firedAt)
	}
	s = shadowFor(t)
	for i := 0; i < 3; i++ {
		s.observe("bash", `{"command":"go build ./..."}`, ToolExecutionInfo{Output: "x.go:3: undefined: y", Error: "exit status 1"})
		s.observe("read_file", `{"path":"x.go"}`, ToolExecutionInfo{Output: "src"})
	}
	sc, parts, _ := s.score()
	if parts["alternating"] != 2 || parts["errors"] < 2 || parts["fail_streak"] != 3 || sc < 7 {
		t.Fatalf("alternating build-fail/read: %d %v", sc, parts)
	}
}

// The turn's line lands in the log with its calls.
func TestShadowFinishWritesALine(t *testing.T) {
	dir := t.TempDir()
	old := shadowLogPath
	shadowLogPath = func() string { return filepath.Join(dir, "shadow.jsonl") }
	t.Cleanup(func() { shadowLogPath = old })
	s := shadowFor(t)
	s.observe("grep", `{"pattern":"x"}`, ToolExecutionInfo{Output: "a.go:1:x"})
	s.observeTruncation()
	s.finish(&AgentResponse{Model: "deepseek/deepseek-v4.1-flash"})
	b, err := os.ReadFile(filepath.Join(dir, "shadow.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rec shadowRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Session != "s1" || rec.Model != "deepseek/deepseek-v4.1-flash" || rec.Calls != 1 || rec.Truncations != 1 || rec.ToolCalls[0].Kind != "read" || rec.Fired {
		t.Fatalf("%+v", rec)
	}
}

func TestCallKind(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"bash", `{"command":"go test ./gateway -run X"}`}:   "test",
		{"bash", `{"command":"go vet ./..."}`}:               "build",
		{"bash", `{"command":"sed -n 1,20p a.go"}`}:          "read",
		{"bash", `{"command":"cat > a.go <<'EOF'\nx\nEOF"}`}: "edit",
		{"apply_patch", "x"}:                                 "edit",
		{"jgrep", "x"}:                                       "read",
		{"todo_write", "x"}:                                  "other",
	} {
		if got := callKind(in[0], in[1]); got != want {
			t.Errorf("%v: %s, want %s", in, got, want)
		}
	}
}
