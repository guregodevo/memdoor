package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/decision"
)

// scriptedJudge answers every verify question with one probability, or refuses
// the way a machine without a seat does, and counts how often it was asked.
type scriptedJudge struct {
	p       float64
	refuse  bool
	calls   int
	lastReq decision.Request
}

func (j *scriptedJudge) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	j.calls++
	j.lastReq = req
	if j.refuse {
		return decision.Unavailable(decision.ReasonNotConfigured, "no decision_model configured")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"ok": {Kind: decision.KindBoolean, ProbabilityTrue: j.p},
	}}
}

func runVerify(t *testing.T, command string) string {
	t.Helper()
	in, _ := json.Marshal(VerifyInput{Command: command, Dir: t.TempDir()})
	out, err := Verify(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func withJudge(t *testing.T, j decision.Service) {
	t.Helper()
	saved := decisionService()
	SetDecisionService(j)
	t.Cleanup(func() { SetDecisionService(saved) })
}

// EXIT 0 IS NOT A PASS BY ITSELF. A test runner that found nothing to run, a
// program that printed its own error and carried on: both exit 0, both used to
// come back PASS. With a seat, the decision model reads the output.
func TestVerifyExitZeroThatReadsAsFailureIsAFail(t *testing.T) {
	j := &scriptedJudge{p: 0.08}
	withJudge(t, j)
	out := runVerify(t, `echo "ok  	example.com/pkg	[no test files]"`)
	if !strings.HasPrefix(out, "FAIL") || !strings.Contains(out, "P(success)=0.08") {
		t.Fatalf("a judged failure on exit 0 must be a FAIL with its probability:\n%s", out)
	}
	if j.calls != 1 || !strings.Contains(j.lastReq.State, "no test files") {
		t.Fatalf("the judge must see the command's output, calls=%d state=%q", j.calls, j.lastReq.State)
	}
}

func TestVerifyExitZeroThatReadsAsSuccessStaysAPass(t *testing.T) {
	withJudge(t, &scriptedJudge{p: 0.97})
	if out := runVerify(t, `echo "Hello World"`); !strings.HasPrefix(out, "PASS") {
		t.Fatalf("a judged success stays a PASS:\n%s", out)
	}
}

// A NON-ZERO EXIT IS FINAL. The judge is never asked, so it can never turn a
// failure into a pass — the one mistake that would ship broken code.
func TestVerifyNonZeroExitIsAFailWithoutAskingTheJudge(t *testing.T) {
	j := &scriptedJudge{p: 0.99} // a judge that would say "fine" to anything
	withJudge(t, j)
	out := runVerify(t, `echo "looks fine to me"; exit 1`)
	if !strings.HasPrefix(out, "FAIL") {
		t.Fatalf("exit 1 must be a FAIL whatever a judge would say:\n%s", out)
	}
	if j.calls != 0 {
		t.Fatalf("the judge must not be asked about a failing exit, asked %d times", j.calls)
	}
}

// WITHOUT A SEAT, NOTHING CHANGES. The decision service answers unavailable and
// the verdict is the exit code, exactly as before the judge existed.
func TestVerifyWithoutASeatIsTheExitCode(t *testing.T) {
	withJudge(t, &scriptedJudge{refuse: true})
	if out := runVerify(t, `echo "[no test files]"`); !strings.HasPrefix(out, "PASS") {
		t.Fatalf("no seat means the exit code decides:\n%s", out)
	}
	withJudge(t, nil)
	if out := runVerify(t, `echo hi`); !strings.HasPrefix(out, "PASS") {
		t.Fatalf("no decision service at all means the exit code decides:\n%s", out)
	}
}
