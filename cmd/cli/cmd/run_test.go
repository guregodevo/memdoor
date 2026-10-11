package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The exit code is the receipt's verdict: checked is 0, unverified or a
// failing check 2, an unanswered question 3, an error 1.
func TestHeadlessExitCodes(t *testing.T) {
	cases := []struct {
		text  string
		asked bool
		want  int
	}{
		{"Done.\n✓ checked: 1 file changed · go test ./... PASS", false, 0},
		{"Done.\n⚠ unverified: 2 files changed · no build, test or run after it", false, 2},
		// Greg's first run (2026-10-11): failed twice on the way, then passed — the receipt says checked, so 0.
		{"Done.\n✓ checked: 2 files changed · `go build calc.go` FAIL · `go test ./...` FAIL · `go mod init tmpcalc && go test ./...` PASS", false, 0},
		{"⚠ nothing changed · ran: `go test ./...` FAIL · `go test ./...` PASS", false, 0},
		{"⚠ nothing changed · ran: `go test ./...` PASS · `go vet ./...` FAIL", false, 2},
		{"Here is the answer, no tools used.", false, 0},
		{"Green.\n⚠ nothing changed · ran: `go test ./...` PASS", false, 0},
		{"Red.\n⚠ nothing changed · ran: `go test ./...` FAIL", false, 2},
		{"Changed.\n⚠ failing after the change: go test ./... FAIL", false, 2},
		{"✓ checked: 1 file changed · go test ./... PASS", true, 3},
	}
	for _, c := range cases {
		if got := runExitCode(nil, c.asked, receiptIn(c.text)); got != c.want {
			t.Errorf("%q asked=%v: exit %d, want %d", c.text, c.asked, got, c.want)
		}
	}
	if got := runExitCode(bytes.ErrTooLarge, false, "✓ checked: x PASS"); got != 1 {
		t.Errorf("an error exits 1, got %d", got)
	}
}

// --json is NDJSON a pipeline can parse: one event per line, the receipt and
// the exit code on the last; --yes answers a question with its first option.
func TestHeadlessSinkJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	s := &headlessSink{out: &out, errOut: &errOut, json: true, yes: true}
	s.ToolStart("t1", "bash", json.RawMessage(`{"command":"go test ./..."}`))
	s.ToolDone("t1", "ok", false)
	if got := s.Ask("Which file?", []string{"a.go", "b.go"}, ""); got != "a.go" {
		t.Fatalf("--yes picks the first option, got %q", got)
	}
	if got := s.Ask("Run: rm -rf build — allow?", []string{"Yes", "No"}, "bash"); got != "Yes" {
		t.Fatalf("--yes approves, got %q", got)
	}
	s.Text("Fixed.\n✓ checked: 1 file changed · go test ./... PASS")
	if code := s.finish("c0921b32", nil); code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var kinds []string
	for _, l := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		kinds = append(kinds, ev["event"].(string))
		if ev["event"] == "done" {
			if ev["conversation"] != "c0921b32" || ev["exit"].(float64) != 0 || !strings.HasPrefix(ev["receipt"].(string), "✓ checked") {
				t.Fatalf("done event: %v", ev)
			}
		}
	}
	if strings.Join(kinds, ",") != "tool_start,tool_done,question,question,text,done" {
		t.Fatalf("events = %v", kinds)
	}

	// At a terminal the person answers: a number picks an option, text is
	// the answer, and the run does not exit 3.
	tty := &headlessSink{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}, answers: bufio.NewReader(strings.NewReader("2\nmain.go\n"))}
	if got := tty.Ask("Which?", []string{"a", "b"}, ""); got != "b" {
		t.Fatalf("a number picks the option, got %q", got)
	}
	if got := tty.Ask("Which file?", nil, ""); got != "main.go" {
		t.Fatalf("text is the answer, got %q", got)
	}
	if tty.finish("c2", nil) != 0 {
		t.Fatal("an answered question is not exit 3")
	}

	// In a pipe without --yes a question is left to the runner's default and the run exits 3.
	var plain, perr bytes.Buffer
	p := &headlessSink{out: &plain, errOut: &perr}
	if got := p.Ask("Which?", []string{"a", "b"}, ""); got != "" {
		t.Fatalf("no --yes: no answer, got %q", got)
	}
	if !strings.Contains(perr.String(), "1. a") {
		t.Fatalf("the options are printed with the question: %q", perr.String())
	}
	p.Text("answer")
	if code := p.finish("c1", nil); code != 3 || !strings.Contains(perr.String(), "? Which?") || !strings.Contains(perr.String(), "memdoor resume c1") {
		t.Fatalf("code=%d stderr=%q", code, perr.String())
	}
	if plain.String() != "answer\n" {
		t.Fatalf("plain stdout = %q", plain.String())
	}
}

// Stdin is the input only when it is a pipe or a file; an open socket or a
// terminal is never read, so `run "…"` cannot hang on one.
func TestPromptFromArgsAndStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	w.WriteString("diff --git a b\n")
	w.Close()
	got, err := promptFromArgsAndStdin([]string{"review", "this"}, r)
	if err != nil || !strings.HasPrefix(got, "review this\n\nInput:\ndiff --git a b") {
		t.Fatalf("args + pipe: %q %v", got, err)
	}
	r2, w2, _ := os.Pipe()
	w2.WriteString("summarize the failing tests")
	w2.Close()
	if got, _ := promptFromArgsAndStdin(nil, r2); got != "summarize the failing tests" {
		t.Fatalf("pipe alone is the prompt, got %q", got)
	}
	devnull, _ := os.Open(os.DevNull) // a character device: not read
	if got, err := promptFromArgsAndStdin([]string{"hi"}, devnull); err != nil || got != "hi" {
		t.Fatalf("a device on stdin is left alone: %q %v", got, err)
	}
	if _, err := promptFromArgsAndStdin(nil, devnull); err == nil {
		t.Fatal("no args and no pipe is no prompt")
	}
}
