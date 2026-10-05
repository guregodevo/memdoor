package decision

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	id    string
	ready bool
	res   Result
	err   error
	sleep time.Duration
	seen  string
}

func (f *fakeProvider) ID() string  { return f.id }
func (f *fakeProvider) Ready() bool { return f.ready }
func (f *fakeProvider) Evaluate(ctx context.Context, model string, req Request) (Result, error) {
	f.seen = model
	if f.sleep > 0 {
		select {
		case <-time.After(f.sleep):
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	return f.res, f.err
}

func okResult(ids ...string) Result {
	r := Result{Status: StatusOK, Answers: map[string]Answer{}}
	for _, id := range ids {
		r.Answers[id] = Answer{Kind: KindBoolean, ProbabilityTrue: 0.9}
	}
	return r
}

func boolReq() Request {
	q, _ := Boolean("Reply?", "", "")
	return Request{State: "hello", Questions: map[string]Question{"reply": q}}
}

func TestQuestionValidation(t *testing.T) {
	if _, err := Choice("q", Labels("a")); err == nil {
		t.Fatal("one option accepted")
	}
	if _, err := Choice("q", Labels("a", "a")); err == nil {
		t.Fatal("duplicate labels accepted")
	}
	if _, err := Score("q", make([]string, 11)); err == nil {
		t.Fatal("11 levels accepted")
	}
	if _, err := Boolean(" ", "", ""); err == nil {
		t.Fatal("empty instructions accepted")
	}
	if _, err := ParseKind("maybe"); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if k, _ := ParseKind("noul"); k != KindBoolean {
		t.Fatalf("noul -> %q, want boolean", k)
	}
	if err := (Request{State: "", Questions: nil}).Validate(); err == nil {
		t.Fatal("empty request accepted")
	}
}

func TestModelRef(t *testing.T) {
	ref, off, err := ParseModelRef(" systemone/jev-latest ")
	if err != nil || off || ref.Provider != "systemone" || ref.Model != "jev-latest" {
		t.Fatalf("got %+v off=%v err=%v", ref, off, err)
	}
	if _, off, _ := ParseModelRef("OFF"); !off {
		t.Fatal("off not recognised")
	}
	if ref, _, _ := ParseModelRef(""); ref.Provider != "" {
		t.Fatal("empty should be unconfigured")
	}
	if (ModelRef{Provider: "local"}).String() != "local" || (ModelRef{"a", "b"}).String() != "a/b" {
		t.Fatal("String round trip")
	}
}

func TestServiceRoutesAndNeverFallsBack(t *testing.T) {
	setting := ""
	loc := &fakeProvider{id: "local", ready: true, res: okResult("reply")}
	so := &fakeProvider{id: "systemone", ready: false}
	svc, err := NewService(func(string) string { return setting }, loc, so)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if r := svc.Evaluate(ctx, boolReq(), Options{}); r.Status != StatusUnavailable || r.Reason != ReasonNotConfigured {
		t.Fatalf("unconfigured: %+v", r)
	}
	setting = "off"
	if r := svc.Evaluate(ctx, boolReq(), Options{}); r.Reason != ReasonDisabled {
		t.Fatalf("disabled: %+v", r)
	}
	setting = "nope"
	if r := svc.Evaluate(ctx, boolReq(), Options{}); r.Reason != ReasonNotConfigured {
		t.Fatalf("unknown provider: %+v", r)
	}
	setting = "systemone/jev-latest"
	if r := svc.Evaluate(ctx, boolReq(), Options{}); r.Reason != ReasonNotConfigured {
		t.Fatalf("not-ready provider must be unavailable, not routed elsewhere: %+v", r)
	}
	setting = "local/whatever"
	r := svc.Evaluate(ctx, boolReq(), Options{})
	if !r.OK() || r.Provider != "local" || loc.seen != "whatever" {
		t.Fatalf("routed: %+v seen=%q", r, loc.seen)
	}
	if r := svc.Evaluate(ctx, Request{State: "x"}, Options{}); r.Reason != ReasonUnsupportedInput {
		t.Fatalf("invalid request: %+v", r)
	}
}

func TestServiceClassifiesFailures(t *testing.T) {
	slow := &fakeProvider{id: "local", ready: true, res: okResult("reply"), sleep: time.Second}
	svc, _ := NewService(func(string) string { return "local" }, slow)
	if r := svc.Evaluate(context.Background(), boolReq(), Options{Timeout: 20 * time.Millisecond}); r.Reason != ReasonDeadline {
		t.Fatalf("timeout: %+v", r)
	}
	broken := &fakeProvider{id: "local", ready: true, err: errors.New("boom")}
	svc, _ = NewService(func(string) string { return "local" }, broken)
	if r := svc.Evaluate(context.Background(), boolReq(), Options{}); r.Reason != ReasonProviderError {
		t.Fatalf("error: %+v", r)
	}
	partial := &fakeProvider{id: "local", ready: true, res: okResult("other")}
	svc, _ = NewService(func(string) string { return "local" }, partial)
	if r := svc.Evaluate(context.Background(), boolReq(), Options{}); r.Reason != ReasonProviderError {
		t.Fatalf("missing answer: %+v", r)
	}
	if _, err := NewService(nil); err == nil {
		t.Fatal("nil resolver accepted")
	}
	if _, err := NewService(func(string) string { return "" }, &fakeProvider{id: "x"}, &fakeProvider{id: "x"}); err == nil {
		t.Fatal("duplicate provider accepted")
	}
}
