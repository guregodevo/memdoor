package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// shellFallbackWhen is the condition the three fallback steps each carried before fallback-why (#348).
const shellFallbackWhen = `grep -qx FAIL .memdoor/runs/team-solve-fanout-p/review-rule.txt || { grep -q '^kept:' .memdoor/runs/team-solve-fanout-p/keep.txt && [ "$(cat .memdoor/runs/team-solve-fanout-p/final-status.txt 2>/dev/null)" != 0 ] && grep -qx 'check 1' .memdoor/runs/team-solve-fanout-p/keep.txt; }`

// fallbackReason is the rule fallback-why says, written out: review when a line of the review's rule is
// FAIL; else check when keep kept the work (a line that starts with kept:) and the final Check's status
// is a failure (a number other than 0, not none); else nothing.
func fallbackReason(rule, keep, final string) string {
	for _, l := range strings.Split(rule, "\n") {
		if l == "FAIL" {
			return "review"
		}
	}
	kept := false
	for _, l := range strings.Split(keep, "\n") {
		if strings.HasPrefix(l, "kept:") {
			kept = true
		}
	}
	status := strings.TrimSpace(final)
	failed := status != "" && status != "0" && strings.Trim(status, "0123456789") == "" && !strings.HasPrefix(status, "0")
	if kept && failed {
		return "check"
	}
	return ""
}

// fallback-why says once what the three fallback steps test. On every state of the run folder below it
// says the rule above; where the review failed it agrees with the shell line the steps carried before.
func TestFanoutFallbackWhySaysTheRule(t *testing.T) {
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
	// Every line keep writes (keep.yaml), with the statuses it puts in them, then lines it does not write.
	keeps := []string{
		"kept: the direct attempt, untouched\n",
		"kept: the team's work (check 0, judge PASS; the direct attempt's check was 1)\n",
		"kept: the team's work (check none, judge PASS; the direct attempt's check was 1)\n",
		"kept: the repair (check 0 with the reproduction, judge PASS; the direct attempt passed test-0, the review failed it)\n",
		"put back: the direct attempt (the team's work did not prove better: check 1, judge FAIL)\n",
		"put back: the direct attempt, which passed test-0 (the repair after the review did not prove better: check 1, judge none; it is in x/repair.patch, not applied)\n",
		"kept: the team's work (check 1, judge none; the direct attempt's check was 1)\ncheck 1\n",
		"check 1\n",
		"", none,
	}
	finals := []string{"0\n", "1\n", "2\n", "none\n", "0", "1", "01\n", "", none}
	fired := map[string]int{}
	for _, rule := range rules {
		for _, keep := range keeps {
			for _, final := range finals {
				dir := t.TempDir()
				run := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-p")
				if err := os.MkdirAll(run, 0o755); err != nil {
					t.Fatal(err)
				}
				files := map[string]string{"review-rule.txt": rule, "keep.txt": keep, "final-status.txt": final}
				for name, content := range files {
					if content != none {
						os.WriteFile(filepath.Join(run, name), []byte(content), 0o644)
					}
				}
				read := func(name string) string {
					if files[name] == none {
						return ""
					}
					return files[name]
				}
				want := fallbackReason(read("review-rule.txt"), read("keep.txt"), read("final-status.txt"))
				got := strings.TrimSpace(ruleAnswer(t, why.Text, dir))
				if got != want {
					t.Errorf("rule %q, keep %q, final %q: fallback-why says %q, want %q", rule, keep, final, got, want)
				}
				fired[got]++
				if got == "review" {
					cmd := exec.Command("bash", "-c", shellFallbackWhen)
					cmd.Dir = dir
					if cmd.Run() != nil {
						t.Errorf("rule %q: the review failed, and the shell line did not hold", rule)
					}
				}
			}
		}
	}
	if fired["review"] == 0 || fired["check"] == 0 || fired[""] == 0 {
		t.Fatalf("answers %v: the table proves nothing", fired)
	}
	t.Logf("%d states: %v", len(rules)*len(keeps)*len(finals), fired)
}

// The check branch the shell line carried could not fire on what keep writes: it asked for a line that is
// exactly "check 1". On a kept direct attempt whose final Check failed, fallback-why now says check where
// the shell line held for nothing.
func TestFanoutFallbackFiresOnAKeptWorkWhoseFinalCheckFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the condition it is compared with is a bash line")
	}
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve-fanout"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in team-solve-fanout beside this package")
	}
	why := load(t, src, &fakeRunner{}).Defs["team-solve-fanout.steps.fallback-why"]
	dir := t.TempDir()
	run := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-p")
	os.MkdirAll(run, 0o755)
	os.WriteFile(filepath.Join(run, "review-rule.txt"), []byte("PASS\n"), 0o644)
	os.WriteFile(filepath.Join(run, "keep.txt"), []byte("kept: the direct attempt, untouched\n"), 0o644)
	os.WriteFile(filepath.Join(run, "final-status.txt"), []byte("1\n"), 0o644)
	cmd := exec.Command("bash", "-c", shellFallbackWhen)
	cmd.Dir = dir
	if cmd.Run() == nil {
		t.Fatal("the old shell line held: the bug this test is about is gone from it")
	}
	if got := strings.TrimSpace(ruleAnswer(t, why.Text, dir)); got != "check" {
		t.Fatalf("fallback-why says %q, want check", got)
	}
}
