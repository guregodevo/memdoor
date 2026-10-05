package tools

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/pkg/decision"
)

type serviceFunc func(ctx context.Context, req decision.Request, opts decision.Options) decision.Result

func (f serviceFunc) Evaluate(ctx context.Context, req decision.Request, opts decision.Options) decision.Result {
	return f(ctx, req, opts)
}

func shortJudgeRetryWait(t *testing.T) {
	t.Helper()
	prev := judgeRetryWait
	judgeRetryWait = time.Millisecond
	t.Cleanup(func() { judgeRetryWait = prev })
}

// One overloaded answer per batch must not cost the judgment: the batch asks
// again and the judge returns scores (live 2026-09-28: one 503 upstream reset
// answered every jev tool UNJUDGED for the turn).
func TestJudgeRelevanceRetriesAnOverloadedBatch(t *testing.T) {
	shortJudgeRetryWait(t)
	var calls atomic.Int32
	svc := serviceFunc(func(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
		if calls.Add(1) <= 2 {
			return decision.Unavailable(decision.ReasonOverloaded, "HTTP 503: upstream connect error")
		}
		r := decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{}}
		for id, q := range req.Questions {
			p := 0.1
			if strings.Contains(q.Instructions, "resumeAfterRestart") {
				p = 0.9
			}
			r.Answers[id] = decision.Answer{Kind: decision.KindBoolean, ProbabilityTrue: p}
		}
		return r
	})
	items := make([]string, 30)
	for i := range items {
		items[i] = fmt.Sprintf("func filler%d() {};", i)
	}
	items[7] = "func resumeAfterRestart() {}"
	ps, err := judgeRelevance(context.Background(), svc, relevanceSpec{
		Task:     "where is a turn resumed after restart",
		Question: "Relevant?",
		Purpose:  "test",
	}, items)
	if err != nil {
		t.Fatalf("one 503 per batch must be retried: %v", err)
	}
	if calls.Load() != 4 || ps[7] != 0.9 || ps[0] != 0.1 || ps[24] != 0.1 {
		t.Fatalf("calls=%d ps[7]=%v ps[0]=%v ps[24]=%v", calls.Load(), ps[7], ps[0], ps[24])
	}
}

// An overload that outlasts the retries is still an unavailable judgment, so
// callers keep their degrade-to-unjudged path.
func TestJudgeRelevanceStillFailsWhenOverloadPersists(t *testing.T) {
	shortJudgeRetryWait(t)
	svc := serviceFunc(func(context.Context, decision.Request, decision.Options) decision.Result {
		return decision.Unavailable(decision.ReasonOverloaded, "HTTP 503: upstream connect error")
	})
	ps, err := judgeRelevance(context.Background(), svc, relevanceSpec{
		Task:     "t",
		Question: "Relevant?",
		Purpose:  "test",
	}, []string{"a", "b", "c"})
	if err == nil || !strings.Contains(err.Error(), "decision model overloaded") || len(ps) != 3 {
		t.Fatalf("persistent overload must still fail the judgment: %v %v", ps, err)
	}
}
