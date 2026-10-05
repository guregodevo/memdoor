package gateway

import (
	"context"
	"strings"
	"testing"

	"memdoor/pkg/decision"
)

type fakeVerdictSvc struct {
	p     float64
	off   float64
	unav  bool
	calls int
	state string
}

func (f *fakeVerdictSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.calls++
	f.state = req.State
	if f.unav {
		return decision.Unavailable(decision.ReasonNotConfigured, "test")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"announces":   {Kind: decision.KindBoolean, ProbabilityTrue: f.p},
		"off_request": {Kind: decision.KindBoolean, ProbabilityTrue: f.off},
	}}
}

func settings(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestTurnVerdict(t *testing.T) {
	reply := "Stage file confirmed: line #2 lives at 3–9s. Cutting the front clean, then captioning."
	for _, c := range []struct {
		name string
		svc  *fakeVerdictSvc
		set  map[string]string
		want bool
	}{
		{"announces", &fakeVerdictSvc{p: 0.83}, nil, true},
		{"finished", &fakeVerdictSvc{p: 0.02}, nil, false},
		{"unavailable keeps the phrase list's answer", &fakeVerdictSvc{unav: true}, nil, false},
		{"off", &fakeVerdictSvc{p: 0.99}, map[string]string{settingTurnVerdict: "off"}, false},
		{"off-request reply", &fakeVerdictSvc{p: 0.05, off: 0.95}, nil, true},
		{"on-request answer", &fakeVerdictSvc{p: 0.05, off: 0.3}, nil, false},
	} {
		v := &turnVerdict{svc: c.svc, read: settings(c.set)}
		if got := v.announcesUndone(context.Background(), "planner", "make a short", reply); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	var none *turnVerdict
	if none.announcesUndone(context.Background(), "coder", "x", reply) {
		t.Error("nil verdict must be false")
	}
}

// The state carries the request and the END of a long reply, where an
// announcement sits.
func TestTurnVerdictStateKeepsTheTail(t *testing.T) {
	f := &fakeVerdictSvc{p: 0.1}
	v := &turnVerdict{svc: f, read: settings(nil)}
	long := strings.Repeat("narration ", 400) + "Cutting the front clean, then captioning."
	v.announcesUndone(context.Background(), "planner", "make a short", long)
	if !strings.Contains(f.state, "Request: make a short") || !strings.HasSuffix(f.state, "then captioning.") {
		t.Fatalf("state: %q", f.state[:120])
	}
}
