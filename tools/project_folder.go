package tools

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"memdoor/pkg/shared"
)

// InProject says whether target is in the project a coder works on: under
// wd, the folder the window was started in, or under a git worktree of the
// same repository. A worktree is the same project in another folder; people
// make one per change and the coder has to follow them there (live
// 2026-11-11: refused a cd into ~/Dev/memdoor-wt-model-name from ~/Dev/memdoor,
// then built it with go -C). With no wd nothing is outside.
//
// A workflow's isolated copy (a worktree locked with
// shared.IsolatedWorktreeLock) is sealed both ways: a turn in it writes only
// there, and a turn elsewhere never writes into it. Without that, an attempt
// of a fan-out wrote its fix into the project's own tree (django-11400,
// 2026-11-12).
func InProject(wd, target string) bool {
	if wd == "" {
		return true
	}
	trees := worktreesOf(wd)
	if iso, ok := isolatedTree(wd, trees); ok {
		return in(iso, target)
	}
	if under(wd, target) {
		return true
	}
	for _, tree := range trees {
		if !tree.isolated && in(tree.path, target) {
			return true
		}
	}
	return false
}

// ProjectFolders names the project for a refusal: wd, its isolated copy when
// it is in one, or wd and its git worktrees when it is in a repository.
func ProjectFolders(wd string) string {
	trees := worktreesOf(wd)
	if iso, ok := isolatedTree(wd, trees); ok {
		return iso + " (an isolated copy of the project)"
	}
	if len(trees) > 0 {
		return wd + " and its git worktrees"
	}
	return wd
}

// in is under, with symlinks resolved on both sides (/tmp → /private/tmp).
func in(root, target string) bool {
	return under(root, target) || under(resolved(root), resolved(target))
}

// isolatedTree is the isolated worktree wd is in, if any.
func isolatedTree(wd string, trees []worktree) (string, bool) {
	for _, tree := range trees {
		if tree.isolated && in(tree.path, wd) {
			return tree.path, true
		}
	}
	return "", false
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

type worktree struct {
	path     string
	isolated bool // locked with shared.IsolatedWorktreeLock
}

// worktreesOf is every worktree of the repository wd is in, the main one
// included; none when wd is not in a repository.
func worktreesOf(wd string) []worktree {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", wd, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var trees []worktree
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			trees = append(trees, worktree{path: path})
		} else if reason, ok := strings.CutPrefix(line, "locked "); ok && len(trees) > 0 && reason == shared.IsolatedWorktreeLock {
			trees[len(trees)-1].isolated = true
		}
	}
	return trees
}
