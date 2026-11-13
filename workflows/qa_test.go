package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// qaRun is a run directory with the explorer's output: one bugs/<slug>/ a bug.
type qaBug struct{ slug, repro string }

func qaProject(t *testing.T, bugs []qaBug) (project, run string) {
	t.Helper()
	project = t.TempDir()
	gitIn(t, project, "init", "-q")
	writeAll(t, map[string]string{filepath.Join(project, "tool.txt"): "v1\n", filepath.Join(project, "README.md"): "# Tool\nThe flag is honoured.\n"}, 0o644)
	gitIn(t, project, "add", "-A")
	gitIn(t, project, "commit", "-qm", "base")
	run = filepath.Join(project, ".memdoor", "runs", "qa-t")
	var slugs []string
	for _, b := range bugs {
		d := filepath.Join(run, "bugs", b.slug)
		writeAll(t, map[string]string{
			filepath.Join(d, "repro.sh"):    b.repro,
			filepath.Join(d, "title.txt"):   "Title of " + b.slug + "\n",
			filepath.Join(d, "expected.md"): "Expected a in " + b.slug + ".\n",
			filepath.Join(d, "source.txt"):  "README.md\nThe flag is honoured.\n",
		}, 0o644)
		slugs = append(slugs, b.slug)
	}
	writeAll(t, map[string]string{filepath.Join(run, "candidates.txt"): strings.Join(slugs, "\n") + "\n"}, 0o644)
	return project, run
}

func qaScript(t *testing.T, project, name string, args ...string) (string, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("qa", name))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	return string(out), exitOf(err, t)
}

func readQA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// confirm.sh keeps a bug only when its script exits 1 with a BUG: line twice and leaves the
// project's files as they were; the others are named with the reason.
func TestQAConfirmKeepsOnlyWhatReproducesTwice(t *testing.T) {
	project, run := qaProject(t, []qaBug{
		{"real", "echo 'BUG: --flag is ignored'\nexit 1\n"},
		{"fixed", "exit 0\n"},
		{"broken", "echo 'BUG: x'\nexit 2\n"},
		{"silent", "exit 1\n"},
		{"flaky", "f=\"$(dirname \"$0\")/seen\"\nif [ -f \"$f\" ]; then exit 0; fi\n: > \"$f\"\necho 'BUG: first time only'\nexit 1\n"},
		{"dirty", "echo changed > tool.txt\necho 'BUG: x'\nexit 1\n"},
		{"Bad_Slug", "echo 'BUG: x'\nexit 1\n"},
		{"unsourced", "echo 'BUG: x'\nexit 1\n"},
	})
	writeAll(t, map[string]string{filepath.Join(run, "bugs", "unsourced", "source.txt"): "README.md\nA sentence the README does not hold.\n"}, 0o644)
	if out, code := qaScript(t, project, "confirm.sh", run); code != 0 {
		t.Fatalf("confirm.sh exit %d:\n%s", code, out)
	}
	if got := strings.TrimSpace(readQA(t, filepath.Join(run, "confirmed.txt"))); got != "real" {
		t.Fatalf("confirmed %q, want only real\n%s", got, readQA(t, filepath.Join(run, "confirm.log")))
	}
	log := readQA(t, filepath.Join(run, "confirm.log"))
	for _, want := range []string{"fixed: dropped, run 1 exited 0", "broken: dropped, run 1 exited 2", "silent: dropped, run 1 printed no BUG: line", "flaky: dropped, run 2 exited 0", "dirty: dropped, repro.sh changed the project's files", "Bad_Slug: dropped, not a slug", "unsourced: dropped, the quote is not in README.md"} {
		if !strings.Contains(log, want) {
			t.Errorf("confirm.log lacks %q:\n%s", want, log)
		}
	}
	if md := readQA(t, filepath.Join(run, "confirmed.md")); !strings.Contains(md, "## real") || !strings.Contains(md, "Title of real") || !strings.Contains(md, "Got: BUG: --flag is ignored") {
		t.Errorf("confirmed.md:\n%s", md)
	}
}

// A slug with a path in it is a slug no script runs for.
func TestQAConfirmRefusesAPathAsASlug(t *testing.T) {
	project, run := qaProject(t, nil)
	writeAll(t, map[string]string{filepath.Join(run, "candidates.txt"): "../../etc\n"}, 0o644)
	qaScript(t, project, "confirm.sh", run)
	if got := strings.TrimSpace(readQA(t, filepath.Join(run, "confirmed.txt"))); got != "" {
		t.Fatalf("confirmed %q", got)
	}
}

