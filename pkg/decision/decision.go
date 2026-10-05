// Package decision is the decision-model slot: typed questions about one
// piece of state, answered with probabilities by whichever provider the
// workspace configured — a hosted System One model (Jev) or another
// decision provider — or not at all.
//
// Shapes follow OpenClaw's decision-model API so plugins and habits transfer:
// a request is a state plus a map of questions keyed by the caller's ids; a
// question is a choice (labels), a score (ordered rubric) or a boolean; a
// result is either "ok" with one answer per id or "unavailable" with a reason.
// There is NO fallback to a chat model anywhere in this package: an
// unavailable result is a fact for the consumer to skip, defer or handle with
// its own logic, never a negative answer.
package decision

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Kind is a question's answer shape. Parse at the boundary with ParseKind.
type Kind string

const (
	KindChoice  Kind = "choice"  // one of N labelled options; answer is a label plus a distribution
	KindScore   Kind = "score"   // ordered rubric levels; answer is a zero-based, probability-weighted position
	KindBoolean Kind = "boolean" // answer is P(true)
)

// ParseKind accepts the OpenClaw names and the System One name for a boolean.
func ParseKind(s string) (Kind, error) {
	switch k := strings.ToLower(strings.TrimSpace(s)); k {
	case "choice":
		return KindChoice, nil
	case "score":
		return KindScore, nil
	case "boolean", "bool", "noul", "yesno":
		return KindBoolean, nil
	}
	return "", fmt.Errorf("unknown question kind %q (choice|score|boolean)", s)
}

// Limits mirror the System One API so a request valid here is valid there.
const (
	MaxChoiceOptions = 255
	MaxScoreLevels   = 10
	MinOptions       = 2
	// DefaultTimeout is the most a consumer waits for a provider; OpenClaw's
	// native decisions enforce the same 30 s ceiling.
	DefaultTimeout = 30 * time.Second
)

// Option is one labelled alternative of a choice, or one rubric level of a
// score (Label is the level's short name, Description its anchor text).
type Option struct {
	Label       string
	Description string
}

// Question is one typed question. Build it with Choice / Score / Boolean.
type Question struct {
	Kind         Kind
	Instructions string
	Options      []Option // choice: in display order; score: levels low -> high
	TrueText     string   // boolean only, optional: what "true" means
	FalseText    string   // boolean only, optional: what "false" means
}

// Choice asks for one of 2..255 distinct labelled options.
func Choice(instructions string, options []Option) (Question, error) {
	q := Question{Kind: KindChoice, Instructions: instructions, Options: options}
	return q, q.Validate()
}

// Labels is a convenience for choices whose options need no description.
func Labels(labels ...string) []Option {
	out := make([]Option, len(labels))
	for i, l := range labels {
		out[i] = Option{Label: l}
	}
	return out
}

// Score asks for a position on 2..10 ordered rubric levels, low to high.
func Score(instructions string, levels []string) (Question, error) {
	opts := make([]Option, len(levels))
	for i, l := range levels {
		opts[i] = Option{Label: fmt.Sprintf("%d", i), Description: l}
	}
	q := Question{Kind: KindScore, Instructions: instructions, Options: opts}
	return q, q.Validate()
}

// Boolean asks a yes/no question; trueText / falseText may be empty.
func Boolean(instructions, trueText, falseText string) (Question, error) {
	q := Question{Kind: KindBoolean, Instructions: instructions, TrueText: trueText, FalseText: falseText}
	return q, q.Validate()
}

// Labels of the question's options, in order.
func (q Question) LabelList() []string {
	out := make([]string, len(q.Options))
	for i, o := range q.Options {
		out[i] = o.Label
	}
	return out
}

// Validate is the single rule set every provider relies on.
func (q Question) Validate() error {
	if _, err := ParseKind(string(q.Kind)); err != nil {
		return err
	}
	if strings.TrimSpace(q.Instructions) == "" {
		return fmt.Errorf("question has no instructions")
	}
	switch q.Kind {
	case KindBoolean:
		if len(q.Options) != 0 {
			return fmt.Errorf("boolean question %q takes no options", q.Instructions)
		}
		return nil
	case KindScore:
		if n := len(q.Options); n < MinOptions || n > MaxScoreLevels {
			return fmt.Errorf("score %q needs %d..%d levels, got %d", q.Instructions, MinOptions, MaxScoreLevels, n)
		}
		for _, o := range q.Options {
			if strings.TrimSpace(o.Description) == "" {
				return fmt.Errorf("score %q has an empty level", q.Instructions)
			}
		}
		return nil
	}
	if n := len(q.Options); n < MinOptions || n > MaxChoiceOptions {
		return fmt.Errorf("choice %q needs %d..%d options, got %d", q.Instructions, MinOptions, MaxChoiceOptions, n)
	}
	seen := map[string]bool{}
	for _, o := range q.Options {
		if strings.TrimSpace(o.Label) == "" {
			return fmt.Errorf("choice %q has an empty option label", q.Instructions)
		}
		if seen[o.Label] {
			return fmt.Errorf("choice %q repeats option %q", q.Instructions, o.Label)
		}
		seen[o.Label] = true
	}
	return nil
}

