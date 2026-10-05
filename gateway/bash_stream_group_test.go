package gateway

import (
	"context"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// The cap kills what bash started, not only bash. Killing bash alone left a
// long child (sleep, find) holding the output pipe, and Run waited for it:
// measured live 2026-09-25, `sleep 401; echo …` held a coder turn 401 s
// against a 30 s cap, and `find ~ -maxdepth 4 …` held one for over an hour.
func TestRunStreamingCommand_CapKillsChildren(t *testing.T) {
	oldD := bashStreamMaxDuration
	bashStreamMaxDuration = 500 * time.Millisecond
	defer func() { bashStreamMaxDuration = oldD }()

	start := time.Now()
	out, truncated, _ := runStreamingCommand(context.Background(),
		"sleep 30 & echo child=$!; wait; echo never", "", func(string) {})
	if !truncated {
		t.Error("want truncated")
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Fatalf("cap held for %s: a child kept the pipe open", d)
	}
	m := regexp.MustCompile(`child=(\d+)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no child pid in %q", out)
	}
	pid, _ := strconv.Atoi(m[1])
	for i := 0; i < 30; i++ {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("child %d survived the cap", pid)
}

// A foreground child too: `sleep 30; echo` with no background job.
func TestRunStreamingCommand_CapStopsForegroundChild(t *testing.T) {
	oldD := bashStreamMaxDuration
	bashStreamMaxDuration = 500 * time.Millisecond
	defer func() { bashStreamMaxDuration = oldD }()
	start := time.Now()
	_, truncated, _ := runStreamingCommand(context.Background(), "sleep 30; echo done", "", func(string) {})
	if !truncated || time.Since(start) > 8*time.Second {
		t.Fatalf("truncated=%v after %s", truncated, time.Since(start))
	}
}
