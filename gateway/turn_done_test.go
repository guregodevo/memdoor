package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"memdoor/pkg/decision"
)

type fakeDoneSvc struct {
	p     float64
	unav  bool
	state string
	calls int
}

func (f *fakeDoneSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.calls++
	f.state = req.State
	if f.unav {
		return decision.Unavailable(decision.ReasonNotConfigured, "test")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"unfinished": {Kind: decision.KindBoolean, ProbabilityTrue: f.p},
	}}
}

func patched(file, build string) ToolExecutionInfo {
	out := "Applied: M " + file
	if build != "" {
		out += "\n\nverify: " + build
	}
	return ToolExecutionInfo{Name: "apply_patch", Input: `{"input":"*** Begin Patch\n*** Update File: ` + file + `\n@@\n-a\n+b\n*** End Patch"}`, Output: out}
}

func verified(cmd string, pass bool) ToolExecutionInfo {
	head := "FAIL — `" + cmd + "` (exit status 1)\n\nfoo_test.go:12: want 3"
	if pass {
		head = "PASS — `" + cmd + "`\n\nok  memdoor/x 0.1s"
	}
	return ToolExecutionInfo{Name: "verify", Input: `{"command":"` + cmd + `"}`, Output: head}
}

// The receipts read what the turn did from the tool records: a change with a
// passing check after it, a change with none, a change whose check failed.
func TestReceiptsReadTheToolRecords(t *testing.T) {
	r := receiptsOf([]ToolExecutionInfo{
		{Name: "read_file", Input: `{"path":"sum.go"}`, Output: "…"},
		patched("sum.go", "PASS — `go build ./...` compiled clean."),
		verified("go test ./...", true),
	})
	if len(r.changed) != 1 || r.changed[0] != "sum.go" || len(r.checks) != 2 || !r.checkedAfterChange || r.failingAfterChange {
		t.Fatalf("%+v", r)
	}
	if line := r.line(); !strings.HasPrefix(line, "✓ checked: 1 file changed") || !strings.Contains(line, "`go test ./...` PASS") {
		t.Fatalf("line: %q", line)
	}

	// edit_file names its file file_path; a bash run is a check (live
	// 2026-10-03: a task that wrote SIGNALS.md by python3 through bash
	// recorded 0 changes, 0 checks, and was judged unfinished twice).
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "edit_file", Input: `{"file_path":"signals.py","new_string":"x"}`, Output: "ok"},
		{Name: "bash", Input: `{"command":"python3 signals.py > SIGNALS.md"}`, Output: ""},
	})
	if len(r.changed) != 1 || r.changed[0] != "signals.py" || len(r.checks) != 1 || !r.checkedAfterChange || r.checks[0].label != "python3 signals.py > SIGNALS.md" {
		t.Fatalf("edit_file + bash: %+v", r)
	}
	// A READ-BACK of the file just written is its check (live 2026-10-05: a
	// child wrote hello.txt, `cat`ed it, and was judged unfinished at 0.90
	// on "nothing was checked after the last change"). A read of something
	// else, or before any change, stays a read.
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "bash", Input: `{"command":"cat hello.txt"}`, Output: "(exit status 1)"},
		{Name: "write_file", Input: `{"path":"hello.txt","content":"hi"}`, Output: "ok"},
		{Name: "bash", Input: `{"command":"ls -la hello.txt && cat hello.txt"}`, Output: "-rw 3 hello.txt\nhi"},
	})
	if len(r.checks) != 1 || !r.checkedAfterChange || r.checks[0].label != "read back: ls -la hello.txt && cat hello.txt" {
		t.Fatalf("read-back: %+v", r)
	}
	if ev := r.evidence(); !strings.Contains(ev, "read back: ls -la hello.txt && cat hello.txt → PASS · printed: -rw 3 hello.txt") {
		t.Fatalf("evidence should carry the check's output head: %q", ev)
	}
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "write_file", Input: `{"path":"hello.txt","content":"hi"}`, Output: "ok"},
		{Name: "bash", Input: `{"command":"ls -la"}`, Output: "total 8"},
	})
	if len(r.checks) != 0 || r.checkedAfterChange {
		t.Fatalf("a read of something else is not a check: %+v", r)
	}
	// A read is not a check: a probe for a missing folder is no FAIL
	// (live 2026-10-04, "ls .agents 2>/dev/null FAIL" on the receipt line).
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "bash", Input: `{"command":"ls -a; echo ---; ls .agents 2>/dev/null"}`, Output: ".\n..\n---\n\n(exit status 1. …)"},
		{Name: "write_file", Input: `{"path":"a.py","content":"x"}`, Output: "ok"},
		{Name: "bash", Input: `{"command":"python3 a.py"}`, Output: "ok"},
	})
	if len(r.checks) != 1 || r.checks[0].label != "python3 a.py" || r.failingAfterChange {
		t.Fatalf("reads count as checks: %+v", r)
	}
	// A bash command that failed says so in its output, not in Error.
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "write_file", Input: `{"path":"wordfreq.go","content":"x"}`, Output: "ok"},
		{Name: "bash", Input: `{"command":"go build ./..."}`, Output: "Command FAILED (exit status 1): `go build ./...`\nOutput: ./wordfreq.go:12:42: undefined: Entry"},
	})
	if !r.failingAfterChange || r.checkedAfterChange || r.checks[0].pass {
		t.Fatalf("a failed bash build is a failing check: %+v", r)
	}
	r = receiptsOf([]ToolExecutionInfo{{Name: "write_file", Input: `{"path":"a.py","content":"x"}`, Output: "ok"}})
	if r.checkedAfterChange || len(r.checks) != 0 || r.line() != "⚠ unverified: 1 file changed · no build, test or run after it" {
		t.Fatalf("unverified: %q %+v", r.line(), r)
	}

	r = receiptsOf([]ToolExecutionInfo{
		verified("go test ./...", true),
		patched("sum.go", "PASS — `go build ./...` compiled clean."),
		verified("go test ./...", false),
	})
	if !r.failingAfterChange || r.checkedAfterChange || !strings.HasPrefix(r.line(), "⚠ failing after the change: ") {
		t.Fatalf("failing: %q %+v", r.line(), r)
	}
	if why := r.reasons(); len(why) != 1 || !strings.Contains(why[0], "FAILED") {
		t.Fatalf("reasons: %v", why)
	}

	// A patch whose own build failed is a failing check, even with no verify.
	r = receiptsOf([]ToolExecutionInfo{patched("sum.go", "FAIL — `go build ./...`:\nsum.go:3: undefined: x")})
	if !r.failingAfterChange || r.checks[0].label != "go build ./..." {
		t.Fatalf("%+v", r)
	}

	// Commands with no change never earn a ✓.
	r = receiptsOf([]ToolExecutionInfo{
		{Name: "bash", Input: `{"command":"mkdir -p wordfreq"}`, Output: ""},
		{Name: "bash", Input: `{"command":"go mod init x"}`, Output: "Command FAILED (exit status 1): `go mod init x`"},
	})
	if line := r.line(); !strings.HasPrefix(line, "⚠ nothing changed · ran: ") || !strings.Contains(line, "`go mod init x` FAIL") {
		t.Fatalf("no change, no tick: %q", line)
	}
	// A read-only turn has no receipt line: nothing changed, nothing checked.
	if line := receiptsOf([]ToolExecutionInfo{{Name: "grep", Input: `{}`, Output: "x"}}).line(); line != "" {
		t.Fatalf("read-only turn must have no line, got %q", line)
	}

	// A failed mutation is not a change.
	r = receiptsOf([]ToolExecutionInfo{{Name: "apply_patch", Input: `{"input":"*** Begin Patch\n*** Update File: a.go\n@@\n-a\n+b\n*** End Patch"}`, Error: "context not found"}})
	if len(r.changed) != 0 || !strings.HasPrefix(r.lastFailure, "apply_patch: context not found") {
		t.Fatalf("%+v", r)
	}
}

