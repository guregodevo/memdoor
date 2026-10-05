package gateway

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"memdoor/tools"
)

// agent_runtime_guards: per-turn state keys and the honesty guards — did a claimed edit actually happen.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// turnMutatedKey carries a *bool through ctx: whether any file-mutating tool has
// succeeded in the current turn. Set by the tool loop, read by the todo_write
// special-case (status-theater guard).
type turnMutatedKey struct{}

// retryTemperatureKey carries a one-call sampling override for the empty-step
// retry — a deterministic re-sample just reproduces the empty completion.
type retryTemperatureKey struct{}

// turnReadsKey carries the set of file base names READ or WRITTEN this turn —
// the ground the model has actually seen. Injected into apply_patch input so it
// can refuse a blind whole-file rewrite (see ApplyPatch's overwrite guard).
type turnReadsKey struct{}

// touchedFilesRe extracts file names from an apply_patch result's
// "added=[a b] modified=[c]" summary.
var touchedFilesRe = regexp.MustCompile(`(?:added|modified)=\[([^\]]*)\]`)

// recordTurnReads notes the files a successful tool call read or wrote, so a
// later whole-file rewrite in the SAME turn is grounded, not from memory.
func recordTurnReads(reads map[string]bool, tool string, input json.RawMessage, output string) {
	switch tool {
	case "read_file":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(input, &p) == nil && p.Path != "" {
			reads[filepath.Base(p.Path)] = true
		}
	case "apply_patch":
		for _, m := range touchedFilesRe.FindAllStringSubmatch(output, -1) {
			for _, f := range strings.Fields(m[1]) {
				reads[filepath.Base(f)] = true
			}
		}
	}
}

// newlyCompletedCount counts items completed in the NEW list that were not
// completed in the OLD one (matched by content) — the delta a completion-claim
// must be backed by. Items carried forward already-completed don't count, so the
// full-list-every-call contract doesn't retrigger the guard.
func newlyCompletedCount(old, new []tools.TodoItem) int {
	done := map[string]bool{}
	for _, t := range old {
		if t.Status == "completed" {
			done[t.Content] = true
		}
	}
	n := 0
	for _, t := range new {
		if t.Status == "completed" && !done[t.Content] {
			n++
		}
	}
	return n
}

// fileChangeClaims are the past-tense verbs a reply uses to assert an edit
// happened. Deliberately narrow (no "built"/"ran" — those are about running) so
// an honest run-report never gets the note.
var fileChangeClaims = []string{
	"has been updated", "has been added", "has been changed", "has been modified",
	"has been replaced", "has been removed", "has been fixed", "has been created",
	"i updated", "i added", "i changed", "i modified", "i replaced", "i removed",
	"i fixed", "i created", "i rewrote",
	"the change was", "code has been updated", "file has been updated",
}

// claimsFileChange reports whether the reply asserts that an edit was made.
func claimsFileChange(text string) bool {
	t := strings.ToLower(text)
	for _, c := range fileChangeClaims {
		if strings.Contains(t, c) {
			return true
		}
	}
	return false
}

// containsNarratedCode reports whether the reply DISPLAYS a program instead of
// writing it: a fenced code block carrying a definition keyword, or a run of 3+
// "+"-prefixed lines (a pasted diff). Prose and short inline snippets don't match.
func containsNarratedCode(text string) bool {
	if i := strings.Index(text, "```"); i >= 0 {
		if j := strings.Index(text[i+3:], "```"); j >= 0 {
			block := text[i+3 : i+3+j]
			for _, kw := range []string{"package main", "func main", "def ", "function ", "console.log", "import "} {
				if strings.Contains(block, kw) {
					return true
				}
			}
		}
	}
	run := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "+") {
			run++
			if run >= 3 {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}

// fileMutationSucceeded reports whether any file-mutating tool call succeeded in
// the turn — the ground truth a change-claim must be backed by.
func fileMutationSucceeded(tools []ToolExecutionInfo) bool {
	for _, t := range tools {
		switch t.Name {
		case "apply_patch", "write_file", "edit_file", "search_replace":
			if t.Error == "" {
				return true
			}
		}
	}
	return false
}
