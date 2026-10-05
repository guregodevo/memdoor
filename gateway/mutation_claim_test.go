package gateway

import (
	"encoding/json"
	"testing"

	"memdoor/tools"
)

// A reply claiming an edit with no successful mutating tool gets flagged; honest
// run-reports and backed claims do not.
func TestClaimsFileChange(t *testing.T) {
	for text, want := range map[string]bool{
		"The ASCII art has been updated with a triangle roof.":          true,
		"I added the log import and ran it.":                            true,
		"The change was to add a println after the existing one.":       true,
		"The program has been successfully built and run. It displays.": false,
		"It prints 12 as expected.":                                     false,
		"Report done: the program prints Flower as expected.":           false,
	} {
		if got := claimsFileChange(text); got != want {
			t.Errorf("claimsFileChange(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestFileMutationSucceeded(t *testing.T) {
	ok := []ToolExecutionInfo{{Name: "read_file"}, {Name: "apply_patch", Error: ""}}
	if !fileMutationSucceeded(ok) {
		t.Error("successful apply_patch should count as a mutation")
	}
	failed := []ToolExecutionInfo{{Name: "apply_patch", Error: "failed to find context"}, {Name: "bash"}}
	if fileMutationSucceeded(failed) {
		t.Error("a FAILED apply_patch must not back a change-claim")
	}
	if fileMutationSucceeded(nil) {
		t.Error("no tools = no mutation")
	}
}

// Newly-completed = completed now but not before; carried-forward completed items
// (the tool takes the FULL list every call) must not retrigger the guard.
func TestNewlyCompletedCount(t *testing.T) {
	old := []tools.TodoItem{
		{Content: "a", Status: "completed"},
		{Content: "b", Status: "in_progress"},
		{Content: "c", Status: "pending"},
	}
	now := []tools.TodoItem{
		{Content: "a", Status: "completed"}, // carried forward — not new
		{Content: "b", Status: "completed"}, // newly done
		{Content: "c", Status: "completed"}, // newly done
	}
	if got := newlyCompletedCount(old, now); got != 2 {
		t.Errorf("newlyCompletedCount = %d, want 2", got)
	}
	if got := newlyCompletedCount(nil, now); got != 3 {
		t.Errorf("from empty store, all completed are new: got %d, want 3", got)
	}
	if got := newlyCompletedCount(now, now); got != 0 {
		t.Errorf("no delta = 0, got %d", got)
	}
}

// Narrated code = a program shown in the reply instead of written: a fenced block
// with a definition keyword, or a pasted diff (3+ "+"-lines). Prose stays clean.
func TestContainsNarratedCode(t *testing.T) {
	for text, want := range map[string]bool{
		"here it is:\n```go\npackage main\n\nfunc main() { }\n```": true,
		"```python\ndef roll():\n    pass\n```":                    true,
		"+package main\n+import \"fmt\"\n+func main() {}":          true,
		"The program prints HEADS as expected.":                    false,
		"use `go run coin.go` to run it":                           false,
		"+ one added line only":                                    false,
	} {
		if got := containsNarratedCode(text); got != want {
			t.Errorf("containsNarratedCode(%q) = %v, want %v", text, got, want)
		}
	}
}

// verify without a dir must be confined to the coder workdir — it otherwise runs
// build commands in the GATEWAY's cwd (live: "stat stats.go: no such file" from a
// /tmp deploy dir). An explicit dir is respected.
func TestConfineVerifyDir(t *testing.T) {
	out := confineToCoderWorkdir("verify", []byte(`{"command":"go run stats.go"}`), "/tmp/wd")
	var in map[string]any
	if err := json.Unmarshal(out, &in); err != nil {
		t.Fatal(err)
	}
	if d, _ := in["dir"].(string); d == "" {
		t.Fatal("verify should get the coder workdir as dir")
	}
	out2 := confineToCoderWorkdir("verify", []byte(`{"command":"go test","dir":"/explicit"}`), "/tmp/wd")
	_ = json.Unmarshal(out2, &in)
	if d, _ := in["dir"].(string); d != "/explicit" {
		t.Fatalf("explicit dir must be respected, got %q", d)
	}
}
