//go:build !windows

// syscall.Kill(pid, 0) is how the test sees a process is gone: Unix only (#283).

package gateway

import (
	"context"
	"encoding/json"
	"memdoor/tools"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func alive(pid int) bool { return syscall.Kill(pid, 0) != syscall.ESRCH }

// gone waits up to 5 s for pid to end.
func gone(pid int) bool {
	for i := 0; i < 50; i++ {
		if !alive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// A COMMAND STILL RUNNING AT THE LIMIT IS LEFT RUNNING, NOT KILLED.
//
// Some take longer, and that is the model's call ("no hard timeout",
// 2026-11-25). The call returns at the limit with the output so far; the
// command goes on, its output and its end in the log the result names.
func TestACommandStillRunningAtTheLimitGoesOn(t *testing.T) {
	ctx, endTurn := withTurnEnd(context.Background())
	defer endTurn()
	start := time.Now()
	r := runStreamingCommand(ctx, "echo started; sleep 1.5; echo finished", "", 300*time.Millisecond, nil)
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("the call waited %s for the command", took)
	}
	if !r.running || r.runErr != nil || r.capped || !strings.Contains(r.out, "started") {
		t.Fatalf("result %+v, want running with its output so far", r)
	}
	defer os.Remove(r.log)
	var log []byte
	for i := 0; i < 50; i++ {
		log, _ = os.ReadFile(r.log)
		if strings.Contains(string(log), "[memdoor: the command ended") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(string(log), "started\nfinished\n") || !strings.Contains(string(log), "[memdoor: the command ended: exit 0]") {
		t.Fatalf("log:\n%s", log)
	}
}

// What a command left running started ends with the turn: bash, a child it
// waits on, and a background child, all in its process group.
func TestACommandLeftRunningEndsWithItsTurn(t *testing.T) {
	ctx, endTurn := withTurnEnd(context.Background())
	r := runStreamingCommand(ctx, "sleep 30 & echo child=$!; wait; echo never", "", 300*time.Millisecond, nil)
	defer os.Remove(r.log)
	m := regexp.MustCompile(`child=(\d+)`).FindStringSubmatch(r.out)
	if !r.running || m == nil {
		t.Fatalf("result %+v", r)
	}
	child, _ := strconv.Atoi(m[1])
	time.Sleep(500 * time.Millisecond)
	if !alive(child) || !alive(r.pid) {
		t.Fatal("the command did not go on past the limit")
	}
	endTurn()
	if !gone(child) || !gone(r.pid) {
		t.Fatal("the command outlived its turn")
	}
}

// Esc (the turn's context cancelled) stops a command left running too.
func TestACommandLeftRunningEndsOnEsc(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := runStreamingCommand(ctx, "sleep 30", "", 300*time.Millisecond, nil)
	defer os.Remove(r.log)
	if !r.running {
		t.Fatalf("result %+v", r)
	}
	cancel()
	if !gone(r.pid) {
		t.Fatal("Esc did not stop it")
	}
}

// The coder is told it was not stopped, where its output goes and how to
// stop it.
func TestTheCoderIsToldACommandWentOn(t *testing.T) {
	old := tools.BashTimeoutLimit
	tools.BashTimeoutLimit = 300 * time.Millisecond
	defer func() { tools.BashTimeoutLimit = old }()
	ctx, endTurn := withTurnEnd(context.Background())
	defer endTurn()
	in, _ := json.Marshal(map[string]string{"command": "echo searching; sleep 30"})
	out, err := bashStreamer{}.Stream(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	pid := regexp.MustCompile(`kill -- -(\d+)`).FindStringSubmatch(out)
	logPath := regexp.MustCompile(`goes on in (\S+),`).FindStringSubmatch(out)
	if pid == nil || logPath == nil || !strings.Contains(out, "searching") || !strings.Contains(out, "not stopped") || strings.Contains(out, "killed") {
		t.Fatalf("result:\n%s", out)
	}
	os.Remove(logPath[1])
}