// One yes/no over the receipts, the request and the final message. Off,
// unavailable or under the threshold leaves the turn alone; a yes sends it
// back with the receipts' reason in the nudge.
func TestTurnDoneJudgesTheReceipts(t *testing.T) {
	unverified := receiptsOf([]ToolExecutionInfo{patched("sum.go", "")})
	for _, c := range []struct {
		name string
		svc  *fakeDoneSvc
		set  map[string]string
		want bool
	}{
		{"unfinished", &fakeDoneSvc{p: 0.9}, nil, true},
		{"done", &fakeDoneSvc{p: 0.1}, nil, false},
		{"unavailable", &fakeDoneSvc{unav: true}, nil, false},
		{"off", &fakeDoneSvc{p: 0.99}, map[string]string{settingTurnDone: "off"}, false},
		{"threshold raised", &fakeDoneSvc{p: 0.75}, map[string]string{settingTurnDoneThreshold: "0.9"}, false},
	} {
		v := &turnVerdict{svc: c.svc, read: settings(c.set)}
		if _, got := v.unfinished(context.Background(), "coder", "fix the off-by-one in sum.go", "Done, sum.go is fixed.", unverified); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	// A passing check after the last change is proof: the judge is not asked,
	// whatever it would have said (live 2026-10-03: it said 0.84 on a correct,
	// vet-and-race-tested result).
	proven := receiptsOf([]ToolExecutionInfo{patched("sum.go", ""), verified("go test -race ./...", true)})
	never := &fakeDoneSvc{p: 0.99}
	if _, got := (&turnVerdict{svc: never, read: settings(nil)}).unfinished(context.Background(), "coder", "fix it", "Done.", proven); got || never.calls != 0 {
		t.Fatalf("a passing check after the change ends the turn without the judge: retry=%v calls=%d", got, never.calls)
	}
	f := &fakeDoneSvc{p: 0.9}
	v := &turnVerdict{svc: f, read: settings(nil)}
	v.unfinished(context.Background(), "coder", "fix the off-by-one in sum.go", "Done, sum.go is fixed.", unverified)
	for _, want := range []string{"Request: fix the off-by-one", "Files changed: sum.go", "No build, test or run was made.", "Nothing was checked after the last change.", "Agent's final message:\nDone, sum.go is fixed."} {
		if !strings.Contains(f.state, want) {
			t.Errorf("state lacks %q:\n%s", want, f.state)
		}
	}
	if n := doneNudge(unverified); !strings.HasPrefix(n, "Not done yet: nothing was built, tested or run after your last change.") {
		t.Fatalf("nudge: %q", n)
	}
	// No mechanical reason: the nudge still says why in the judge's terms.
	if n := doneNudge(receiptsOf([]ToolExecutionInfo{patched("sum.go", "PASS — `go build ./...` ok"), verified("go test ./...", true)})); !strings.Contains(n, "not shown done by what this turn ran") {
		t.Fatalf("nudge: %q", n)
	}
	var none *turnVerdict
	if _, got := none.unfinished(context.Background(), "coder", "x", "y", unverified); got {
		t.Error("nil verdict must be false")
	}
}

// A budget runs the turn on: the first continuation is free, the rest spend
// the budget; the model's own last words become the next prompt; the
// system prompt says to ask first.
func TestTurnBudgetContinues(t *testing.T) {
	if !continueAllowed(0, 0, 0) || continueAllowed(1, 0, 0) {
		t.Fatal("no budget: exactly one continuation")
	}
	if !continueAllowed(3, 90_000, 100_000) || continueAllowed(3, 100_000, 100_000) {
		t.Fatal("a budget continues until spent")
	}
	v := &turnVerdict{read: settings(map[string]string{settingTurnBudget: "250000"})}
	if v.tokenBudget() != 250_000 || (&turnVerdict{read: settings(nil)}).tokenBudget() != 0 || (&turnVerdict{read: settings(map[string]string{settingTurnBudget: "-5"})}).tokenBudget() != 0 {
		t.Fatal("budget parse")
	}
	r := receiptsOf([]ToolExecutionInfo{patched("sum.go", "")})
	n := continueNudge("I fixed the loop bound. Next I will run go test ./... to confirm.", r)
	for _, want := range []string{"You ended on: \"", "Next I will run go test ./...", "nothing was built, tested or run after your last change", "Do that next step now"} {
		if !strings.Contains(n, want) {
			t.Errorf("nudge lacks %q: %s", want, n)
		}
	}
	if continueNudge("   ", r) != doneNudge(r) {
		t.Error("no last words: the receipts' nudge")
	}
	if p := unattendedPrompt(250_000); !strings.Contains(p, "250k tokens") || !strings.Contains(p, "ask_user_question") || !strings.Contains(p, "before changing anything") || !strings.Contains(p, "you do, not ask") {
		t.Fatalf("unattended prompt: %s", p)
	}
}

// A declared done is checked, not judged: the file is there or it is not,
// the command exits 0 or it does not; anything else is not a declaration.
func TestDoneWhenChecksTheArtifact(t *testing.T) {
	dir := t.TempDir()
	if label, ok, declared := doneWhen(context.Background(), dir, "file SIGNALS.md"); !declared || ok || label != "target SIGNALS.md" {
		t.Fatalf("missing file: %q %v %v", label, ok, declared)
	}
	if err := os.WriteFile(filepath.Join(dir, "SIGNALS.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := doneWhen(context.Background(), dir, "file SIGNALS.md"); !ok {
		t.Fatal("the file exists now")
	}
	if _, ok, declared := doneWhen(context.Background(), dir, "command: test -f SIGNALS.md"); !declared || !ok {
		t.Fatal("command exits 0")
	}
	if _, ok, _ := doneWhen(context.Background(), dir, "command: false"); ok {
		t.Fatal("command exits 1")
	}
	if _, _, declared := doneWhen(context.Background(), dir, ""); declared {
		t.Fatal("no declaration")
	}
	if n := doneWhenNudge("target SIGNALS.md"); !strings.Contains(n, "SIGNALS.md does not exist") {
		t.Fatalf("nudge: %s", n)
	}
}

type fakeExitZeroSvc struct {
	p     float64
	calls int
}

func (f *fakeExitZeroSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.calls++
	p := 0.05
	if strings.Contains(req.State, "FAIL") {
		p = f.p
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{"failed": {Kind: decision.KindBoolean, ProbabilityTrue: p}}}
}

// Exit 0 is not a pass when the output says it failed (a pipe into tail
// hides the status); a run whose output says nothing failed stays a pass; each
// output is judged once.
func TestExitZeroOutputThatFailedIsAFailingCheck(t *testing.T) {
	exitZeroSeen = sync.Map{}
	piped := ToolExecutionInfo{Name: "bash", Input: `{"command":"run-tests 2>&1 | tail -20"}`, Output: "--- FAIL: TestReset\nFAIL"}
	listing := ToolExecutionInfo{Name: "bash", Input: `{"command":"python3 report.py"}`, Output: "a.txt\nb.txt"}
	f := &fakeExitZeroSvc{p: 0.95}
	v := &turnVerdict{svc: f, read: settings(nil)}
	r := v.judgeExitZeroChecks(context.Background(), receiptsOf([]ToolExecutionInfo{patched("reset.go", ""), listing, piped}))
	if r.checks[0].pass != true || r.checks[1].pass != false || !r.failingAfterChange || r.checkedAfterChange {
		t.Fatalf("a clean run passes, the piped FAIL fails and decides: %+v", r)
	}
	if !strings.HasPrefix(r.line(), "⚠ failing after the change: ") {
		t.Fatalf("line: %q", r.line())
	}
	calls := f.calls
	v.judgeExitZeroChecks(context.Background(), receiptsOf([]ToolExecutionInfo{patched("reset.go", ""), listing, piped}))
	if f.calls != calls {
		t.Fatalf("each output is judged once, asked again %d times", f.calls-calls)
	}
	// No model: receipts unchanged.
	if r := (&turnVerdict{}).judgeExitZeroChecks(context.Background(), receiptsOf([]ToolExecutionInfo{patched("x.go", ""), piped})); !r.checkedAfterChange {
		t.Fatalf("no model leaves the exit status standing: %+v", r)
	}
}

type stateRecorder struct{ state string }

func (s *stateRecorder) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	s.state = req.State
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{"failed": {Kind: decision.KindBoolean, ProbabilityTrue: 0.1}}}
}

