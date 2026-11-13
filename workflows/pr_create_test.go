package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func prScript(t *testing.T, dir, name string, env []string, args ...string) (string, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("pr-create", name))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), exitOf(err, t)
}

const prTemplate = "## Description\n\n<!-- What changes -->\n\n## Related issue\n\nFixes #\n\n## Tests\n\n- [ ] `make test`\n- [ ] `make check` (fmt, vet)\n- [ ] `make scan`\n\n```\n<test output>\n```\n\n## Checklist\n\n- [ ] done\n"

func prRun(t *testing.T, files map[string]string) (dir, run string) {
	t.Helper()
	dir = t.TempDir()
	run = filepath.Join(dir, "run")
	all := map[string]string{filepath.Join(run, "description.md"): "Add the qa workflow\n\n**What**\nAdds it.\n"}
	for p, c := range files {
		all[filepath.Join(dir, p)] = c
	}
	writeAll(t, all, 0o644)
	return dir, run
}

// checks.sh records each command's exit status and output, a failure included, and runs the next one.
func TestPRCreateChecksRecordsFailuresAndContinues(t *testing.T) {
	dir, run := prRun(t, nil)
	out, code := prScript(t, dir, "checks.sh", nil, run, "echo fine,echo broke; exit 3,echo after")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if tsv := readPR(t, filepath.Join(run, "checks.tsv")); tsv != "echo fine\t0\necho broke; exit 3\t3\necho after\t0\n" {
		t.Fatalf("checks.tsv:\n%q", tsv)
	}
	if log := readPR(t, filepath.Join(run, "checks.txt")); !strings.Contains(log, "$ echo broke; exit 3   (exit 3)") || !strings.Contains(log, "broke") {
		t.Fatalf("checks.txt:\n%s", log)
	}
}

