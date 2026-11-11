package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
