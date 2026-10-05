package tools

import (
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A command past the timeout is killed with its children, and the output so
// far comes back with the reason.
func TestRunPlainBashTimeoutKillsGroup(t *testing.T) {
	oldLimit := BashTimeoutLimit
	BashTimeoutLimit = time.Second
	defer func() { BashTimeoutLimit = oldLimit }()
	start := time.Now()
	out, err := runPlainBash("sleep 30 & echo child=$!; wait; echo never", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want a timeout error, got %v", err)
	}
	if strings.Contains(out, "never") {
		t.Fatalf("command ran past the kill: %q", out)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("took %s", d)
	}
	m := regexp.MustCompile(`child=(\d+)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no child pid in %q", out)
	}
	pid, _ := strconv.Atoi(m[1])
	for i := 0; i < 30; i++ { // the orphan is reaped by launchd/init shortly after
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("child %d survived the timeout", pid)
}

func TestRunPlainBashFastCommand(t *testing.T) {
	out, err := runPlainBash("printf ok", t.TempDir())
	if err != nil || out != "ok" {
		t.Fatalf("got %q, %v", out, err)
	}
}
