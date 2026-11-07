package gateway

import (
	"context"
	"strings"
	"testing"

	"memdoor/pkg/decision"
)

// fakeTaskDecisionSvc answers one probability and keeps the request.
type fakeTaskDecisionSvc struct {
	p    float64
	down bool
	req  decision.Request
	opts decision.Options
}

func (f *fakeTaskDecisionSvc) Evaluate(_ context.Context, req decision.Request, opts decision.Options) decision.Result {
	f.req, f.opts = req, opts
	if f.down {
		return decision.Result{Status: decision.StatusUnavailable, Reason: decision.ReasonDeadline, Detail: "2.5s passed"}
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{"holds": {Kind: decision.KindBoolean, ProbabilityTrue: f.p}}}
}

func settingsOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestADecisionTaskIsJudgedOnTheDecisionService(t *testing.T) {
	svc := &fakeTaskDecisionSvc{p: 0.83}
	d := &workflowDecider{svc: svc, read: settingsOf(nil)}
	p, err := d.Decide(context.Background(), "The change does what the task asks.", "verdict: PASS\nall tests green")
	if err != nil || p != 0.83 {
		t.Fatalf("p = %v, err = %v", p, err)
	}
	if svc.opts.Purpose != taskDecisionPurpose || svc.opts.Timeout != taskDecisionTimeout {
		t.Fatalf("options = %+v", svc.opts)
	}
	if q := svc.req.Questions["holds"]; q.Kind != decision.KindBoolean || q.Instructions != "The change does what the task asks." {
		t.Fatalf("question = %+v", q)
	}
	if !strings.Contains(svc.req.State, "all tests green") {
		t.Fatalf("state = %q", svc.req.State)
	}
}

func TestWithoutAServiceOrSwitchedOffTheDeciderErrsSoTheElseDecides(t *testing.T) {
	var none *workflowDecider
	if _, err := none.Decide(context.Background(), "x", "y"); err == nil {
		t.Fatal("a nil decider should err")
	}
	if _, err := (&workflowDecider{}).Decide(context.Background(), "x", "y"); err == nil {
		t.Fatal("no service should err")
	}
	svc := &fakeTaskDecisionSvc{p: 0.9}
	d := &workflowDecider{svc: svc, read: settingsOf(map[string]string{settingTaskDecision: "off"})}
	if _, err := d.Decide(context.Background(), "x", "y"); err == nil || !strings.Contains(err.Error(), "off") {
		t.Fatalf("switched off should err, got %v", err)
	}
	if svc.req.State != "" {
		t.Fatal("switched off must not call the service")
	}
	d = &workflowDecider{svc: &fakeTaskDecisionSvc{down: true}, read: settingsOf(nil)}
	if _, err := d.Decide(context.Background(), "x", "y"); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("an unavailable answer should err with its reason, got %v", err)
	}
}

func TestTheStateIsClippedHeadAndTail(t *testing.T) {
	svc := &fakeTaskDecisionSvc{p: 0.5}
	d := &workflowDecider{svc: svc, read: settingsOf(nil)}
	state := "TASK " + strings.Repeat("a", 20000) + " VERDICT"
	if _, err := d.Decide(context.Background(), "x", state); err != nil {
		t.Fatal(err)
	}
	if len(svc.req.State) > taskDecisionMaxState+16 {
		t.Fatalf("state not clipped: %d", len(svc.req.State))
	}
	if !strings.HasPrefix(svc.req.State, "TASK ") || !strings.HasSuffix(svc.req.State, " VERDICT") {
		t.Fatalf("head and tail should survive the clip: %q … %q", svc.req.State[:12], svc.req.State[len(svc.req.State)-12:])
	}
}

func TestTheDefaultThresholdIsTheWorkspacesThen07(t *testing.T) {
	if got := (&workflowDecider{read: settingsOf(nil)}).DefaultThreshold(); got != taskDecisionDefaultThreshold {
		t.Fatalf("default = %v", got)
	}
	if got := (&workflowDecider{read: settingsOf(map[string]string{settingTaskDecisionThreshold: "0.85"})}).DefaultThreshold(); got != 0.85 {
		t.Fatalf("workspace threshold = %v", got)
	}
	if got := (&workflowDecider{read: settingsOf(map[string]string{settingTaskDecisionThreshold: "1.5"})}).DefaultThreshold(); got != taskDecisionDefaultThreshold {
		t.Fatalf("an impossible threshold falls back: %v", got)
	}
}
