package decision

import (
	"context"
	"time"
)

// How many requests a decision endpoint serves at once. The judge behind the
// jev tools (jgrep, jread, jlogs) batches its questions, and it may not send
// more batches at once than the endpoint answers at once.
//
// A local runtime such as janis answers one at a time: its own compute lock
// serialises them, so eight requests sent together come back one after
// another, each after the one before it finished. With a 30 s wait
// (DefaultTimeout) the later ones come back as a deadline and the read goes
// unjudged (issue #288, 2026-11-11).

// DefaultParallel is how many requests a provider serves at once when it says
// nothing about itself: a hosted API takes them together.
const DefaultParallel = 8

// Parallelism is a provider saying how many requests it serves at once. A
// provider that does not implement it serves DefaultParallel.
type Parallelism interface {
	Parallel() int
}

// ServiceParallelism is the same question asked of a Service: how many
// requests at once the model it would pick for agentID serves.
type ServiceParallelism interface {
	Parallel(agentID string) int
}

// Parallel is how many requests at once the model this service would pick for
// agentID serves: a local runtime says 1, a hosted one many. Anything short of
// an answer is DefaultParallel, so a caller never plans to send fewer than it
// could.
func (s *service) Parallel(agentID string) int {
	ref, disabled, err := ParseModelRef(s.resolve(agentID))
	if err != nil || disabled {
		return DefaultParallel
	}
	p, ok := s.providers[ref.Provider]
	if !ok {
		return DefaultParallel
	}
	if pm, ok := p.(Parallelism); ok {
		if n := pm.Parallel(); n > 0 {
			return n
		}
	}
	return DefaultParallel
}

// fewSlots is how many requests at once p serves when that is fewer than
// DefaultParallel, else 0: a hosted provider is not limited here.
func fewSlots(p Provider) int {
	if pm, ok := p.(Parallelism); ok {
		if n := pm.Parallel(); n > 0 && n < DefaultParallel {
			return n
		}
	}
	return 0
}

// admit waits for one of the provider's slots, when it has few: every caller
// of this service shares them (the judge, the tool routing, the effort, the
// turn verdict, the message gate), so a local runtime gets no more requests
// at once than it serves, whoever sends them. The wait is bounded by the
// caller's timeout and comes before it: a request's own timeout starts when
// it is sent, not while it queues here (issue #288).
func (s *service) admit(ctx context.Context, providerID string, timeout time.Duration) (release func(), ok bool) {
	slots, few := s.slots[providerID]
	if !few {
		return func() {}, true
	}
	wait := time.NewTimer(timeout)
	defer wait.Stop()
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, true
	case <-ctx.Done():
		return nil, false
	case <-wait.C:
		return nil, false
	}
}
