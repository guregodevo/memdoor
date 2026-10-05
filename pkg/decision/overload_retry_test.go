package decision

import (
	"context"
	"testing"
	"time"
)

// overloadThenOK answers overloaded left times, then answers. Live
// 2026-09-28: one 503 upstream connect error reached consumers as an
// overloaded refusal they had to degrade on.
type overloadThenOK struct {
	id    string
	ready bool
	left  int
	calls int
}

func (p *overloadThenOK) ID() string  { return p.id }
func (p *overloadThenOK) Ready() bool { return p.ready }

func (p *overloadThenOK) Evaluate(_ context.Context, _ string, req Request) (Result, error) {
	p.calls++
	if p.left > 0 {
		p.left--
		return Unavailable(ReasonOverloaded, "HTTP 503: upstream connect error"), nil
	}
	r := Result{Status: StatusOK, Answers: map[string]Answer{}}
	for id := range req.Questions {
		r.Answers[id] = Answer{Kind: KindBoolean, ProbabilityTrue: 0.9}
	}
	return r, nil
}

func TestServiceRetriesOverload(t *testing.T) {
	p := &overloadThenOK{id: "local", ready: true, left: 2}
	svc, _ := NewService(func(string) string { return "local" }, p)
	if r := svc.Evaluate(context.Background(), boolReq(), Options{}); !r.OK() || p.calls != 3 {
		t.Fatalf("two 503s then ok: calls=%d res=%+v", p.calls, r)
	}

	p = &overloadThenOK{id: "local", ready: true, left: 99}
	svc, _ = NewService(func(string) string { return "local" }, p)
	if r := svc.Evaluate(context.Background(), boolReq(), Options{Timeout: 600 * time.Millisecond}); r.OK() || r.Reason != ReasonOverloaded || p.calls != 2 {
		t.Fatalf("a refusal past the caller's budget stands: calls=%d res=%+v", p.calls, r)
	}

	p = &overloadThenOK{id: "local", ready: true, left: 99}
	svc, _ = NewService(func(string) string { return "local" }, p)
	start := time.Now()
	if r := svc.Evaluate(context.Background(), boolReq(), Options{}); r.OK() || r.Reason != ReasonOverloaded || p.calls != overloadMaxAttempts {
		t.Fatalf("a lasting overload stops after %d asks: calls=%d res=%+v", overloadMaxAttempts, p.calls, r)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("a lasting overload must answer fast, took %v", took)
	}
}
