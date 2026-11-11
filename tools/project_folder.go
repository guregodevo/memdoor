package tools

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InProject says whether target is in the project a coder works on: under
// wd, the folder the window was started in, or under a git worktree of the
// same repository. A worktree is the same project in another folder; people
// make one per change and the coder has to follow them there (live
// 2026-11-11: refused a cd into ~/Dev/memdoor-wt-model-name from ~/Dev/memdoor,
// then built it with go -C). With no wd nothing is outside.
func InProject(wd, target string) bool {
	if wd == "" || under(wd, target) {
		return true
	}
	real := resolved(target)
	for _, tree := range worktreesOf(wd) {
		if under(tree, target) || under(resolved(tree), real) {
			return true
		}
	}
	return false
}

// ProjectFolders names the project for a refusal: wd, and its git worktrees
// when it is in a repository.
func ProjectFolders(wd string) string {
	if len(worktreesOf(wd)) > 0 {
		return wd + " and its git worktrees"
	}
	return wd
}

func under(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolved follows symlinks in the deepest part of p that exists, so a file
// not yet written still resolves (/tmp/x/new.go → /private/tmp/x/new.go).
func resolved(p string) string {
	p = filepath.Clean(p)
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest)
		}
		if filepath.Dir(dir) == dir {
			return p
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// worktreesOf is every worktree of the repository wd is in, the main one
// included; none when wd is not in a repository.
func worktreesOf(wd string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", wd, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var trees []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if path, ok := strings.CutPrefix(sc.Text(), "worktree "); ok {
			trees = append(trees, path)
		}
	}
	return trees
}
