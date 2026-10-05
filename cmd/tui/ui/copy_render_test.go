package ui

import (
	"strings"
	"testing"
)

// /copy is the rendering probe — "use copy to see rendered" — but it dumped
// RAW tool output while the screen showed collapsed frames. Measured
// 2026-08-31 on a real coding turn: the live view said "⏺ Read(stats.go) ⎿
// Read 17 lines" and drew a five-line diff; /copy pasted the whole file
// twice plus the patch echo ("stats.go is now: <entire file>"). What you copy
// must be what you saw.
func TestCopyCollapsesAReadTheWayTheScreenDoes(t *testing.T) {
	file := strings.Repeat("line of code\n", 17)
	got := formatTranscriptForCopy([]Message{{
		Role: "tool_call", ToolName: "read_file",
		ToolInput: `{"path":"stats.go"}`, ToolOutput: file,
	}})
	if !strings.Contains(got, "Read 17 lines") {
		t.Errorf("copy does not show the collapsed read summary:\n%s", got)
	}
	if strings.Count(got, "line of code") > 0 {
		t.Errorf("copy dumped the file the screen never showed:\n%s", got)
	}
}

func TestCopyShowsTheDiffNotThePatchEcho(t *testing.T) {
	out := "Patch applied. added=[] modified=[stats.go] deleted=[]\n\nstats.go is now:\npackage main\nfunc mean() {}\n"
	in := `{"input":"*** Update File: stats.go\n@@\n-return sum / float64(len(xs))\n+if len(xs) == 0 {\n+\treturn 0\n+}\n+return sum / float64(len(xs))\n"}`
	got := formatTranscriptForCopy([]Message{{
		Role: "tool_call", ToolName: "apply_patch", ToolInput: in, ToolOutput: out,
	}})
	if !strings.Contains(got, "+") || !strings.Contains(got, "return 0") {
		t.Errorf("copy lost the diff the screen showed:\n%s", got)
	}
	if strings.Contains(got, "stats.go is now:") {
		t.Errorf("copy pasted the patch echo the screen suppresses:\n%s", got)
	}
}

// Bash output is what goes into bug reports — it stays, capped the way the
// screen caps it.
func TestCopyKeepsBashOutput(t *testing.T) {
	got := formatTranscriptForCopy([]Message{{
		Role: "tool_call", ToolName: "bash",
		ToolInput: `{"command":"go run stats.go"}`, ToolOutput: "mean of nothing: 0\nmean of 1..4: 2.5",
	}})
	for _, want := range []string{"mean of nothing: 0", "mean of 1..4: 2.5"} {
		if !strings.Contains(got, want) {
			t.Errorf("bash output missing %q:\n%s", want, got)
		}
	}
}

// Errors always survive verbatim — they are the reason most transcripts get
// pasted anywhere.
func TestCopyKeepsErrors(t *testing.T) {
	got := formatTranscriptForCopy([]Message{{
		Role: "tool_call", ToolName: "bash",
		ToolInput: `{"command":"go build"}`, ToolError: "exit status 1: undefined: mean",
	}})
	if !strings.Contains(got, "undefined: mean") {
		t.Errorf("the error was lost:\n%s", got)
	}
}

// Plain text means plain: the frames render through lipgloss, and pasting a
// transcript full of colour escapes into a PR is worse than either extreme.
func TestCopyCarriesNoANSI(t *testing.T) {
	got := formatTranscriptForCopy([]Message{
		{Role: "user", Content: "fix it"},
		{Role: "tool_call", ToolName: "read_file", ToolInput: `{"path":"a.go"}`, ToolOutput: "x\ny\n"},
		{Role: "tool_call", ToolName: "apply_patch", ToolInput: `{"input":"*** Update File: a.go\n@@\n-x\n+y\n"}`, ToolOutput: "Patch applied."},
	})
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("ANSI escapes in copied text: %q", got)
	}

	// Under `go test` lipgloss detects no TTY and emits no colour, so the
	// transcript check above cannot catch a broken stripper — pin the helper
	// itself against real escapes.
	if got := plainText("\x1b[35mmagenta\x1b[0m plain"); got != "magenta plain" {
		t.Errorf("plainText did not strip: %q", got)
	}
}
