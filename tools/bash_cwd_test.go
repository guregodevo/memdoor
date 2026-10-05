package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/logs"
)

// Bash logs through the global EventLogger, which panics if uninitialised.
// These tests RUN the command (cmd.Dir is only real if the process starts
// there), so they need it up.
func TestMain(m *testing.M) {
	if err := logs.InitGlobalLogger(os.TempDir()+"/memdoor-tools-test-logs", false); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// bash was the only confined tool without a cwd, so the gateway expressed "run
// here" by string-prefixing `cd '<wd>' && ` to the model's command. That is a
// DEFAULT directory rather than a working directory: the model's own cd runs
// after it and wins, and on 2026-08-29 a model emitted
//
//	cd '<wd>' && cd /Users/you/.memdoor/workdir/tetris && go run stats.go
//
// against a path it had invented. cmd.Dir says the same thing to the operating
// system instead of to a shell that can be argued with.
//
// This RUNS bash rather than inspecting the struct — cmd.Dir is only real if
// the process actually starts there.
func TestBashRunsInTheInjectedCwd(t *testing.T) {
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir) // macOS /var -> /private/var
	if err != nil {
		t.Fatal(err)
	}

	in, _ := json.Marshal(map[string]any{"command": "pwd", "cwd": dir})
	out, err := Bash(in)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	got := strings.TrimSpace(out)
	if got != real && got != dir {
		t.Errorf("bash ran in %q, want the injected cwd %q — cmd.Dir is not being applied", got, dir)
	}
}

// With no cwd injected, bash must still work — the field is harness-only and
// every non-coder caller omits it. An empty cmd.Dir means "inherit", which is
// the pre-existing behaviour.
func TestBashWithoutCwdStillRuns(t *testing.T) {
	in, _ := json.Marshal(map[string]any{"command": "echo ok"})
	out, err := Bash(in)
	if err != nil {
		t.Fatalf("bash failed with no cwd: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Errorf("got %q", out)
	}
}

// Relative paths resolve against the injected cwd — which is the whole reason
// the coder's file tools and bash must agree on one directory.
func TestBashResolvesRelativePathsAgainstCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("found"), 0o600); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]any{"command": "cat marker.txt", "cwd": dir})
	out, err := Bash(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "found") {
		t.Errorf("a relative path must resolve against the injected cwd, got %q", out)
	}
}

// A bash call whose ARGUMENTS are the tool-call envelope has no command in it
// at all, and "command is required" reads as "you forgot a field" — inviting
// the same shape again.
//
// Measured 2026-08-30 on a live coder turn: this arrived as a frame labelled
// `Bash(name: "bash")` that ran nothing. The model needs to be told it wrapped
// the call twice.
func TestDoubleWrappedBashCallSaysSo(t *testing.T) {
	_, err := ParseBashCommand(json.RawMessage(`{"name":"bash"}`))
	if err == nil {
		t.Fatal("a call with no command must fail")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "envelope") || !strings.Contains(msg, "arguments") {
		t.Errorf("the error must name the actual mistake (double-wrapping), got: %v", err)
	}
	// A genuinely empty call keeps the plain message — there is no envelope to
	// blame, and a wrong diagnosis is worse than a terse one.
	_, err = ParseBashCommand(json.RawMessage(`{}`))
	if err == nil || strings.Contains(strings.ToLower(err.Error()), "envelope") {
		t.Errorf("an empty call is not a double-wrap: %v", err)
	}
}
