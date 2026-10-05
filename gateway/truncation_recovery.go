package gateway

import "fmt"

// RecoveryKind is what to do about a reply the model did not finish.
type RecoveryKind int

const (
	// RecoveryEscalate retries the SAME request at a larger output cap, on the
	// theory that the model simply needed more room. Once per turn.
	RecoveryEscalate RecoveryKind = iota
	// RecoveryResume keeps the truncated text and asks the model to pick up
	// mid-thought.
	RecoveryResume
	// RecoveryExhausted means stop and say so, rather than presenting a
	// half-written reply as an answer.
	RecoveryExhausted
)

// RecoveryStep is one decision from a TruncationRecovery.
type RecoveryStep struct {
	Kind RecoveryKind
	// MaxTokens is the escalated cap, set only for RecoveryEscalate.
	MaxTokens int64
	// Prompt is the resume instruction, set only for RecoveryResume.
	Prompt string
	// Attempt is which resume this is, 1-based, set only for RecoveryResume.
	Attempt int
}

// TruncationRecovery decides how a turn responds to a reply cut off at the
// output cap.
//
// It is an interface because the POLICY is the interesting part and the turn
// driver should not own it: the loop asks "what now?" and acts, while the
// sequence — escalate once, then resume a bounded number of times, then give up
// — is decided here and can be tested without running a single inference.
type TruncationRecovery interface {
	// Next returns what to do about the truncated reply just received, and
	// advances the policy. Calling it is what consumes an attempt.
	Next() RecoveryStep
}

// resumePrompt is the instruction handed back to a model that was cut off.
//
// Taken from Claude Code (query.ts:1223), whose wording is deliberate: no
// apology and no recap, because a model that re-explains itself spends the new
// budget on the same tokens it just lost — and "break remaining work into
// smaller pieces" is the only instruction that changes the outcome rather than
// repeating the attempt.
const resumePrompt = "Output token limit hit. Resume directly — no apology, no recap of " +
	"what you were doing. Pick up mid-thought if that is where the cut " +
	"happened. Break remaining work into smaller pieces."

// maxResumeAttempts bounds stage two. Claude Code uses 3
// (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT); a model that cannot finish in three
// resumes is not going to on the fourth, and each one is a full generation.
const maxResumeAttempts = 3

type truncationRecovery struct {
	escalatedCap int64
	escalated    bool
	resumes      int
}

// NewTruncationRecovery returns the policy for one turn.
//
// escalatedCap must exceed the turn's normal cap — retrying at the same limit
// would burn another full generation only to hit it again — so a bad value
// fails here rather than wasting a model call discovering it.
func NewTruncationRecovery(escalatedCap int64) (TruncationRecovery, error) {
	if escalatedCap <= 0 {
		return nil, fmt.Errorf("escalated cap must be positive, got %d", escalatedCap)
	}
	return &truncationRecovery{escalatedCap: escalatedCap}, nil
}

func (r *truncationRecovery) Next() RecoveryStep {
	if !r.escalated {
		r.escalated = true
		return RecoveryStep{Kind: RecoveryEscalate, MaxTokens: r.escalatedCap}
	}
	if r.resumes < maxResumeAttempts {
		r.resumes++
		return RecoveryStep{Kind: RecoveryResume, Prompt: resumePrompt, Attempt: r.resumes}
	}
	return RecoveryStep{Kind: RecoveryExhausted}
}
