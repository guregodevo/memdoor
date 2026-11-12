package decision

import (
	"context"
	"sync"
	"testing"
	"time"
)

// serialProvider is a local runtime: it takes one request at a time.
type serialProvider struct {
	fakeProvider
	parallel int
}

func (p *serialProvider) Parallel() int { return p.parallel }

// The service answers with what the provider it would pick says, and with
// DefaultParallel whenever it cannot know (issue #288).
func TestServiceParallel(t *testing.T) {
	local := &serialProvider{fakeProvider: fakeProvider{id: "local", ready: true}, parallel: 1}
	hosted := &fakeProvider{id: "hosted", ready: true}
	timed := &serialProvider{fakeProvider: fakeProvider{id: "timed", ready: true}, parallel: 0}
	svcFor := func(ref string, provs ...Provider) *service {
		s, err := NewService(func(string) string { return ref }, provs...)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		return s.(*service)
	}
	cases := []struct {
		name string
		svc  *service
		want int
	}{
		{"a local runtime serves one", svcFor("local", local), 1},
		{"a provider saying nothing serves the default", svcFor("hosted", hosted), DefaultParallel},
		{"a provider saying zero serves the default", svcFor("timed", timed), DefaultParallel},
		{"no model configured", svcFor("", hosted), DefaultParallel},
		{"decisions off", svcFor("off", hosted), DefaultParallel},
		{"unknown provider", svcFor("nope/x", hosted), DefaultParallel},
	}
	for _, c := range cases {
		if got := c.svc.Parallel(""); got != c.want {
			t.Fatalf("%s: Parallel() = %d, want %d", c.name, got, c.want)
		}
	}
}

// The service is what the judge asks, so it must answer through
// ServiceParallelism, not only through a concrete provider.
func TestServiceIsAServiceParallelism(t *testing.T) {
	svc, err := NewService(func(string) string { return "local" }, &serialProvider{fakeProvider: fakeProvider{id: "local", ready: true}, parallel: 1})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	p, ok := svc.(ServiceParallelism)
	if !ok || p.Parallel("") != 1 {
		t.Fatalf("service must answer ServiceParallelism with the provider's number: %v", ok)
	}
}

// countingRuntime is a local runtime that answers one yes/no after hold and
// records the most requests it had at once.
type countingRuntime struct {
	parallel int
	hold     time.Duration
	mu       sync.Mutex
	now, top int
}

func (c *countingRuntime) ID() string    { return "local" }
func (c *countingRuntime) Ready() bool   { return true }
func (c *countingRuntime) Parallel() int { return c.parallel }
func (c *countingRuntime) Evaluate(ctx context.Context, _ string, _ Request) (Result, error) {
	c.mu.Lock()
	c.now++
	c.top = max(c.top, c.now)
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.now--; c.mu.Unlock() }()
	select {
	case <-time.After(c.hold):
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	return okResult("q"), nil
}

func (c *countingRuntime) peak() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.top
}

func oneQuestion(t *testing.T) Request {
	t.Helper()
	q, err := Boolean("Is it?", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return Request{State: "s", Questions: map[string]Question{"q": q}}
}

// Every caller of the service shares the provider's slots: four callers at
// once (a judged read, the tool routing, the effort, the gate) send a local
// runtime one request at a time, and each is answered (issue #288).
func TestCallersShareALocalRuntimesSlots(t *testing.T) {
	rt := &countingRuntime{parallel: 1, hold: 20 * time.Millisecond}
	svc, err := NewService(func(string) string { return "local" }, rt)
	if err != nil {
		t.Fatal(err)
	}
	req := oneQuestion(t)
	var wg sync.WaitGroup
	results := make([]Result, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = svc.Evaluate(context.Background(), req, Options{})
		}()
	}
	wg.Wait()
	if p := rt.peak(); p != 1 {
		t.Fatalf("a runtime that serves one got %d at once", p)
	}
	for i, r := range results {
		if !r.OK() {
			t.Fatalf("caller %d: %s %s", i, r.Reason, r.Detail)
		}
	}
}

// A request's timeout starts when it is sent, not while it waits for a slot:
// two 60 ms requests under a 100 ms timeout are both answered, the second
// after 120 ms in all.
func TestTheWaitForASlotIsNotTheRequestsTimeout(t *testing.T) {
	rt := &countingRuntime{parallel: 1, hold: 60 * time.Millisecond}
	svc, err := NewService(func(string) string { return "local" }, rt)
	if err != nil {
		t.Fatal(err)
	}
	req := oneQuestion(t)
	var wg sync.WaitGroup
	results := make([]Result, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = svc.Evaluate(context.Background(), req, Options{Timeout: 100 * time.Millisecond})
		}()
	}
	wg.Wait()
	for i, r := range results {
		if !r.OK() {
			t.Fatalf("caller %d: %s %s", i, r.Reason, r.Detail)
		}
	}
}

// A slot that does not come within the caller's timeout is a deadline, and a
// hosted provider has no slots at all.
func TestASlotThatDoesNotComeIsADeadline(t *testing.T) {
	rt := &countingRuntime{parallel: 1, hold: 200 * time.Millisecond}
	svc, err := NewService(func(string) string { return "local" }, rt)
	if err != nil {
		t.Fatal(err)
	}
	req := oneQuestion(t)
	go svc.Evaluate(context.Background(), req, Options{})
	for rt.peak() == 0 {
		time.Sleep(time.Millisecond)
	}
	if r := svc.Evaluate(context.Background(), req, Options{Timeout: 30 * time.Millisecond}); r.Reason != ReasonDeadline {
		t.Fatalf("no slot within the timeout: %s %s", r.Reason, r.Detail)
	}
	if n := fewSlots(&countingRuntime{parallel: DefaultParallel}); n != 0 {
		t.Fatalf("a provider serving the default is not limited: %d", n)
	}
}