// The judge reads the whole command, not its first line: what a run means to
// test (an error path fed bad input on purpose) often comes after it.
func TestExitZeroJudgeReadsTheWholeCommand(t *testing.T) {
	exitZeroSeen = sync.Map{}
	rec := &stateRecorder{}
	v := &turnVerdict{svc: rec, read: settings(nil)}
	v.exitZeroFailed(context.Background(), "python3 qant.py pnl\necho '=== error paths ==='\npython3 qant.py pnl --strategy nosuch; echo \"exit=$?\"", "exit=1")
	if !strings.Contains(rec.state, "error paths") || !strings.Contains(rec.state, "--strategy nosuch") {
		t.Fatalf("the judge saw only part of the command:\n%s", rec.state)
	}
}

// The run view's receipt is the line a turn ends on.
func TestReceiptOfTheTurnText(t *testing.T) {
	if got := receiptOf("Done.\n\n✓ target SIGNALS.md exists · 1 file changed"); got != "✓ target SIGNALS.md exists · 1 file changed" {
		t.Fatalf("%q", got)
	}
	if got := receiptOf("Done.\n\n⚠ unverified: 1 file changed · no build, test or run after it\n"); !strings.HasPrefix(got, "⚠ unverified") {
		t.Fatalf("%q", got)
	}
	if receiptOf("an answer with no receipt") != "" {
		t.Fatal("no receipt")
	}
}

// A workflow task's budget overrides the workspace's for its turns.
func TestATaskBudgetOverridesTheWorkspace(t *testing.T) {
	v := &turnVerdict{read: settings(map[string]string{settingTurnBudget: "100000"})}
	s := &Session{ID: "workflow:w:p:t", Metadata: map[string]interface{}{}}
	if v.sessionBudget(s) != 100_000 {
		t.Fatal("no task budget: the workspace's")
	}
	s.SetMetadata(turnBudgetKey, int64(250_000))
	if v.sessionBudget(s) != 250_000 {
		t.Fatal("the task's budget wins")
	}
	if (&turnVerdict{read: settings(nil)}).sessionBudget(nil) != 0 {
		t.Fatal("nothing set: 0")
	}
}

func TestRenderPartitionMatchesMario(t *testing.T) {
	if got := renderPartition("file reports/x-{{.partition}}.md", "2026-10-03T183419"); got != "file reports/x-2026-10-03T183419.md" {
		t.Fatalf("%q", got)
	}
	if got := renderPartition("file a-{{ .partition }}.md", "p"); got != "file a-p.md" {
		t.Fatalf("%q", got)
	}
}
