package decision

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service is the one entry point consumers call. It resolves the configured
// model, picks the provider, bounds the wait, and never substitutes a chat
// completion when the provider is missing.
type Service interface {
	Evaluate(ctx context.Context, req Request, opts Options) Result
}

// ModelResolver returns the decision model reference in force for an agent
// ("" when nothing is configured). Duck-typed so the package owns no
// repository import.
type ModelResolver func(agentID string) string

// Providers lists what a Service can route to, for status displays.
type Providers interface {
	Providers() []Provider
}

type service struct {
	resolve   ModelResolver
	providers map[string]Provider
	order     []Provider
}

// NewService builds a Service over the given providers. Fail-fast: a nil
// resolver or a duplicate provider id is a wiring bug, not a runtime state.
func NewService(resolve ModelResolver, providers ...Provider) (Service, error) {
	if resolve == nil {
		return nil, errors.New("decision: nil model resolver")
	}
	s := &service{resolve: resolve, providers: map[string]Provider{}}
	for _, p := range providers {
		if p == nil || p.ID() == "" {
			return nil, errors.New("decision: provider without an id")
		}
		if _, dup := s.providers[p.ID()]; dup {
			return nil, fmt.Errorf("decision: duplicate provider %q", p.ID())
		}
		s.providers[p.ID()] = p
		s.order = append(s.order, p)
	}
	return s, nil
}

func (s *service) Providers() []Provider { return append([]Provider(nil), s.order...) }

// Availability reports, without a network call, whether an Evaluate for
// agentID would reach a ready provider.
type Availability interface {
	Available(agentID string) bool
}

func (s *service) Available(agentID string) bool {
	ref, disabled, err := ParseModelRef(s.resolve(agentID))
	if err != nil || disabled || ref.Provider == "" {
		return false
	}
	p, ok := s.providers[ref.Provider]
	return ok && p.Ready()
}

func (s *service) Evaluate(ctx context.Context, req Request, opts Options) Result {
	if err := req.Validate(); err != nil {
		return Unavailable(ReasonUnsupportedInput, err.Error())
	}
	ref, disabled, err := ParseModelRef(s.resolve(opts.AgentID))
	switch {
	case err != nil:
		return Unavailable(ReasonNotConfigured, err.Error())
	case disabled:
		return Unavailable(ReasonDisabled, "decisions are off")
	case ref.Provider == "":
		return Unavailable(ReasonNotConfigured, "no decision model: it runs on an OpenRouter key (OPEN_ROUTER_API_KEY) or a decision key (MEMDOOR_SYSTEMONE_API_KEY)")
	}
	p, ok := s.providers[ref.Provider]
	if !ok {
		return Unavailable(ReasonNotConfigured, fmt.Sprintf("decision provider %q is not known", ref))
	}
	if !p.Ready() {
		return Unavailable(ReasonNotConfigured, fmt.Sprintf("provider %q is not ready", p.ID()))
	}
	timeout := opts.Timeout
	if timeout <= 0 || timeout > DefaultTimeout {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	deadline := time.Now().Add(timeout)
	wait := overloadRetryWait
	for attempt := 1; ; attempt++ {
		res, err := p.Evaluate(ctx, ref.Model, req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return Unavailable(ReasonDeadline, err.Error())
			}
			return Unavailable(ReasonProviderError, err.Error())
		}
		if res.Provider == "" {
			res.Provider = p.ID()
		}
		if !res.OK() {
			// Overloaded is capacity (429 / 503 / 529), not a verdict: back
			// off and ask again, a few times at most and only while the
			// caller's budget fits the wait. Live 2026-09-28: one 503 upstream
			// reset turned a whole turn's judged tool output unjudged. A
			// lasting overload still answers within ~1s instead of hammering
			// a rate-limited upstream until the deadline. Every other reason
			// answers the same on a retry and stands.
			if res.Reason != ReasonOverloaded || attempt >= overloadMaxAttempts || !time.Now().Add(wait).Before(deadline) {
				return res
			}
			select {
			case <-ctx.Done():
				return res
			case <-time.After(wait):
			}
			wait *= 2
			continue
		}
		for id := range req.Questions {
			if _, has := res.Answers[id]; !has {
				return Unavailable(ReasonProviderError, fmt.Sprintf("provider %q returned no answer for %q", p.ID(), id))
			}
		}
		return res
	}
}

// overloadRetryWait is the first wait after an overloaded answer; each
// further wait doubles, for at most overloadMaxAttempts asks in all. Short
// enough that a single upstream reset is invisible to the consumer, and
// bounded by whatever timeout the caller set.
const (
	overloadRetryWait   = 400 * time.Millisecond
	overloadMaxAttempts = 3
)
