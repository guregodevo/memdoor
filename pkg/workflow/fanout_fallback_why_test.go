package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// shellFallbackWhen is the condition the three fallback steps each carried before fallback-why.
const shellFallbackWhen = `grep -qx FAIL .memdoor/runs/team-solve-fanout-p/review-rule.txt || { grep -q '^kept:' .memdoor/runs/team-solve-fanout-p/keep.txt && [ "$(cat .memdoor/runs/team-solve-fanout-p/final-status.txt 2>/dev/null)" != 0 ] && grep -qx 'check 1' .memdoor/runs/team-solve-fanout-p/keep.txt; }`

// fallback-why says once what the three fallback steps each tested in shell. On every state of the run
// folder below it is non-empty exactly when the shell condition held, and it names the review first.
func TestFanoutFallbackWhySaysWhatTheShellConditionSaid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the condition it is compared with is a bash line")
	}
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve-fanout"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in team-solve-fanout beside this package")
	}
	wf := load(t, src, &fakeRunner{})
	why := wf.Defs["team-solve-fanout.steps.fallback-why"]
	if why == nil || why.Type != "template" {
		t.Fatalf("fallback-why is not a template step: %+v", why)
	}
	for _, step := range []string{"fallback-reset", "fallback", "fallback-done"} {
		if got := strings.TrimSpace(wf.Defs["team-solve-fanout.steps."+step].When); got != `test -n "{{ output "fallback-why" | trim }}"` {
			t.Errorf("%s: when %q does not read fallback-why", step, got)
		}
	}
	const none = "\x00none"
	rules := []string{"FAIL\n", "PASS\n", "FAIL", "FAILED\n", "the review said FAIL\n", "", none}
	keeps := []string{
		"kept: the direct attempt, untouched\n",
		"kept: the team's work (check 1, judge none; the direct attempt's check was 1)\n",
		"kept: the team's work (check 1, judge none; the direct attempt's check was 1)\ncheck 1\n",
		"check 1\nkept: the repair (check 1 with the reproduction, judge PASS)\n",
		"put back: the direct attempt (the team's work did not prove better: check 1, judge FAIL)\ncheck 1\n",
		"check 1\n",
		"", none,
	}
	finals := []string{"0\n", "1\n", "none\n", "0", "", none}
	held := 0
	for _, rule := range rules {
		for _, keep := range keeps {
			for _, final := range finals {
				dir := t.TempDir()
				run := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-p")
				if err := os.MkdirAll(run, 0o755); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"review-rule.txt": rule, "keep.txt": keep, "final-status.txt": final} {
					if content != none {
						os.WriteFile(filepath.Join(run, name), []byte(content), 0o644)
					}
				}
				cmd := exec.Command("bash", "-c", shellFallbackWhen)
				cmd.Dir = dir
				want := cmd.Run() == nil
				got := strings.TrimSpace(ruleAnswer(t, why.Text, dir))
				if (got != "") != want {
					t.Errorf("rule %q, keep %q, final %q: fallback-why says %q, the shell condition held = %v", rule, keep, final, got, want)
				}
				if want {
					held++
					reviewFailed := strings.TrimSpace(rule) == "FAIL"
					if (got == "review") != reviewFailed || (got != "review" && got != "check") {
						t.Errorf("rule %q, keep %q, final %q: reason %q", rule, keep, final, got)
					}
				}
			}
		}
	}
	if held == 0 || held == len(rules)*len(keeps)*len(finals) {
		t.Fatalf("the condition held on %d of the states: the table proves nothing", held)
	}
	t.Logf("%d states, the condition held on %d", len(rules)*len(keeps)*len(finals), held)
}
