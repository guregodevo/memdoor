package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"memdoor/pkg/shared"
)

// gitWorktree makes a repository with one commit at <tmp>/repo and a worktree
// of it at <tmp>/repo-wt, the layout people use for one change per folder.
func gitWorktree(t *testing.T) (tmp, repo, tree string) {
	t.Helper()
	tmp = t.TempDir()
	repo, tree = filepath.Join(tmp, "repo"), filepath.Join(tmp, "repo-wt")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base"},
		{"-C", repo, "worktree", "add", "-q", "-b", "change", tree},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return tmp, repo, tree
}

func TestAWorktreeIsTheSameProject(t *testing.T) {
	tmp, repo, tree := gitWorktree(t)
	if err := os.Mkdir(filepath.Join(tmp, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]bool{
		repo:                                 true,
		filepath.Join(repo, "a.go"):          true,
		tree:                                 true,
		filepath.Join(tree, "pkg", "new.go"): true, // not written yet
		filepath.Join(tmp, "other"):          false,
		filepath.Join(tmp, "other", "x.go"):  false,
		filepath.Join(tree, "..", "other", "x.go"): false,
		tmp: false,
	} {
		if got := InProject(repo, target); got != want {
			t.Errorf("InProject(repo, %s) = %v, want %v", target, got, want)
		}
	}
	// From the worktree, the main checkout is the same project too.
	if !InProject(tree, filepath.Join(repo, "a.go")) {
		t.Error("the main checkout is a worktree of the same repository")
	}
}

func TestOutsideARepositoryOnlyTheFolderIsTheProject(t *testing.T) {
	tmp := t.TempDir()
	wd := filepath.Join(tmp, "plain")
	if err := os.Mkdir(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	if !InProject(wd, filepath.Join(wd, "x")) || InProject(wd, filepath.Join(tmp, "y")) {
		t.Error("with no repository the project is the folder alone")
	}
	if !InProject("", "/anywhere") {
		t.Error("no folder means no fence")
	}
}

func TestApplyPatchWritesInAWorktree(t *testing.T) {
	_, repo, tree := gitWorktree(t)
	if !outsideProject(repo, filepath.Join(tree+"-gone", "x.go")) {
		t.Error("a folder beside the worktree is outside")
	}
	if outsideProject(repo, filepath.Join(tree, "x.go")) {
		t.Error("a patch into a worktree of the project must be written")
	}
}

// A refusal names the worktrees only where there are some.
func TestARefusalNamesTheWorktreesOnlyInARepository(t *testing.T) {
	_, repo, _ := gitWorktree(t)
	if got := ProjectFolders(repo); got != repo+" and its git worktrees" {
		t.Errorf("in a repository: %q", got)
	}
	plain := t.TempDir()
	if got := ProjectFolders(plain); got != plain {
		t.Errorf("outside one: %q", got)
	}
}

// A workflow's isolated copy is sealed both ways: a turn in it writes only
// there (not into the project's tree, not into another attempt's copy), and
// a turn elsewhere never writes into it. A person's worktree still counts
// (django-11400, 2026-11-12: an attempt wrote its fix into /testbed).
func TestAnIsolatedCopyIsSealed(t *testing.T) {
	tmp, repo, tree := gitWorktree(t)
	iso, other := filepath.Join(tmp, "iso-1"), filepath.Join(tmp, "iso-2")
	for _, p := range []string{iso, other} {
		for _, args := range [][]string{
			{"-C", repo, "worktree", "add", "-q", "--detach", p, "HEAD"},
			{"-C", repo, "worktree", "lock", "--reason", shared.IsolatedWorktreeLock, p},
		} {
			if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
	for _, c := range []struct {
		wd, target string
		want       bool
	}{
		{iso, filepath.Join(iso, "django", "x.py"), true},
		{iso, filepath.Join(repo, "django", "x.py"), false},
		{iso, filepath.Join(other, "x.py"), false},
		{iso, filepath.Join(tree, "x.py"), false},
		{repo, filepath.Join(iso, "x.py"), false},
		{repo, filepath.Join(tree, "x.py"), true},
		{tree, filepath.Join(repo, "x.py"), true},
	} {
		if got := InProject(c.wd, c.target); got != c.want {
			t.Errorf("InProject(%s, %s) = %v, want %v", filepath.Base(c.wd), c.target, got, c.want)
		}
	}
	real, _ := filepath.EvalSymlinks(iso)
	// git prints the worktree path with forward slashes on Windows; the
	// refusal text is read by the MODEL, so slashes, not native separators,
	// are what it is compared with. Live 2026-11-12: the raw EvalSymlinks
	// form failed the check on windows-latest (#321's Windows Go tests).
	want := filepath.ToSlash(real) + " (an isolated copy of the project)"
	if got := filepath.ToSlash(ProjectFolders(iso)); got != want {
		t.Errorf("ProjectFolders(iso) = %q, want %q", got, want)
	}
}
