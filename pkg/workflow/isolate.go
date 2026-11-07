package workflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/guregodevo/mario/tasks"
)

// NewIsolator is the isolation a workflow's `isolate: true` task and each
// fan-out attempt run in: a working directory of their own, a copy of the
// project as it stands, uncommitted changes and untracked files included. In
// a git repository it is a worktree (cheap: the objects are shared); outside
// one, a copy. Either way the copy's state is committed as its base, so its
// change is a plain diff, and the project's .memdoor is linked in: the run's
// folder, its scripts and its outputs are the same for every attempt, and
// what an attempt writes there (its check, its notes) is where the steps
// after it look.
func NewIsolator() tasks.Isolator { return gitIsolator{} }

type gitIsolator struct{}

type gitIsolation struct {
	dir      string // where the task runs (a subdirectory of root when the project is one)
	root     string // the worktree or copy
	base     string // the commit its change is measured from
	worktree string // the repository it is a worktree of, "" for a copy
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// excluded is what never belongs to a change: the run's own folder and
// Python's caches.
var excluded = []string{".", ":(exclude).memdoor", ":(exclude,glob)**/__pycache__/**"}

func (gitIsolator) Isolate(ctx context.Context, dir, name string) (tasks.Isolation, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	parent, err := os.MkdirTemp("", "memdoor-isolated-")
	if err != nil {
		return nil, err
	}
	root := filepath.Join(parent, unsafeName.ReplaceAllString(name, "_"))
	iso := &gitIsolation{root: root}
	fail := func(err error) (tasks.Isolation, error) {
		_ = iso.Close()
		_ = os.RemoveAll(parent)
		return nil, err
	}
	top, gitErr := git(ctx, dir, "rev-parse", "--show-toplevel")
	// A repository with no commit yet has no HEAD to make a worktree of: it
	// is copied like a project outside git.
	if gitErr == nil {
		if _, err := git(ctx, dir, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
			gitErr = err
		}
	}
	if gitErr == nil && strings.TrimSpace(top) != "" {
		top = strings.TrimSpace(top)
		prefix, _ := git(ctx, dir, "rev-parse", "--show-prefix")
		if _, err := git(ctx, top, "worktree", "add", "--detach", root, "HEAD"); err != nil {
			return fail(fmt.Errorf("git worktree: %w", err))
		}
		iso.worktree = top
		iso.dir = filepath.Join(root, strings.TrimSpace(prefix))
		// The tree as it stands, not HEAD: what is uncommitted is the
		// task's starting point too.
		if diff, err := git(ctx, top, append([]string{"diff", "--binary", "HEAD", "--"}, excluded...)...); err != nil {
			return fail(err)
		} else if strings.TrimSpace(diff) != "" {
			cmd := exec.CommandContext(ctx, "git", "-C", root, "apply", "--binary", "--whitespace=nowarn", "-")
			cmd.Stdin = strings.NewReader(diff)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fail(fmt.Errorf("replaying the uncommitted changes: %v: %s", err, out))
			}
		}
		untracked, err := git(ctx, top, append([]string{"ls-files", "--others", "--exclude-standard", "-z", "--"}, excluded...)...)
		if err != nil {
			return fail(err)
		}
		for _, f := range strings.Split(untracked, "\x00") {
			if f == "" {
				continue
			}
			if err := copyFile(filepath.Join(top, f), filepath.Join(root, f)); err != nil {
				return fail(fmt.Errorf("copying %s: %w", f, err))
			}
		}
	} else {
		if err := copyTree(dir, root); err != nil {
			return fail(fmt.Errorf("copying the project: %w", err))
		}
		iso.dir = root
		if _, err := git(ctx, root, "init", "-q"); err != nil {
			return fail(err)
		}
	}
	// The run's folder is the project's: one for every attempt.
	if _, err := os.Stat(filepath.Join(dir, ".memdoor")); err == nil {
		_ = os.RemoveAll(filepath.Join(iso.dir, ".memdoor"))
		if err := os.Symlink(filepath.Join(dir, ".memdoor"), filepath.Join(iso.dir, ".memdoor")); err != nil {
			return fail(err)
		}
	}
	if _, err := git(ctx, root, append([]string{"add", "-A", "--"}, excluded...)...); err != nil {
		return fail(err)
	}
	if _, err := git(ctx, root, "-c", "user.name=memdoor", "-c", "user.email=memdoor@localhost", "commit", "-q", "--no-verify", "--allow-empty", "-m", "isolation base"); err != nil {
		return fail(err)
	}
	base, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return fail(err)
	}
	iso.base = strings.TrimSpace(base)
	return iso, nil
}

func (i *gitIsolation) Dir() string { return i.dir }

// Changes is the task's change in its copy, new files included, paths
// relative to the project's directory: a patch git apply takes there.
func (i *gitIsolation) Changes(ctx context.Context) (string, error) {
	if _, err := git(ctx, i.dir, append([]string{"add", "-A", "-N", "--"}, excluded...)...); err != nil {
		return "", err
	}
	return git(ctx, i.dir, append([]string{"diff", "--binary", "--relative", i.base, "--"}, excluded...)...)
}

// Close removes the copy, and the worktree from its repository's list.
func (i *gitIsolation) Close() error {
	if i.worktree != "" {
		_, _ = git(context.Background(), i.worktree, "worktree", "remove", "--force", i.root)
		_, _ = git(context.Background(), i.worktree, "worktree", "prune")
	}
	return os.RemoveAll(filepath.Dir(i.root))
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// copyTree copies a project that is not a git repository, or one with no
// commit yet: its .memdoor left out (it is linked in), its .git too (the copy
// gets its own).
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		for _, skip := range []string{".memdoor", ".git"} {
			if rel == skip || strings.HasPrefix(rel, skip+string(filepath.Separator)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}
