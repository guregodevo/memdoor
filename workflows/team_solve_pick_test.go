package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// pick.yaml's command on a throwaway git tree with three saved attempts and
// their checks. Attempt a changes a.txt to "alpha"; b to "fix b"; c to "fix"
// (the smallest). check-a passes only on a; check-b and check-c pass on any
// tree whose a.txt starts with fix. A check that passes on the
// tree as found proves nothing and is ignored.
func runPick(t *testing.T, checks map[string]string) (pick, file string) {
	t.Helper()
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	var step struct {
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join(wf, "steps", "pick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
		}
	}
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	if err := os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wf, filepath.Join(dir, ".memdoor", "workflows", "team-solve")); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string, mode os.FileMode) {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		if err := os.WriteFile(name, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
	run("git", "init", "-q")
	write("a.txt", "start\n", 0o644)
	run("git", "add", "a.txt")
	run("git", "commit", "-q", "-m", "base")
	head, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	write(filepath.Join(r, "base.txt"), strings.TrimSpace(string(head))+"\n", 0o644)
	write(filepath.Join(r, "untracked-before.txt"), "", 0o644)
	// The fan-out saves each attempt as attempts/<n>.patch (direct.yaml); the
	// letters of the cases below are attempts 1, 2 and 3.
	num := map[string]string{"a": "1", "b": "2", "c": "3"}
	if err := os.MkdirAll(filepath.Join(r, "attempts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"a": "alpha\n", "b": "fix b\n", "c": "fix\n"} {
		write("a.txt", content, 0o644)
		run("bash", filepath.Join(wf, "snapshot.sh"), "save", r, "attempts/"+num[name])
		run("bash", filepath.Join(wf, "snapshot.sh"), "reset", r)
	}
	for name, script := range checks {
		write(filepath.Join(r, "attempt-"+num[name]), "#!/bin/sh\n"+script+"\n", 0o755)
	}
	cross, err := os.ReadFile(filepath.Join(wf, "cross.sh"))
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(r, "cross.sh"), string(cross), 0o755)
	cmd := exec.Command("sh", "-c", strings.ReplaceAll(step.Command, "{{.partition}}", "1"))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pick: %v\n%s", err, out)
	}
	p, err := os.ReadFile(filepath.Join(r, "pick.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(p)), strings.TrimSpace(string(f))
}

func TestTeamSolvePickTakesTheAttemptTheOthersChecksPass(t *testing.T) {
	for _, c := range []struct {
		name   string
		checks map[string]string
		pick   string // the attempt named in pick.txt
		file   string // a.txt after the pick
	}{
		// b and c each pass the other's check (1 point); a passes none; the tie goes to c, the smaller change.
		{"the checks tell them apart", map[string]string{"a": "grep -q alpha a.txt", "b": "grep -q '^fix' a.txt", "c": "grep -q '^fix' a.txt"}, "attempt 3", "fix"},
		// Only a's check discriminates and only a passes it is not a point for a (own check); b passes a's? no: b scores on check a only if "fix-a" is in b: it is not. Everyone 0: the smallest change, c.
		{"one check nobody else passes", map[string]string{"a": "grep -q alpha a.txt"}, "attempt 3", "fix"},
		// A check that passes on the tree as found proves nothing; with no other check, the smallest change.
		{"a check that passes on the tree as found", map[string]string{"b": "test -f a.txt"}, "attempt 3", "fix"},
		// b's check wants the word b: only b has it, and b never scores on its own check. c's check wants "fix b": only b passes it. b wins with 1.
		{"one attempt passes the other's check", map[string]string{"b": "grep -qw b a.txt", "c": "grep -q 'fix b' a.txt"}, "attempt 2", "fix b"},
		{"no check at all", map[string]string{}, "attempt 3", "fix"},
	} {
		pick, file := runPick(t, c.checks)
		if !strings.Contains(pick, "picked: "+c.pick) || file != c.file {
			t.Errorf("%s: pick.txt %q, a.txt %q; want %q and %q", c.name, pick, file, c.pick, c.file)
		}
	}
}

// cross.sh's exit status is the fan-out's `between`: 1 while the top of the
// matrix is a tie on checks that discriminate (another attempt can break it),
// 0 once one attempt leads or no check tells any apart.
func TestCrossSettlesOnlyWhenThePickIsNotATie(t *testing.T) {
	wf, _ := filepath.Abs("team-solve")
	for _, c := range []struct {
		name    string
		checks  map[string]string
		settled bool
	}{
		{"each passes the other's check: a tie", map[string]string{"1": "grep -q fix a.txt", "2": "grep -q fix a.txt"}, false},
		{"one leads", map[string]string{"1": "grep -q 'fix two' a.txt", "2": "grep -q nothing a.txt"}, true},
		{"no check discriminates", map[string]string{"1": "test -f a.txt"}, true},
	} {
		dir := t.TempDir()
		sh := func(line string) error {
			cmd := exec.Command("sh", "-c", line)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			out, err := cmd.CombinedOutput()
			if err != nil && !strings.HasPrefix(line, "bash r/cross.sh") {
				t.Fatalf("%s: %v\n%s", line, err, out)
			}
			return err
		}
		r := filepath.Join(dir, "r")
		os.MkdirAll(filepath.Join(r, "attempts"), 0o755)
		sh("git init -q && echo start > a.txt && git add a.txt && git commit -q -m base && git rev-parse HEAD > r/base.txt && : > r/untracked-before.txt")
		for n, content := range map[string]string{"1": "fix one", "2": "fix two"} {
			sh("echo '" + content + "' > a.txt && git diff > r/attempts/" + n + ".patch && git checkout -q a.txt")
		}
		for n, check := range c.checks {
			os.WriteFile(filepath.Join(r, "attempt-"+n), []byte("#!/bin/sh\n"+check+"\n"), 0o755)
		}
		b, _ := os.ReadFile(filepath.Join(wf, "cross.sh"))
		os.WriteFile(filepath.Join(r, "cross.sh"), b, 0o755)
		if err := sh("bash r/cross.sh r"); (err == nil) != c.settled {
			t.Errorf("%s: settled=%v, want %v", c.name, err == nil, c.settled)
		}
		if got, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output(); strings.TrimSpace(string(got)) != "?? r/" {
			t.Errorf("%s: cross.sh touched the project's tree: %q", c.name, got)
		}
	}
}
