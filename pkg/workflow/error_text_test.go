package workflow

import (
	"strings"
	"testing"
)

// A command task's error shows its exit status and what it printed, not the
// command it repeats (mario: "task <name>: <command>: exit status 1 — …").
func TestErrorTextDropsTheCommand(t *testing.T) {
	got := ErrorText("task w.steps.verdict: f=x\n[ -f \"$f\" ] || exit 0\n: exit status 1 — 1 finding(s) that must not merge:\n- **P0** `a.go:6` — wrong")
	want := "exit status 1 — 1 finding(s) that must not merge:\n- **P0** `a.go:6` — wrong"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := ErrorText("no model is configured"); got != "no model is configured" {
		t.Fatalf("an error with no exit status changed: %q", got)
	}
}

// The window shows the status line and the last lines printed.
func TestErrorLinesKeepTheStatusAndTheTail(t *testing.T) {
	e := "task w.steps.build: make\n: exit status 2 — line1\nline2\nline3\nline4\nline5"
	got := ErrorLines(e, 2)
	if strings.Join(got, "|") != "exit status 2 — line1|…|line4|line5" {
		t.Fatalf("got %q", got)
	}
	if got := ErrorLines("task x: cmd: exit status 1 — only", 3); len(got) != 1 || got[0] != "exit status 1 — only" {
		t.Fatalf("a short error is itself: %q", got)
	}
}

// The paths around a command's failure: a status with nothing printed, a
// nested status in the output, a kill, an agent step's error and an empty
// error pass through or trim at the first status only.
func TestErrorTextEdges(t *testing.T) {
	for in, want := range map[string]string{
		"task w.steps.quiet: exit 1\n: exit status 1":                                    "exit status 1",
		"task w.steps.build: make: exit status 2 — make: *** [all] Error: exit status 1": "exit status 2 — make: *** [all] Error: exit status 1",
		"task w.steps.long: sleep 60: signal: killed":                                    "task w.steps.long: sleep 60: signal: killed",
		"task w.steps.solve: the model answered nothing":                                 "task w.steps.solve: the model answered nothing",
		"": "",
	} {
		if got := ErrorText(in); got != want {
			t.Errorf("ErrorText(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ErrorLines("", 3); len(got) != 1 || got[0] != "" {
		t.Errorf("no error, one empty line: %q", got)
	}
	if got := ErrorLines("task x: c: exit status 1 — a\nb\nc\nd", 3); strings.Join(got, "|") != "exit status 1 — a|b|c|d" {
		t.Errorf("four lines fit three plus the status: %q", got)
	}
}
