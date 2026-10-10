package gateway

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// A finite command streams its output through emit and returns it all.
func TestRunStreamingCommand_Finite(t *testing.T) {
	var mu sync.Mutex
	var chunks []string
	out, truncated, _ := runStreamingCommand(context.Background(), "printf 'a\\nb\\nc\\n'", "", func(c string) { mu.Lock(); chunks = append(chunks, c); mu.Unlock() })

	if truncated {
		t.Error("a finite command must not be truncated")
	}
	if out != "a\nb\nc\n" {
		t.Errorf("output = %q, want a/b/c", out)
	}
	if strings.Join(chunks, "") != "a\nb\nc\n" {
		t.Errorf("emitted chunks = %q, want the full output", strings.Join(chunks, ""))
	}
}

// A command whose output grows over time is streamed incrementally (emit is
// called more than once before completion), not dumped all at once.
func TestRunStreamingCommand_Incremental(t *testing.T) {
	old := bashStreamFlushEvery
	bashStreamFlushEvery = 30 * time.Millisecond
	defer func() { bashStreamFlushEvery = old }()

	var mu sync.Mutex
	calls := 0
	out, _, _ := runStreamingCommand(context.Background(), "for i in 1 2 3; do echo line$i; sleep 0.1; done", "", func(string) { mu.Lock(); calls++; mu.Unlock() })

	if !strings.Contains(out, "line1") || !strings.Contains(out, "line3") {
		t.Errorf("missing lines: %q", out)
	}
	mu.Lock()
	c := calls
	mu.Unlock()
	if c < 2 {
		t.Errorf("expected multiple incremental emits, got %d", c)
	}
}

// A non-terminating command is cut off by the caps and reported truncated,
// instead of hanging forever.
func TestRunStreamingCommand_CapsInfinite(t *testing.T) {
	oldD, oldB := bashStreamMaxDuration, bashStreamMaxBytes
	bashStreamMaxDuration = 400 * time.Millisecond
	bashStreamMaxBytes = 1 << 20
	defer func() { bashStreamMaxDuration, bashStreamMaxBytes = oldD, oldB }()

	start := time.Now()
	out, truncated, _ := runStreamingCommand(context.Background(), "while true; do echo spam; sleep 0.02; done", "", func(string) {})

	if !truncated {
		t.Error("an infinite command must be reported truncated")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("cap did not stop the command promptly")
	}
	if !strings.Contains(out, "spam") {
		t.Errorf("expected some captured output before the cap, got %q", out)
	}
}

// The byte cap bounds total captured output.
func TestRunStreamingCommand_CapsBytes(t *testing.T) {
	oldB := bashStreamMaxBytes
	bashStreamMaxBytes = 200
	defer func() { bashStreamMaxBytes = oldB }()

	out, truncated, _ := runStreamingCommand(context.Background(), "for i in $(seq 1 100000); do echo aaaaaaaaaaaaaaaaaaaa; done", "", func(string) {})

	if !truncated {
		t.Error("output over the byte cap must be truncated")
	}
	if len(out) > 200 {
		t.Errorf("captured %d bytes, cap was 200", len(out))
	}
}

// A non-zero exit surfaces as runErr so executeTool can format the coder's
// "Command FAILED — fix it" message; a cap does not.
func TestRunStreamingCommand_ExitError(t *testing.T) {
	out, truncated, err := runStreamingCommand(context.Background(), "echo oops; exit 3", "", func(string) {})
	if err == nil {
		t.Error("a non-zero exit must return an error")
	}
	if truncated {
		t.Error("a clean (failed) exit is not a truncation")
	}
	if out != "oops\n" {
		t.Errorf("output = %q", out)
	}
}

// The streamer dispatch is a switch returning the interface: bash streams,
// everything else falls through to the normal one-shot path (nil).
func TestStreamerFor(t *testing.T) {
	if streamerFor("bash") == nil {
		t.Error("bash must have a streaming executor")
	}
	if streamerFor("read_file") != nil {
		t.Error("a non-streaming tool must return nil (normal Function path)")
	}
}

// bashStreamer folds a non-zero exit into the coder's FAILED message (returned
// as output, not an error), and streams normal output unchanged.
func TestBashStreamer(t *testing.T) {
	out, err := bashStreamer{}.Stream(context.Background(), []byte(`{"command":"printf hi"}`), func(string) {})
	if err != nil || out != "hi" {
		t.Fatalf("ok case: out=%q err=%v", out, err)
	}
	failOut, err := bashStreamer{}.Stream(context.Background(), []byte(`{"command":"exit 2"}`), func(string) {})
	if err != nil {
		t.Fatalf("a failed command must fold into output, not error: %v", err)
	}
	if !strings.Contains(failOut, "Command FAILED") {
		t.Errorf("missing FAILED message: %q", failOut)
	}
}

// The streamer formats a non-zero exit with the one-shot tool's formatter:
// a probe for a missing folder is an answer, not "Command FAILED — fix IT"
// (live 2026-10-04: `ls -a; echo ---; ls .agents 2>/dev/null`).
func TestTheStreamerReadsAMissingPathAsAnAnswer(t *testing.T) {
	dir := t.TempDir()
	out, err := bashStreamer{}.Stream(context.Background(), []byte(`{"command":"ls -a; echo ---; ls .agents 2>/dev/null","cwd":"`+dir+`"}`), func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	// A missing path is exit 1 under BSD ls (macOS) and exit 2 under GNU ls
	// (the CI runner); either is a result, not a failure.
	if strings.Contains(out, "FAILED") || strings.Contains(out, "fix IT") || !(strings.Contains(out, "exit status 1") || strings.Contains(out, "exit status 2")) {
		t.Fatalf("a read's exit status is a result: %q", out)
	}
	out, _ = bashStreamer{}.Stream(context.Background(), []byte(`{"command":"false","cwd":"`+dir+`"}`), func(string) {})
	if !strings.Contains(out, "Command FAILED") {
		t.Fatalf("a command that is not a read still fails: %q", out)
	}
}

// A command that launches something in the background and exits 0 is a
// launch, not a failure, even though the child holds the output pipe past
// WaitDelay (live, 2026-10-10: `(python3 … > log) & echo bg` and a `nohup sh
// -c … &` both came back "Command FAILED (exec: WaitDelay expired before I/O
// complete)" with the exit code 0 and "bg" printed).
func TestRunStreamingCommand_BackgroundLaunchIsNotAFailure(t *testing.T) {
	out, truncated, err := runStreamingCommand(context.Background(), "(sleep 7 > /dev/null 2>&1; sleep 0) & echo bg", "", nil)
	if err != nil {
		t.Fatalf("an exit-0 launch must not fail: %v", err)
	}
	if truncated || !strings.Contains(out, "bg") {
		t.Fatalf("out=%q truncated=%v, want bg and not truncated", out, truncated)
	}
	if _, _, err := runStreamingCommand(context.Background(), "(sleep 7 > /dev/null 2>&1; sleep 0) & exit 3", "", nil); err == nil {
		t.Fatal("a non-zero exit must still fail")
	}
}
