package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// The shell's view of a run: one line per task with its glyph, its time,
// what it requires, and the reason it failed.
func TestPrintWorkflowTasks(t *testing.T) {
	r := workflowRunStatus{Tasks: []workflowTaskStatus{
		{Name: "tests", State: "skipped"},
		{Name: "notes", State: "done", Took: "11s", Requires: []string{"tests"}},
		{Name: "approve", State: "waiting", External: true},
		{Name: "announce", State: "failed", Error: "exit 1", Requires: []string{"notes", "approve"}},
	}}
	old := os.Stdout
	rd, w, _ := os.Pipe()
	os.Stdout = w
	printWorkflowTasks(r)
	w.Close()
	os.Stdout = old
	var b bytes.Buffer
	io.Copy(&b, rd)
	out := b.String()
	for _, want := range []string{"✓ tests     skipped", "✓ notes     done 11s  ← tests", "○ approve   waiting  (external)", "✗ announce  failed  ← notes, approve", "      exit 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
