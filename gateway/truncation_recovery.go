package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"memdoor/pkg/llm"
)

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

// keepOfCutReply is what a reply cut at the output cap leaves that a provider
// accepts: its non-empty text and the tool calls whose arguments arrived whole,
// each with its result. A call cut mid-arguments is dropped with its result:
// its input is not JSON, and a request carrying it is refused. The reply is
// nil when nothing is left.
func keepOfCutReply(reply llm.MessageParam, results []llm.ContentBlockParamUnion) (*llm.MessageParam, []llm.ContentBlockParamUnion) {
	kept := map[string]bool{}
	var blocks []llm.ContentBlockParamUnion
	for _, b := range reply.Content {
		switch {
		case b.OfText != nil && strings.TrimSpace(b.OfText.Text) == "":
		case b.OfToolUse != nil && !json.Valid(b.OfToolUse.Input):
		case b.OfToolUse != nil:
			kept[b.OfToolUse.ID] = true
			blocks = append(blocks, b)
		default:
			blocks = append(blocks, b)
		}
	}
	var keptResults, rest []llm.ContentBlockParamUnion
	for _, r := range results {
		switch {
		case r.OfToolResult == nil:
			rest = append(rest, r)
		case kept[r.OfToolResult.ToolUseID]:
			keptResults = append(keptResults, r)
		}
	}
	if len(blocks) == 0 {
		return nil, rest
	}
	reply.Content = blocks
	return &reply, append(keptResults, rest...)
}

// resumeAfterCut replaces the cut reply that ends conv with what a provider
// accepts, then asks the model to go on: the kept calls' results come first in
// the next user message, as every provider requires, then the prompt.
func resumeAfterCut(conv []llm.MessageParam, results []llm.ContentBlockParamUnion, prompt string) []llm.MessageParam {
	if len(conv) == 0 || conv[len(conv)-1].Role != llm.MessageParamRoleAssistant {
		return append(conv, llm.NewUserMessage(llm.NewTextBlock(prompt)))
	}
	reply, next := keepOfCutReply(conv[len(conv)-1], results)
	conv = conv[:len(conv)-1]
	if reply != nil {
		conv = append(conv, *reply)
	}
	return append(conv, llm.NewUserMessage(append(next, llm.NewTextBlock(prompt))...))
}

// closeCutReply ends a turn that gave up on a cut reply with a conversation
// the next turn can send: the reply's text stays, its tool calls go (the turn
// ends here, so their results are never sent), and an empty reply is dropped.
func closeCutReply(conv []llm.MessageParam) []llm.MessageParam {
	if len(conv) == 0 || conv[len(conv)-1].Role != llm.MessageParamRoleAssistant {
		return conv
	}
	var text []llm.ContentBlockParamUnion
	for _, b := range conv[len(conv)-1].Content {
		if b.OfText != nil && strings.TrimSpace(b.OfText.Text) != "" {
			text = append(text, b)
		}
	}
	conv = conv[:len(conv)-1]
	if len(text) == 0 {
		return conv
	}
	return append(conv, llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: text})
}