// With a template: the description in its section, passed checks ticked, a failed one marked, the output in.
func TestPRCreateBodyFillsTheTemplateFromTheRecordedRun(t *testing.T) {
	dir, run := prRun(t, map[string]string{".github/PULL_REQUEST_TEMPLATE.md": prTemplate})
	writeAll(t, map[string]string{
		filepath.Join(run, "checks.tsv"): "make test\t0\nmake check\t2\n",
		filepath.Join(run, "checks.txt"): "$ make test   (exit 0)\nok\n",
	}, 0o644)
	if out, code := prScript(t, dir, "body.sh", nil, run, "326"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	body := readPR(t, filepath.Join(run, "body.md"))
	for _, want := range []string{"Adds it.", "Fixes #326", "- [x] `make test`", "- [ ] `make check` (fmt, vet) (exit 2, see the output)", "- [ ] `make scan`\n", "$ make test   (exit 0)", "- [ ] done"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "What changes") || strings.Contains(body, "<test output>") {
		t.Errorf("template placeholders left:\n%s", body)
	}
	if got := strings.TrimSpace(readPR(t, filepath.Join(run, "title.txt"))); got != "Add the qa workflow" {
		t.Errorf("title %q", got)
	}
}

// No template: the description and what ran. A check that was not run is not ticked or claimed.
func TestPRCreateBodyWithoutATemplate(t *testing.T) {
	dir, run := prRun(t, nil)
	writeAll(t, map[string]string{filepath.Join(run, "checks.tsv"): "", filepath.Join(run, "checks.txt"): "no checks run (set the checks arg)\n"}, 0o644)
	prScript(t, dir, "body.sh", nil, run, "")
	body := readPR(t, filepath.Join(run, "body.md"))
	if !strings.Contains(body, "Adds it.") || !strings.Contains(body, "no checks run") || strings.Contains(body, "[x]") {
		t.Fatalf("body:\n%s", body)
	}
}

func prRepo(t *testing.T) (dir, run, remote string) {
	t.Helper()
	dir, run = prRun(t, nil)
	remote = t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare")
	gitIn(t, dir, "init", "-q", "-b", "main")
	writeAll(t, map[string]string{filepath.Join(dir, "a.txt"): "a\n", filepath.Join(run, "title.txt"): "A title\n", filepath.Join(run, "body.md"): "A body\n"}, 0o644)
	gitIn(t, dir, "add", "a.txt")
	gitIn(t, dir, "commit", "-qm", "base")
	gitIn(t, dir, "remote", "add", "origin", remote)
	return dir, run, remote
}

// open.sh refuses the default branch, and in draft mode pushes nothing and calls no gh.
func TestPRCreateOpenRefusesTheDefaultBranchAndDraftsByDefault(t *testing.T) {
	dir, run, remote := prRepo(t)
	if out, code := prScript(t, dir, "open.sh", nil, run, "now"); code == 0 || !strings.Contains(out, "needs its own branch") {
		t.Fatalf("on main: exit %d:\n%s", code, out)
	}
	gitIn(t, dir, "checkout", "-q", "-b", "topic")
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls.txt")
	writeAll(t, map[string]string{filepath.Join(bin, "gh"): "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n"}, 0o755)
	env := []string{"PATH=" + bin + ":" + os.Getenv("PATH")}
	out, code := prScript(t, dir, "open.sh", env, run, "draft")
	if code != 0 || !strings.Contains(out, "draft: would push topic") {
		t.Fatalf("draft: exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Fatal("draft called gh")
	}
	if refs := gitIn(t, remote, "branch", "--list"); refs != "" {
		t.Fatalf("draft pushed: %q", refs)
	}
}

// With open: now it pushes the branch and creates the pull request against main with the title and body.
func TestPRCreateOpenNowPushesAndCreates(t *testing.T) {
	dir, run, remote := prRepo(t)
	gitIn(t, dir, "checkout", "-q", "-b", "topic")
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls.txt")
	writeAll(t, map[string]string{filepath.Join(bin, "gh"): "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n[ \"$1 $2\" = \"pr create\" ] && echo https://github.com/o/r/pull/9\nexit 0\n"}, 0o755)
	env := []string{"PATH=" + bin + ":" + os.Getenv("PATH")}
	out, code := prScript(t, dir, "open.sh", env, run, "now")
	if code != 0 || !strings.Contains(out, "/pull/9") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if refs := gitIn(t, remote, "branch", "--list"); !strings.Contains(refs, "topic") {
		t.Fatalf("branch not pushed: %q", refs)
	}
	if c := readPR(t, calls); !strings.Contains(c, "pr create --base main --head topic --title A title --body-file") {
		t.Fatalf("gh calls:\n%s", c)
	}
}

// Args belong to the step that declares them; an undeclared {{.name}} renders empty.
func TestPRCreateEveryTemplateArgIsDeclaredByItsStep(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("pr-create", "steps", "*.yaml"))
	if len(files) == 0 {
		t.Fatal("no steps")
	}
	ref := regexp.MustCompile(`\{\{\s*\.(\w+)`)
	for _, f := range files {
		var step struct {
			Args map[string]any `yaml:"args"`
		}
		text := readPR(t, f)
		if err := yaml.Unmarshal([]byte(text), &step); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range ref.FindAllStringSubmatch(text, -1) {
			if _, ok := step.Args[m[1]]; !ok && m[1] != "partition" {
				t.Errorf("%s uses {{.%s}} and declares no such arg", f, m[1])
			}
		}
	}
}

func readPR(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A failing check's log names the failure before the tail: the line that says which test failed.
func TestPRCreateChecksNamesTheFailureNotOnlyTheTail(t *testing.T) {
	dir, run := prRun(t, nil)
	cmd := "(echo '--- FAIL: TestThing (0.00s)'; for i in $(seq 1 30); do echo ok line $i; done; echo FAIL; exit 1)"
	if out, code := prScript(t, dir, "checks.sh", nil, run, cmd); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	log := readPR(t, filepath.Join(run, "checks.txt"))
	if !strings.Contains(log, "--- FAIL: TestThing") {
		t.Fatalf("the failing test is not named:\n%s", log)
	}
}

func prGit(t *testing.T) (dir, run string) {
	t.Helper()
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir, run = prRun(t, nil)
	gitIn(t, dir, "init", "-q", "-b", "main")
	writeAll(t, map[string]string{filepath.Join(dir, "a.txt"): "one\n", filepath.Join(dir, ".gitignore"): "run/\n"}, 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "base")
	gitIn(t, dir, "remote", "add", "origin", remote)
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "remote", "set-head", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "-b", "topic")
	return dir, run
}

func commitOn(t *testing.T, dir, branch, file, text string) {
	t.Helper()
	gitIn(t, dir, "checkout", "-q", branch)
	writeAll(t, map[string]string{filepath.Join(dir, file): text}, 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "change "+file)
}

// sync.sh: a branch that merges is clean and may be behind; one that edits the same line the base did
// conflicts, names the file and exits 1; open.sh then refuses.
func TestPRCreateSyncCleanBehindAndConflict(t *testing.T) {
	dir, run := prGit(t)
	commitOn(t, dir, "topic", "b.txt", "topic\n")
	gitIn(t, dir, "checkout", "-q", "main")
	commitOn(t, dir, "main", "c.txt", "base moved\n")
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "topic")
	out, code := prScript(t, dir, "sync.sh", nil, run)
	if code != 0 || !strings.Contains(out, "merges cleanly") || !strings.Contains(out, "1 ahead, 1 behind") {
		t.Fatalf("clean: exit %d:\n%s", code, out)
	}
	commitOn(t, dir, "topic", "a.txt", "topic edit\n")
	gitIn(t, dir, "checkout", "-q", "main")
	commitOn(t, dir, "main", "a.txt", "base edit\n")
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "topic")
	out, code = prScript(t, dir, "sync.sh", nil, run)
	if code != 1 || !strings.Contains(out, "conflicts with origin/main in:") || !strings.Contains(out, "a.txt") {
		t.Fatalf("conflict: exit %d:\n%s", code, out)
	}
	if got := strings.TrimSpace(readPR(t, filepath.Join(run, "sync.status"))); got != "conflict" {
		t.Fatalf("sync.status %q", got)
	}
	writeAll(t, map[string]string{filepath.Join(run, "title.txt"): "t\n", filepath.Join(run, "body.md"): "b\n"}, 0o644)
	if out, code := prScript(t, dir, "open.sh", nil, run, "now"); code == 0 || !strings.Contains(out, "conflicts with origin/main") {
		t.Fatalf("open on a conflict: exit %d:\n%s", code, out)
	}
}

