package cmd

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// Outside a repository on GitHub there is no nightly offer: nothing printed, nothing read.
func TestTheNightlyOfferIsNotShownOutsideARepository(t *testing.T) {
	var out bytes.Buffer
	in := bufio.NewReader(strings.NewReader("y\n"))
	if err := offerNightly(in, &out, ""); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("printed outside a repository:\n%s", out.String())
	}
	if line, _ := in.ReadString('\n'); line != "y\n" {
		t.Fatalf("the answer was consumed with no question asked: %q left", line)
	}
}

// Declined, or just Enter: the cron line for later, in the repository, and no schedule written.
func TestDecliningTheNightlyNamesTheCronLine(t *testing.T) {
	for _, answer := range []string{"n\n", "\n"} {
		var out bytes.Buffer
		if err := offerNightly(bufio.NewReader(strings.NewReader(answer)), &out, "/tmp/some-repo"); err != nil {
			t.Fatal(err)
		}
		got := out.String()
		if !strings.Contains(got, "Work some-repo's open issues at night?") || !strings.Contains(got, `memdoor cron add --workflow team-backlog "0 2 * * *"`) {
			t.Fatalf("answer %q: the question and the line for later:\n%s", answer, got)
		}
	}
}

// A directory that is not a git repository is not a backlog.
func TestGithubRepoAtIsEmptyOutsideGit(t *testing.T) {
	if got := githubRepoAt(t.TempDir()); got != "" {
		t.Fatalf("a fresh temp dir is a repository? %q", got)
	}
	if got := githubRepoAt(""); got != "" {
		t.Fatalf("an empty dir is a repository? %q", got)
	}
}

// A repository whose origin is not on GitHub is not one either.
func TestGithubRepoAtIsEmptyWithoutAGitHubOrigin(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://gitlab.com/x/y.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := githubRepoAt(dir); got != "" {
		t.Fatalf("a GitLab origin is a GitHub repository? %q", got)
	}
}
