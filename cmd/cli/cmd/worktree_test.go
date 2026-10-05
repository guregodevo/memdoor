package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo is a throwaway git repository with one commit, and the process moved
// into it the way a person would be standing in their project.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "first")
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	return real
}

// A SESSION IN ITS OWN WORKTREE: made from HEAD on its own branch, the process
// inside it, the main checkout left clean — and an untouched worktree cleans
// itself up, so trying the flag leaves nothing behind.
func TestAWorktreeSessionThatChangedNothingLeavesNothing(t *testing.T) {
	root := newRepo(t)
	w, err := startWorktree("try-it")
	if err != nil {
		t.Fatal(err)
	}
	if w.Branch != "memdoor/try-it" || w.Path != filepath.Join(root, ".memdoor", "worktrees", "try-it") {
		t.Fatalf("worktree %+v", w)
	}
	if cwd, _ := os.Getwd(); cwd != w.Path {
		if real, _ := filepath.EvalSymlinks(cwd); real != w.Path {
			t.Fatalf("the window must run inside the worktree, cwd=%s", cwd)
		}
	}
	// The main checkout does not see the worktree as untracked files.
	if st, _ := gitOut(root, "status", "--porcelain"); st != "" {
		t.Fatalf("the main checkout must stay clean, git status: %q", st)
	}
	msg := w.finish()
	if !strings.Contains(msg, "were removed") {
		t.Fatalf("nothing changed, so it goes: %s", msg)
	}
	if _, err := os.Stat(w.Path); !os.IsNotExist(err) {
		t.Error("the worktree directory must be gone")
	}
	if _, err := gitOut(root, "rev-parse", "--verify", "--quiet", w.Branch); err == nil {
		t.Error("the branch must be gone")
	}
}

// Work that happened is never thrown away: the worktree stays, and the message
// says where it is and the three things you can do with it.
func TestAWorktreeWithWorkIsKeptAndExplained(t *testing.T) {
	root := newRepo(t)
	w, err := startWorktree("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(w.Name, "session-") {
		t.Errorf("an unnamed worktree gets a name from the clock, got %q", w.Name)
	}
	if err := os.WriteFile(filepath.Join(w.Path, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg := w.finish()
	for _, want := range []string{w.Branch, "uncommitted changes", "git merge " + w.Branch, "git worktree remove", "--worktree " + w.Name} {
		if !strings.Contains(msg, want) {
			t.Errorf("the hand-back should mention %q:\n%s", want, msg)
		}
	}
	if _, err := os.Stat(w.Path); err != nil {
		t.Fatal("work must never be removed")
	}
	// The main checkout never saw the edit.
	if b, _ := os.ReadFile(filepath.Join(root, "main.go")); string(b) != "package main\n" {
		t.Errorf("the edit leaked into the main checkout: %q", b)
	}
	// And the same name reopens the same work.
	_ = os.Chdir(root)
	again, err := startWorktree(w.Name)
	if err != nil || again.Path != w.Path || again.Created {
		t.Fatalf("reopening the same name must reuse the worktree: %+v %v", again, err)
	}
}

// Outside a repository the flag says so, in words, before any window opens.
func TestWorktreeOutsideARepositoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(dir)
	if _, err := startWorktree("x"); err == nil || !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("want a plain refusal, got %v", err)
	}
}

// A worktree is cut from what git tracks; .worktreeinclude brings the
// untracked paths it needs to build (the first dogfooding turn in one could
// not link `go test` without llm/lib — the path is this test's example, the
// real one having gone with the local engine, 2026-10-04). Linked, never
// offered for a commit, and nothing outside the checkout is followed.
func TestAWorktreeLinksWhatWorktreeincludeLists(t *testing.T) {
	root := newRepo(t)
	_ = os.MkdirAll(filepath.Join(root, "llm", "lib"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "llm", "lib", "libx.a"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "big.bin"), []byte("b"), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("llm/lib/\nbig.bin\n.worktreeinclude\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".worktreeinclude"), []byte("# build deps\nllm/lib/\nbig.bin\nmissing/dir\n../escape\n/abs\n"), 0o644)
	w, err := startWorktree("deps")
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(w.Path, "llm", "lib", "libx.a")); err != nil || string(b) != "x" {
		t.Fatalf("llm/lib not reachable in the worktree: %v", err)
	}
	// A folder is copied: //go:embed refuses a link ("cannot embed
	// irregular file dist"). A file is linked.
	if fi, err := os.Lstat(filepath.Join(w.Path, "llm", "lib")); err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("llm/lib must be a real folder, a copy: %v", err)
	}
	if fi, err := os.Lstat(filepath.Join(w.Path, "big.bin")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("big.bin must be a link: %v", err)
	}
	for _, p := range []string{"missing", filepath.Join("..", "escape")} {
		if _, err := os.Lstat(filepath.Join(w.Path, p)); err == nil {
			t.Errorf("%s must not be created", p)
		}
	}
	if st, _ := gitOut(w.Path, "status", "--porcelain"); strings.Contains(st, "llm") {
		t.Fatalf("the link is offered for a commit: %q", st)
	}
}
