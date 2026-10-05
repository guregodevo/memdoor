package cmd

import (
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A SESSION IN ITS OWN WORKTREE (Greg, 2026-09-27: "next item: worktree";
// MUST.md "worktrees and commits", the gap against Codex and Claude Code).
//
//	memdoor tui --worktree            a fresh worktree, named for now
//	memdoor tui --worktree fix-login  a named one, reused if it exists
//
// The agent works in <repo>/.memdoor/worktrees/<name> on branch
// memdoor/<name>, cut from HEAD. Your checkout is not touched while it works,
// and two windows on two worktrees cannot step on each other's files.
//
// The window runs INSIDE the worktree — the process changes directory before
// it starts — so every turn's workdir and the tab's name
// are the worktree's without any of them knowing worktrees exist.
//
// When the window closes it says where the work is and how to bring it back.
// A worktree nothing happened in — no edits, no commits — is removed with its
// branch, so trying the flag leaves nothing behind.
type worktree struct {
	Root    string // the main checkout
	Path    string // the worktree
	Name    string
	Branch  string
	Base    string // the commit it was cut from
	Created bool   // made by this session (a reused one is never auto-removed)
}

var worktreeNameRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// startWorktree makes (or reuses) the worktree and moves the process into it.
func startWorktree(name string) (*worktree, error) {
	root, err := gitOut("", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("--worktree needs a git repository, and this directory is not in one")
	}
	name = strings.Trim(worktreeNameRe.ReplaceAllString(strings.TrimSpace(name), "-"), "-.")
	if name == "" || name == "auto" {
		name = "session-" + time.Now().Format("0102-150405")
	}
	w := &worktree{Root: root, Name: name, Branch: "memdoor/" + name,
		Path: filepath.Join(root, shared.MemdoorDirName, "worktrees", name)}

	if _, err := os.Stat(filepath.Join(w.Path, ".git")); err == nil {
		// Reuse: the same name is the same piece of work.
		w.Base, _ = gitOut(w.Path, "rev-parse", "HEAD")
	} else {
		w.Base, err = gitOut(root, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("--worktree needs at least one commit to branch from")
		}
		args := []string{"worktree", "add", "-b", w.Branch, w.Path, "HEAD"}
		if _, berr := gitOut(root, "rev-parse", "--verify", "--quiet", w.Branch); berr == nil {
			args = []string{"worktree", "add", w.Path, w.Branch} // the branch outlived an earlier worktree
		}
		if _, err := gitOut(root, args...); err != nil {
			return nil, fmt.Errorf("could not create the worktree: %w", err)
		}
		w.Created = true
	}
	excludeWorktrees(root)
	linkWorktreeIncludes(root, w.Path)
	if err := os.Chdir(w.Path); err != nil {
		return nil, err
	}
	return w, nil
}

// excludeWorktrees keeps .memdoor/worktrees/ out of the main checkout's
// `git status` without touching the person's .gitignore: the repository's own
// info/exclude is the file git reads for exactly this.
func excludeWorktrees(root string) { excludeLocally(root, ".memdoor/worktrees/") }

// worktreeIncludeFile lists, one per line, the untracked paths a worktree
// needs to build — the convention Claude Code reads. A worktree is cut from
// what git tracks, so build output and native libraries are missing there:
// the first dogfooding turn in one (2026-09-30) could not link `go test`
// without the tokenizer library and had to build its own copy.
const worktreeIncludeFile = ".worktreeinclude"

// linkWorktreeIncludes brings each listed path of the main checkout into the
// worktree, when the checkout has it and the worktree does not. A file is
// linked (a native library is tens of megabytes); a folder is copied —
// //go:embed refuses a link ("cannot embed irregular file dist", a dogfooding
// turn, 2026-09-30) and build output is small. Each is excluded locally so it
// is never offered for a commit (a link to an ignored "dir/" is a file to
// git, and the ignore rule does not match it).
func linkWorktreeIncludes(root, path string) []string {
	b, err := os.ReadFile(filepath.Join(root, worktreeIncludeFile))
	if err != nil {
		return nil
	}
	var linked []string
	for _, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		rel := filepath.Clean(strings.TrimSuffix(t, "/"))
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		src, dst := filepath.Join(root, rel), filepath.Join(path, rel)
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if os.MkdirAll(filepath.Dir(dst), 0o755) != nil {
			continue
		}
		if fi.IsDir() {
			if os.CopyFS(dst, os.DirFS(src)) != nil {
				_ = os.RemoveAll(dst)
				continue
			}
		} else if os.Symlink(src, dst) != nil {
			continue
		}
		excludeLocally(root, "/"+filepath.ToSlash(rel))
		linked = append(linked, rel)
	}
	return linked
}

// excludeLocally adds line to the repository's info/exclude once.
func excludeLocally(root, line string) {
	common, err := gitOut(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	path := filepath.Join(common, "info", "exclude")
	b, _ := os.ReadFile(path)
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == line {
			return
		}
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString(line + "\n")
}

// finish is what the person reads when the window closes.
func (w *worktree) finish() string {
	dirty, _ := gitOut(w.Path, "status", "--porcelain")
	ahead, _ := gitOut(w.Path, "rev-list", "--count", w.Base+"..HEAD")
	if strings.TrimSpace(dirty) == "" && (ahead == "" || ahead == "0") {
		if w.Created {
			_, _ = gitOut(w.Root, "worktree", "remove", w.Path)
			_, _ = gitOut(w.Root, "branch", "-D", w.Branch)
			return fmt.Sprintf("Nothing changed in worktree %s, so it and its branch were removed.", w.Name)
		}
		return fmt.Sprintf("Nothing changed in worktree %s. It is still at %s.", w.Name, w.Path)
	}
	var what []string
	if ahead != "" && ahead != "0" {
		what = append(what, ahead+" commit(s)")
	}
	if strings.TrimSpace(dirty) != "" {
		what = append(what, "uncommitted changes")
	}
	return fmt.Sprintf(`The work is on branch %s (%s), in %s
  continue:    memdoor tui --worktree %s
  bring back:  cd %s && git add -A && git commit -m "…"  then, in your checkout:  git merge %s
  throw away:  git worktree remove --force %s && git branch -D %s`,
		w.Branch, strings.Join(what, " and "), w.Path, w.Name, w.Path, w.Branch, w.Path, w.Branch)
}

// gitOut runs git in dir ("" = the current directory) and returns trimmed
// stdout, or an error carrying git's own words.
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