// Request is one state and the questions to answer about it.
type Request struct {
	State     string
	Questions map[string]Question // keyed by the caller's ids; answers come back under the same keys
}

// Validate refuses what no provider could answer.
func (r Request) Validate() error {
	if strings.TrimSpace(r.State) == "" {
		return fmt.Errorf("state is empty")
	}
	if len(r.Questions) == 0 {
		return fmt.Errorf("no questions")
	}
	for id, q := range r.Questions {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("a question has an empty id")
		}
		if err := q.Validate(); err != nil {
			return fmt.Errorf("question %q: %w", id, err)
		}
	}
	return nil
}

// Options qualify one evaluation: who asks (for per-agent model selection),
// why (for logs), and how long the consumer can wait.
type Options struct {
	AgentID string
	Purpose string
	Timeout time.Duration
}

// Answer is one question's result; the fields used depend on Kind.
type Answer struct {
	Kind Kind
	// KindChoice
	Choice        string
	Probabilities map[string]float64 // by label
	// KindScore: zero-based rubric position, probability-weighted, and the
	// distribution over levels in rubric order
	Score              float64
	ScoreProbabilities []float64
	// KindBoolean
	ProbabilityTrue float64
	// Confidence is the probability of the top option (choice / score).
	Confidence float64
}

// Status of a Result.
type Status string

const (
	StatusOK          Status = "ok"
	StatusUnavailable Status = "unavailable"
)

// Reason says why a result is unavailable. The first five are OpenClaw's;
// provider-error covers a provider that answered wrongly or not at all.
type Reason string

const (
	ReasonDisabled         Reason = "disabled"          // the workspace switched decisions off
	ReasonNotConfigured    Reason = "not-configured"    // no decision model chosen, or an unknown one
	ReasonUnsupportedInput Reason = "unsupported-input" // the provider cannot take this request
	ReasonOverloaded       Reason = "overloaded"        // rate limit or capacity
	ReasonDeadline         Reason = "deadline"          // the consumer's timeout passed
	ReasonProviderError    Reason = "provider-error"    // anything else the provider did wrong
)

// Result is what every consumer gets: answers, or a reason there are none.
type Result struct {
	Status   Status
	Reason   Reason
	Detail   string
	Answers  map[string]Answer
	Provider string
	Model    string
}

// OK reports whether Answers may be read.
func (r Result) OK() bool { return r.Status == StatusOK }

// Unavailable builds a refused result.
func Unavailable(reason Reason, detail string) Result {
	return Result{Status: StatusUnavailable, Reason: reason, Detail: detail}
}

// Provider answers requests for one backend. Ready must be synchronous and
// network-free (credentials present, model loadable); Evaluate returns an
// unavailable Result for refusals it can classify and an error only for
// failures it cannot.
type Provider interface {
	ID() string
	Ready() bool
	Evaluate(ctx context.Context, model string, req Request) (Result, error)
}

// ModelRef names a configured decision model: "<provider>" or
// "<provider>/<model>", e.g. "systemone/jev-latest".
type ModelRef struct {
	Provider string
	Model    string
}

// ParseModelRef splits a decision model reference. "" means not configured;
// "off" / "none" / "disabled" mean deliberately disabled.
func ParseModelRef(s string) (ref ModelRef, disabled bool, err error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "":
		return ModelRef{}, false, nil
	case "off", "none", "disabled":
		return ModelRef{}, true, nil
	}
	prov, model, _ := strings.Cut(s, "/")
	prov = strings.ToLower(strings.TrimSpace(prov))
	if prov == "" {
		return ModelRef{}, false, fmt.Errorf("decision model %q has no provider", s)
	}
	return ModelRef{Provider: prov, Model: strings.TrimSpace(model)}, false, nil
}

// String renders the reference the way the setting spells it.
func (m ModelRef) String() string {
	if m.Model == "" {
		return m.Provider
	}
	return m.Provider + "/" + m.Model
}
