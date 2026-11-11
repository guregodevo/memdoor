package gateway

import (
	"encoding/json"
	"memdoor/tools"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const wd = "/Users/you/memdoor-coder"

func bashCmd(t *testing.T, cmd string) (command, cwd string) {
	t.Helper()
	in, _ := json.Marshal(map[string]any{"command": cmd})
	out := confineToCoderWorkdir("bash", in, wd)
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	c, _ := got["command"].(string)
	d, _ := got["cwd"].(string)
	return c, d
}

// The cd prefix was only a DEFAULT directory, not a fence: the model's own cd
// runs after it and wins.
//
// Observed 2026-08-29:
//
//	cd '/Users/you/memdoor-coder' && cd /Users/you/.memdoor/workdir/tetris && go run stats.go
//
// a path the model invented, present in no prompt or skill in this repo. Two
// tool calls were wasted on the failure — and had it invented a path that
// EXISTS, the command would have run there. That is the likely mechanism behind
// the stray main.go written into the repo root and gateway/client/ (AGENTS.md,
// "three times this weekend"), because the coder runs always-auto with no
// permission prompt.
func TestBashCannotCdOutOfTheWorkdir(t *testing.T) {
	got, _ := bashCmd(t, "cd /Users/you/.memdoor/workdir/tetris && go run stats.go")
	if strings.Contains(got, "go run stats.go") {
		t.Errorf("the escaping command must not be executed at all, got: %s", got)
	}
	if !strings.Contains(got, "refused") {
		t.Errorf("the refusal must say why, got: %s", got)
	}
	if !strings.Contains(got, "/Users/you/.memdoor/workdir/tetris") {
		t.Errorf("the refusal must name the offending path so the model can correct it: %s", got)
	}
	// It must tell the model what to do instead, or it will simply try again.
	if !strings.Contains(got, "drop the cd") {
		t.Errorf("the refusal must redirect, not just deny: %s", got)
	}
}

// Live 2026-11-11: a coder sent to a sibling git worktree was told "You are
// ALREADY in the right directory" (it was not: the work was in the other
// checkout), and the refusal came back with the note for a read whose exit
// status is an answer ("grep found no match…"). The refusal is the whole
// result: printed, exit 0, and it says what to do when the folder is the right
// one.
func TestBashCdRefusalIsTheWholeResult(t *testing.T) {
	got, _ := bashCmd(t, "cd /Users/you/memdoor-wt-x && go build ./... 2>&1 | tail; echo DONE")
	if strings.Contains(got, "ALREADY in the right directory") {
		t.Errorf("the refusal must not claim the workdir is where the work is: %s", got)
	}
	if !strings.Contains(got, "start memdoor in that folder") {
		t.Errorf("the refusal must say how to work in the other folder: %s", got)
	}
	out, err := exec.Command("bash", "-c", got).Output()
	if err != nil {
		t.Fatalf("the refusal must exit 0 so it is neither a failed command nor a read's status: %v", err)
	}
	if strings.Contains(string(out), "DONE") || !strings.HasPrefix(string(out), "refused: ") {
		t.Errorf("only the refusal may run: %q", out)
	}
	if !tools.IsBashRead(got) {
		t.Errorf("the printed refusal must count as a read, not a check: %s", got)
	}
}

// Legitimate movement inside the workdir must be untouched — a coder navigating
// its own project is the normal case, and breaking it would be worse than the
// bug.
func TestBashMayMoveWithinTheWorkdir(t *testing.T) {
	for _, cmd := range []string{
		"cd internal/server && go build ./...",  // relative
		"cd " + wd + " && go test ./...",        // the root itself
		"cd " + wd + "/pkg/foo && go vet ./...", // a subdirectory
		"ls -la && go run .",                    // no cd at all
		"grep -rn 'cd /etc' .",                  // the string appears but is not a cd
	} {
		got, cwd := bashCmd(t, cmd)
		if strings.Contains(got, "refused") {
			t.Errorf("must not refuse legitimate command %q, got: %s", cmd, got)
		}
		// The command is passed through UNCHANGED — no shell rewriting — and
		// the workdir travels as cwd, which the tool applies via cmd.Dir.
		if got != cmd {
			t.Errorf("the command must not be rewritten: %q -> %q", cmd, got)
		}
		if cwd != wd {
			t.Errorf("the workdir must travel as cwd: %q -> cwd %q, want %q", cmd, cwd, wd)
		}
	}
}

// A cd anywhere in the chain escapes, not just a leading one.
func TestBashCatchesALaterCd(t *testing.T) {
	got, _ := bashCmd(t, "go build ./... ; cd /etc && cat passwd")
	if !strings.Contains(got, "refused") {
		t.Errorf("a cd later in the chain still leaves the workdir: %s", got)
	}
}

// Reading an absolute path is still allowed — that is the deliberate "may
// target a real project on purpose" behaviour, and only DIRECTORY CHANGES are
// what make the confinement meaningless.
func TestAbsolutePathReadsAreStillAllowed(t *testing.T) {
	got, cwd := bashCmd(t, "cat /etc/hosts")
	if strings.Contains(got, "refused") {
		t.Errorf("reading an absolute path must still work: %s", got)
	}
	if cwd != wd {
		t.Errorf("it must still run in the workdir: cwd %q", cwd)
	}
}

// A git worktree of the project is the project in another folder: people make
// one per change, and the coder works, builds and runs there with them.
func TestTheCoderWorksInAWorktreeOfItsProject(t *testing.T) {
	tmp := t.TempDir()
	repo, tree := filepath.Join(tmp, "repo"), filepath.Join(tmp, "repo-wt")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base"},
		{"-C", repo, "worktree", "add", "-q", "-b", "change", tree},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cmd := "cd " + tree + " && go build ./... && go test ./..."
	in, _ := json.Marshal(map[string]any{"command": cmd})
	var got map[string]any
	_ = json.Unmarshal(confineToCoderWorkdir("bash", in, repo), &got)
	if got["command"] != cmd {
		t.Errorf("a cd into a worktree of the project must run: %v", got["command"])
	}
	write, _ := json.Marshal(map[string]any{"path": filepath.Join(tree, "x.go"), "content": "x"})
	if msg := coderPathEscape("write_file", write, repo); msg != "" {
		t.Errorf("a write into a worktree of the project must go through: %s", msg)
	}
	other, _ := json.Marshal(map[string]any{"command": "cd " + tmp + " && ls"})
	_ = json.Unmarshal(confineToCoderWorkdir("bash", other, repo), &got)
	if c, _ := got["command"].(string); !strings.Contains(c, "refused") {
		t.Errorf("the folder holding the worktrees is not the project: %s", c)
	}
}
