package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pr-description lists the change's commits with one git command and writes them with a template: the
// file the describe steps read holds each commit's subject and body.
func TestPRDescriptionCommitsAreTheChangesCommits(t *testing.T) {
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "pr-description"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in pr-description beside this package")
	}
	// Not a pull request: on CI the diff step would fetch a base this repository has no remote for.
	t.Setenv("GITHUB_BASE_REF", "")
	root := t.TempDir()
	gitAt(t, root, "init", "-q", "-b", "main")
	for _, c := range [][2]string{{"Old thing", "before the change"}, {"Add the flag", "it does a thing"}} {
		if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(c[0]), 0o644); err != nil {
			t.Fatal(err)
		}
		gitAt(t, root, "add", "-A")
		gitAt(t, root, "commit", "-q", "-m", c[0], "-m", c[1])
	}
	r := &fakeRunner{}
	wf := load(t, src, r)
	if d := wf.Defs["pr-description.steps.commits"]; d == nil || d.Type != "template" {
		t.Fatalf("commits is not a template step: %+v", d)
	}
	// The describe step's agent is a fake that writes no description: only the commits file is read here.
	_, _ = Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Retry: 20 * time.Millisecond})
	commits, err := os.ReadFile(filepath.Join(root, ".memdoor", "runs", "pr-description-p.commits"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(commits), "Add the flag\nit does a thing") || strings.Contains(string(commits), "Old thing") {
		t.Errorf("the commits file:\n%s", commits)
	}
}