func ciGH(t *testing.T, checks, view string) []string {
	t.Helper()
	bin := t.TempDir()
	counter := filepath.Join(bin, "n")
	script := "#!/bin/sh\nif [ \"$1 $2\" = \"pr checks\" ]; then\n  n=$(cat '" + counter + "' 2>/dev/null || echo 0); echo $((n+1)) > '" + counter + "'\n" + checks + "\nfi\nif [ \"$1 $2\" = \"pr view\" ]; then echo '" + view + "'; fi\n"
	writeAll(t, map[string]string{filepath.Join(bin, "gh"): script}, 0o755)
	return []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "PR_CREATE_POLL=0"}
}

func ciRun(t *testing.T, env []string) (string, int, string) {
	t.Helper()
	dir, run := prRun(t, map[string]string{})
	writeAll(t, map[string]string{filepath.Join(run, "url.txt"): "https://github.com/o/r/pull/9\n"}, 0o644)
	out, code := prScript(t, dir, "ci.sh", env, run, "1")
	return out, code, run
}

// ci.sh waits out a pending check, then reports; a failed check or a conflicting pull request is exit 1.
func TestPRCreateCIWaitsForPendingAndReportsFailures(t *testing.T) {
	pendingThenPass := `if [ "$n" -lt 2 ]; then printf 'pending\tbuild\n'; else printf 'pass\tbuild\npass\tlint\n'; fi`
	out, code, run := ciRun(t, ciGH(t, pendingThenPass, "MERGEABLE (CLEAN)"))
	if code != 0 || !strings.Contains(out, "pass\tbuild") || !strings.Contains(out, "mergeable: MERGEABLE (CLEAN)") {
		t.Fatalf("pass: exit %d:\n%s", code, out)
	}
	if !strings.Contains(readPR(t, filepath.Join(run, "ci.txt")), "pass\tlint") {
		t.Fatal("ci.txt lacks the checks")
	}
	out, code, _ = ciRun(t, ciGH(t, `printf 'fail\ttest\npass\tlint\n'`, "MERGEABLE (UNSTABLE)"))
	if code != 1 || !strings.Contains(out, "1 check(s) failed") {
		t.Fatalf("fail: exit %d:\n%s", code, out)
	}
	out, code, _ = ciRun(t, ciGH(t, `printf 'pass\ttest\n'`, "CONFLICTING (DIRTY)"))
	if code != 1 || !strings.Contains(out, "conflicting=1") {
		t.Fatalf("conflicting: exit %d:\n%s", code, out)
	}
}

// No CI at all, and no pull request opened, are not failures.
func TestPRCreateCINoChecksAndNoPullRequest(t *testing.T) {
	out, code, _ := ciRun(t, ciGH(t, ``, "MERGEABLE (CLEAN)"))
	if code != 0 || !strings.Contains(out, "no checks") {
		t.Fatalf("no checks: exit %d:\n%s", code, out)
	}
	dir, run := prRun(t, nil)
	if out, code := prScript(t, dir, "ci.sh", nil, run, "1"); code != 0 || !strings.Contains(out, "nothing to check") {
		t.Fatalf("no url: exit %d:\n%s", code, out)
	}
}