func settled(t *testing.T, slugs []string, dups, open string, max string) (project, run, out string) {
	t.Helper()
	var bugs []qaBug
	for _, s := range slugs {
		bugs = append(bugs, qaBug{s, "echo 'BUG: x'\nexit 1\n"})
	}
	project, run = qaProject(t, bugs)
	qaScript(t, project, "confirm.sh", run)
	writeAll(t, map[string]string{filepath.Join(run, "dups.txt"): dups, filepath.Join(run, "open.txt"): open}, 0o644)
	out, code := qaScript(t, project, "settle.sh", run, max)
	if code != 0 {
		t.Fatalf("settle.sh exit %d:\n%s", code, out)
	}
	return project, run, out
}

// settle.sh drops a bug an open issue describes, ignores a number that is not an open issue, and
// keeps the first max of the rest.
func TestQASettleDropsDuplicatesAndAppliesTheCap(t *testing.T) {
	_, run, out := settled(t, []string{"a", "b", "c", "d"}, "a #12\nb #99\nnone\n", "#12 The flag is ignored\n#5 Other\n", "2")
	if got := strings.Fields(readQA(t, filepath.Join(run, "final.txt"))); strings.Join(got, ",") != "b,c" {
		t.Fatalf("final %v, want b,c (a is #12, b's #99 is invented, d is over the cap)", got)
	}
	for _, want := range []string{"4 confirmed, 1 already an open issue, 2 to file", "a: the open issue #12", "d: over the cap of 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	body := readQA(t, filepath.Join(run, "bugs", "b", "issue.md"))
	for _, want := range []string{"**Context**", "echo 'BUG: x'", "**Expected results**", "Expected a in b.", "> The flag is honoured.", "**Actual results**", "BUG: x", "Found by the `qa` workflow"} {
		if !strings.Contains(body, want) {
			t.Errorf("issue.md lacks %q:\n%s", want, body)
		}
	}
}

// With no dups.txt at all (dedupe skipped: nothing confirmed, or no model) every confirmed bug stands.
func TestQASettleWithoutADedupeAnswerKeepsThem(t *testing.T) {
	project, run, _ := settled(t, []string{"a"}, "", "(no open issues known)\n", "3")
	if err := os.Remove(filepath.Join(run, "dups.txt")); err != nil {
		t.Fatal(err)
	}
	qaScript(t, project, "settle.sh", run, "3")
	if got := strings.TrimSpace(readQA(t, filepath.Join(run, "final.txt"))); got != "a" {
		t.Fatalf("final %q, want a", got)
	}
}

// fakeGH records every call in calls.txt; script decides the answers by its arguments.
func fakeGH(t *testing.T, script string) (bin, calls string) {
	t.Helper()
	bin = t.TempDir()
	calls = filepath.Join(bin, "calls.txt")
	body := "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n" + script
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

func runFile(t *testing.T, project, run, bin, max string) (string, int) {
	t.Helper()
	script, _ := filepath.Abs(filepath.Join("qa", "file.sh"))
	cmd := exec.Command("bash", script, run, "bug,missing", "qa", max)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), exitOf(err, t)
}

const ghOpen = `case "$1 $2" in
  "auth status") exit 0;;
  "issue list") echo %s;;
  "label list") printf 'bug\nqa\nother\n';;
  "issue create") echo https://github.com/o/r/issues/77;;
esac
`

// file.sh files each settled bug with the labels the repository has, and no label it does not.
func TestQAFileCreatesIssuesWithTheLabelsThatExist(t *testing.T) {
	project, run, _ := settled(t, []string{"a", "b"}, "none\n", "(no open issues known)\n", "3")
	bin, calls := fakeGH(t, strings.Replace(ghOpen, "%s", "0", 1))
	if out, code := runFile(t, project, run, bin, "5"); code != 0 {
		t.Fatalf("file.sh exit %d:\n%s", code, out)
	}
	log := readQA(t, calls)
	if n := strings.Count(log, "issue create"); n != 2 {
		t.Fatalf("%d issues created, want 2:\n%s", n, log)
	}
	if !strings.Contains(log, "--label bug --label qa") || strings.Contains(log, "missing") {
		t.Fatalf("labels wrong:\n%s", log)
	}
	if got := strings.Count(readQA(t, filepath.Join(run, "filed.txt")), "/issues/77"); got != 2 {
		t.Fatalf("filed.txt has %d urls, want 2", got)
	}
}

// At the cap of its own open issues nothing is filed, and the run still ends well.
func TestQAFileStopsAtTheCap(t *testing.T) {
	project, run, _ := settled(t, []string{"a"}, "none\n", "(no open issues known)\n", "3")
	bin, calls := fakeGH(t, strings.Replace(ghOpen, "%s", "5", 1))
	out, code := runFile(t, project, run, bin, "5")
	if code != 0 || !strings.Contains(out, "nothing filed") || strings.Contains(readQA(t, calls), "issue create") {
		t.Fatalf("exit %d:\n%s\n%s", code, out, readQA(t, calls))
	}
}

// The qa workflow's files all parse, and a step that is skipped by `when` is one the DAG names.
func TestQAStepsParseAndRequireEachOther(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("qa", "steps", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no steps: %v", err)
	}
	have := map[string]bool{}
	for _, f := range files {
		have[strings.TrimSuffix(filepath.Base(f), ".yaml")] = true
	}
	for _, f := range files {
		var step struct {
			Type     string `yaml:"type"`
			Requires []struct {
				Table string `yaml:"table_pattern"`
			} `yaml:"requires"`
		}
		if err := yaml.Unmarshal([]byte(readQA(t, f)), &step); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if step.Type == "" {
			t.Errorf("%s has no type", f)
		}
		for _, r := range step.Requires {
			if !have[r.Table] {
				t.Errorf("%s requires %q, which is no step", f, r.Table)
			}
		}
	}
}

// Args belong to the step that declares them: a {{.name}} in a step's text that the step does not
// declare renders empty (the first real run printed "<no value>" and `file: now` would never have
// filed). The partition is the engine's.
func TestQAEveryTemplateArgIsDeclaredByItsStep(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("qa", "steps", "*.yaml"))
	ref := regexp.MustCompile(`\{\{\s*\.(\w+)`)
	for _, f := range files {
		var step struct {
			Args map[string]any `yaml:"args"`
		}
		text := readQA(t, f)
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

// The body has four fixed headings. Every one is data, except the title, the expected sentence and a
// proposal, which appears only when the explorer wrote one.
func TestQAIssueIsTheFixedTemplateFilledFromTheRun(t *testing.T) {
	project, run := qaProject(t, []qaBug{{"a", "echo 'saw 0 lines'\necho 'BUG: wrong count'\nexit 1\n"}, {"b", "echo 'BUG: x'\nexit 1\n"}})
	writeAll(t, map[string]string{filepath.Join(run, "bugs", "a", "proposal.md"): "Count with wc -l in count.sh.\n"}, 0o644)
	qaScript(t, project, "confirm.sh", run)
	writeAll(t, map[string]string{filepath.Join(run, "dups.txt"): "none\n", filepath.Join(run, "open.txt"): "(no open issues known)\n"}, 0o644)
	qaScript(t, project, "settle.sh", run, "3")
	a := readQA(t, filepath.Join(run, "bugs", "a", "issue.md"))
	for _, want := range []string{"**Context**", "echo 'saw 0 lines'", "**Expected results**", "> The flag is honoured.", "**Actual results**", "saw 0 lines", "Exit 1 both times", "**Proposal**", "Count with wc -l"} {
		if !strings.Contains(a, want) {
			t.Errorf("issue a lacks %q:\n%s", want, a)
		}
	}
	if b := readQA(t, filepath.Join(run, "bugs", "b", "issue.md")); strings.Contains(b, "Proposal") {
		t.Errorf("issue b has a Proposal nobody wrote:\n%s", b)
	}
}

func pickArea(t *testing.T, project, run, focus string) string {
	t.Helper()
	out, code := qaScript(t, project, "pick-area.sh", run, focus)
	if code != 0 {
		t.Fatalf("pick-area.sh exit %d:\n%s", code, out)
	}
	return strings.TrimSpace(readQA(t, filepath.Join(run, "focus.txt")))
}

// One area a run: the next the tried list does not hold, round and round; a focus given wins and uses none up.
func TestQAPickAreaRotatesThroughTheAreas(t *testing.T) {
	project, run := qaProject(t, nil)
	writeAll(t, map[string]string{filepath.Join(project, ".memdoor", "qa", "areas.txt"): "the installer\nthe cli\nthe api\n"}, 0o644)
	var got []string
	for i := 0; i < 4; i++ {
		got = append(got, pickArea(t, project, run, ""))
	}
	if strings.Join(got, ",") != "the installer,the cli,the api,the installer" {
		t.Fatalf("picked %v", got)
	}
	if f := pickArea(t, project, run, "workflows"); f != "workflows" {
		t.Fatalf("focus %q", f)
	}
	if next := pickArea(t, project, run, ""); next != "the cli" {
		t.Fatalf("a focus used up an area: next %q, want the cli", next)
	}
}

// No list yet is not an error: the whole project, an empty focus.
func TestQAPickAreaWithoutAListIsTheWholeProject(t *testing.T) {
	project, run := qaProject(t, nil)
	if f := pickArea(t, project, run, ""); f != "" {
		t.Fatalf("focus %q, want empty", f)
	}
}

// The explorer's deadline is written as epoch seconds, minutes ahead.
func TestQAPickAreaWritesTheDeadline(t *testing.T) {
	project, run := qaProject(t, nil)
	if out, code := qaScript(t, project, "pick-area.sh", run, "", "8"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	got, err := strconv.ParseInt(strings.TrimSpace(readQA(t, filepath.Join(run, "deadline.txt"))), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if d := got - time.Now().Unix(); d < 7*60 || d > 8*60+5 {
		t.Fatalf("deadline is %ds ahead, want about 480", d)
	}
}
