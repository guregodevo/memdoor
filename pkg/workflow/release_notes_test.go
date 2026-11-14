package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitAt(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// release-notes builds its log with one-line git commands and a template: the release, its date, the tag
// before it, and the commits since that tag only. The short log for a slow model holds the subjects.
func TestReleaseNotesLogIsTheCommitsSinceTheLastTag(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "workflows", "release-notes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in release-notes beside this package")
	}
	root := t.TempDir()
	gitAt(t, root, "init", "-q", "-b", "main")
	commit := func(file, subject, body string) {
		if err := os.WriteFile(filepath.Join(root, file), []byte(subject), 0o644); err != nil {
			t.Fatal(err)
		}
		gitAt(t, root, "add", "-A")
		gitAt(t, root, "commit", "-q", "-m", subject, "-m", body)
	}
	commit("a.txt", "Old thing", "before the tag")
	gitAt(t, root, "tag", "v1.0.0")
	commit("b.txt", "Add the flag", "it does a thing")
	commit("c.txt", "Fix the crash", "on empty input")

	r := &fakeRunner{}
	wf := load(t, src, r)
	if _, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Retry: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(filepath.Join(root, ".memdoor", "runs", "release-notes-p.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"release: unreleased\n", "since: v1.0.0\n", "Add the flag\nit does a thing", "Fix the crash\non empty input"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
	if strings.Contains(string(log), "Old thing") {
		t.Errorf("the log holds a commit from before the tag:\n%s", log)
	}
	lean, err := os.ReadFile(filepath.Join(root, ".memdoor", "runs", "release-notes-p-lean.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lean), "Fix the crash") || strings.Contains(string(lean), "on empty input") || !strings.Contains(string(lean), "since: v1.0.0") {
		t.Errorf("the short log:\n%s", lean)
	}
}

// With no tag before the release the log starts at the first commit and says so.
func TestReleaseNotesLogWithNoTagBefore(t *testing.T) {
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "release-notes"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in release-notes beside this package")
	}
	root := t.TempDir()
	gitAt(t, root, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644)
	gitAt(t, root, "add", "-A")
	gitAt(t, root, "commit", "-q", "-m", "First")
	r := &fakeRunner{}
	if _, err := Run(context.Background(), load(t, src, r), types(r), Options{Workdir: root, Partition: "p", Retry: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(filepath.Join(root, ".memdoor", "runs", "release-notes-p.log"))
	if !strings.Contains(string(log), "since: the first commit\n") || !strings.Contains(string(log), "First") {
		t.Fatalf("log:\n%s", log)
	}
}
