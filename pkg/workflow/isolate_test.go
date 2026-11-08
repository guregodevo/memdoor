package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A worktree starts from the project as it stands (an uncommitted edit, an
// untracked file), shares its .memdoor, answers only what the task changed,
// and leaves the project as it was.
func TestAWorktreeIsolationStartsFromTheTreeAsItStands(t *testing.T) {
	p := t.TempDir()
	gitT(t, p, "init", "-q")
	write(t, filepath.Join(p, "a.txt"), "one\n")
	gitT(t, p, "add", "a.txt")
	gitT(t, p, "commit", "-q", "-m", "base")
	write(t, filepath.Join(p, "a.txt"), "one, uncommitted\n")
	write(t, filepath.Join(p, "new.txt"), "untracked\n")
	write(t, filepath.Join(p, ".memdoor", "runs", "r", "task.md"), "the task\n")

	iso, err := NewIsolator().Isolate(context.Background(), p, "direct-1")
	if err != nil {
		t.Fatal(err)
	}
	d := iso.Dir()
	if b, _ := os.ReadFile(filepath.Join(d, "a.txt")); string(b) != "one, uncommitted\n" {
		t.Fatalf("a.txt in the copy = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "new.txt")); string(b) != "untracked\n" {
		t.Fatalf("new.txt in the copy = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(d, ".memdoor", "runs", "r", "task.md")); string(b) != "the task\n" {
		t.Fatalf("the run folder is not shared: %q", b)
	}
	if ch, err := iso.Changes(context.Background()); err != nil || strings.TrimSpace(ch) != "" {
		t.Fatalf("no change yet, got %q, %v", ch, err)
	}
	write(t, filepath.Join(d, "a.txt"), "two\n")
	write(t, filepath.Join(d, "pkg", "added.go"), "package pkg\n")
	write(t, filepath.Join(d, ".memdoor", "runs", "r", "attempt-1"), "#!/bin/sh\n")
	ch, err := iso.Changes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ch, "-one, uncommitted") || !strings.Contains(ch, "+two") || !strings.Contains(ch, "pkg/added.go") {
		t.Fatalf("change = %q", ch)
	}
	if strings.Contains(ch, ".memdoor") {
		t.Fatalf("the run folder is never part of a change: %q", ch)
	}
	if b, _ := os.ReadFile(filepath.Join(p, ".memdoor", "runs", "r", "attempt-1")); string(b) != "#!/bin/sh\n" {
		t.Fatal("what the attempt wrote in the run folder is in the project's")
	}
	if b, _ := os.ReadFile(filepath.Join(p, "a.txt")); string(b) != "one, uncommitted\n" {
		t.Fatalf("the project changed: %q", b)
	}
	// The change applies to the project as it stands.
	cmd := exec.Command("git", "-C", p, "apply", "--check", "-")
	cmd.Stdin = strings.NewReader(ch)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the change does not apply to the project: %v\n%s", err, out)
	}
	if err := iso.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Fatal("the copy was not removed")
	}
	out, _ := exec.Command("git", "-C", p, "worktree", "list").Output()
	if strings.Count(string(out), "\n") != 1 {
		t.Fatalf("worktree left behind:\n%s", out)
	}
}

func TestAProjectOutsideGitIsCopied(t *testing.T) {
	p := t.TempDir()
	write(t, filepath.Join(p, "a.txt"), "one\n")
	iso, err := NewIsolator().Isolate(context.Background(), p, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer iso.Close()
	write(t, filepath.Join(iso.Dir(), "a.txt"), "two\n")
	ch, err := iso.Changes(context.Background())
	if err != nil || !strings.Contains(ch, "+two") {
		t.Fatalf("change = %q, %v", ch, err)
	}
	if b, _ := os.ReadFile(filepath.Join(p, "a.txt")); string(b) != "one\n" {
		t.Fatal("the project changed")
	}
}

func TestARepositoryWithNoCommitIsCopied(t *testing.T) {
	p := t.TempDir()
	gitT(t, p, "init", "-q")
	write(t, filepath.Join(p, "task.md"), "the task\n")
	iso, err := NewIsolator().Isolate(context.Background(), p, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer iso.Close()
	write(t, filepath.Join(iso.Dir(), "solution.py"), "print(1)\n")
	ch, err := iso.Changes(context.Background())
	if err != nil || !strings.Contains(ch, "solution.py") || strings.Contains(ch, "task.md") {
		t.Fatalf("change = %q, %v", ch, err)
	}
}
