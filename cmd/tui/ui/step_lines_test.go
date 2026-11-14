package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A running step's command uses the window's width (2026-11-14, "too
// narrow"): a graph was drawn at a panel's 100 columns, so a 140-column
// command was cut at 90 in a 210-column window.
func TestAStepsCommandUsesTheWindowsWidth(t *testing.T) {
	cmd := "$ cat .memdoor/runs/qa-2026-11-14T234227/open.txt 2>/dev/null; echo ---; ls -la .memdoor/runs/qa-2026-11-14T234227/ | head -40"
	r := WorkflowRun{ID: "r1", Workflow: "qa", State: "running", Total: 9}
	for _, n := range []string{"context", "areas", "pick", "explore", "confirm", "dedupe", "settle", "file", "done"} {
		st := "waiting"
		if n == "explore" {
			st = "running"
		}
		r.Tasks = append(r.Tasks, WorkflowTask{Name: n, State: st})
	}
	r.Tasks[3].addLine("c1", cmd)
	for _, expanded := range []bool{false, true} {
		got := ansi.Strip(workflowGraphMessage(r, graphWidth(210), expanded).Content)
		if !strings.Contains(got, cmd) {
			t.Fatalf("expanded=%v: the command is cut in a 210-column window:\n%s", expanded, got)
		}
	}
	if got := ansi.Strip(workflowGraphMessage(r, graphWidth(80), false).Content); strings.Contains(got, cmd) || !strings.Contains(got, "$ cat .memdoor/runs/qa-") {
		t.Fatalf("an 80-column window cuts it to fit:\n%s", got)
	}
}

// Each call wears its tool's hue, the newest call of a running step is in body
// text and the ones before in grey, a failed exit in red (2026-11-14: "same
// color", "i cant see clearly").
func TestAStepsLinesSayWhatEachCallIs(t *testing.T) {
	inColour(t)
	task := WorkflowTask{Name: "explore", State: "running"}
	task.addLine("a", "read answer.md")
	task.addLine("b", "$ go test ./... → exit 1")
	task.addLine("c", "$ git log -20 --oneline · 4s")
	got := stepLinesShownOf(task, true, 200)
	if len(got) != 3 {
		t.Fatalf("three calls, three lines: %q", got)
	}
	for _, c := range []struct {
		line int
		want []string
	}{
		{0, []string{fg(colRead) + "read ", fg(colDim) + "answer.md"}},
		{1, []string{fg(colRun) + "$ ", fg(colDim) + "go test ./...", fg(colErr) + " → exit 1"}},
		{2, []string{fg(colRun) + "$ ", fg(colText) + "git log -20 --oneline", fg(colDim) + " · 4s"}},
	} {
		for _, w := range c.want {
			if !strings.Contains(got[c.line], w) {
				t.Fatalf("line %d: no %q in %q", c.line, w, got[c.line])
			}
		}
	}
	done := WorkflowTask{Name: "x", State: "done"}
	done.addLine("a", "✎ answer.md")
	done.addLine("b", "$ go test ./... → exit 0")
	got = stepLinesShownOf(done, true, 200)
	if !strings.Contains(got[0], fg(colWrite)+"✎ ") || !strings.Contains(got[1], fg(colDim)+"go test") || !strings.Contains(got[1], fg(colDim)+" → exit 0") {
		t.Fatalf("a finished step's calls are grey, exit 0 too: %q", got)
	}
	if more := stepLineShown("… 4 more (ctrl+o)", false); !strings.HasPrefix(more, fg(colDim)) {
		t.Fatalf("the count stays grey: %q", more)
	}
}
