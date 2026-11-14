package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/guregodevo/mario/templates"
)

// ruleAnswer renders a rule step's text in dir as mario does: the string functions, file and exists.
func ruleAnswer(t *testing.T, text, dir string) string {
	t.Helper()
	funcs := templates.StringFuncs()
	funcs["file"] = func(path string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, path))
		return string(b), err
	}
	funcs["exists"] = func(path string) bool {
		fi, err := os.Stat(filepath.Join(dir, path))
		return err == nil && !fi.IsDir()
	}
	tpl, err := template.New("rule").Funcs(funcs).Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := tpl.Execute(&out, map[string]string{"partition": "p"}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// shellRule is the rule as the step ran it before it was a template: team-solve's verdict.sh, then the
// status file when there is no verdict.
func shellRule(t *testing.T, dir, answer, status string) string {
	t.Helper()
	script, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve", "verdict.sh"))
	cmd := exec.Command("bash", "-c", `r=.memdoor/runs/team-solve-fanout-p
v=$(bash "$0" "$r/`+answer+`")
if [ "$v" = PASS ] || { [ "$v" = none ] && [ "$(cat "$r/`+status+`" 2>/dev/null)" = 0 ]; }; then echo PASS; else echo FAIL; fi`, script)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// The review's and the judge's rules are templates now. On every answer below, and on every review.md and
// verdict.md under MEMDOOR_VERDICT_REPLAY when it is set, they say what the shell rule said.
func TestFanoutVerdictRulesSayWhatTheShellRuleSaid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the rule it is compared with is a bash script")
	}
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve-fanout"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in team-solve-fanout beside this package")
	}
	wf := load(t, src, &fakeRunner{})
	answers := map[string]string{
		"a verdict":                 "verdict: PASS\nbecause\n",
		"a failing verdict":         "verdict: FAIL\nthe loop bound\n",
		"lower case":                "verdict: pass\n",
		"bold":                      "**Verdict:** PASS\n",
		"bold, the colon outside":   "**verdict**: FAIL\n",
		"code":                      "`verdict: FAIL`\n",
		"indented":                  "   verdict:   PASS\n",
		"a tab":                     "\tverdict:\tFAIL\n",
		"after a blank line":        "\n\nverdict: PASS\n",
		"on the tenth line":         strings.Repeat("x\n", 9) + "verdict: FAIL\n",
		"on the eleventh line":      strings.Repeat("x\n", 10) + "verdict: FAIL\n",
		"two verdicts":              "verdict: FAIL\nverdict: PASS\n",
		"in a sentence":             "My verdict: PASS, I think\n",
		"narrated":                  "Let me inspect the diff first.\n",
		"passed, not pass":          "verdict: PASSED\n",
		"the verdict on a new line": "verdict:\nPASS\n",
		"a heading":                 "## Verdict: PASS\n",
		"a list item":               "- verdict: PASS\n",
		"no final newline":          "verdict: FAIL",
		"windows line ends":         "verdict: PASS\r\n",
		"empty":                     "",
		"no file":                   "\x00none",
	}
	if replay := os.Getenv("MEMDOOR_VERDICT_REPLAY"); replay != "" {
		n := 0
		_ = filepath.WalkDir(replay, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (d.Name() == "review.md" || d.Name() == "verdict.md") {
				if b, err := os.ReadFile(path); err == nil {
					answers[path] = string(b)
					n++
				}
			}
			return nil
		})
		t.Logf("replayed %d saved answers under %s", n, replay)
	}
	for _, step := range []struct{ name, answer, status string }{
		{"team-solve-fanout.steps.review-rule", "review.md", "test-status.txt"},
		{"team-solve-fanout.steps.judge-rule", "verdict.md", "final-status.txt"},
	} {
		d := wf.Defs[step.name]
		if d == nil || d.Type != "template" {
			t.Fatalf("%s is not a template step: %+v", step.name, d)
		}
		for name, answer := range answers {
			for _, status := range []string{"0\n", "1\n", "none\n", "\x00none"} {
				dir := t.TempDir()
				run := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-p")
				if err := os.MkdirAll(run, 0o755); err != nil {
					t.Fatal(err)
				}
				if answer != "\x00none" {
					os.WriteFile(filepath.Join(run, step.answer), []byte(answer), 0o644)
				}
				if status != "\x00none" {
					os.WriteFile(filepath.Join(run, step.status), []byte(status), 0o644)
				}
				if got, want := ruleAnswer(t, d.Text, dir), shellRule(t, dir, step.answer, step.status); got != want {
					t.Errorf("%s, %s, status %q: the template says %q, the shell rule said %q", step.name, name, status, got, want)
				}
			}
		}
	}
}
