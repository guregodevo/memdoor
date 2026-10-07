package gateway

import (
	"context"
	"strings"

	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
)

// A blank turn is never the answer.
//
// The brain's reply came back with no text and no tool call twice in a row
// — the second time on a retry — and the turn ended with nothing in the
// window and an empty assistant message in the session (2026-09-19, Sara's
// brief after a tool error: the reply was bare JSON the parser scrubbed).
// The person saw nothing at all, and the empty turn stayed in the history
// for the next reply to imitate.

// emptyReplyRetryWanted says whether a reply with no text and no tool call
// is retried: once, for any agent that works a codebase or a film. Whether
// a tool already ran this turn does not matter — the "continue" that
// followed the first blank had run none and got no retry.
func emptyReplyRetryWanted(text string, retried, codebase bool) bool {
	return strings.TrimSpace(text) == "" && !retried && codebase
}

// emptyReplyNotice is what the window shows when the retry came back empty
// too, and what the session keeps in place of the blank.
const emptyReplyNotice = "The brain answered nothing readable twice in a row, so nothing was done. " +
	"Ask again in one sentence — this happens after a tool error, and a fresh ask clears it."

// scrubbedNudge is what the retry adds when the blank was a reply the
// parser could not read: the same inference again produced the same
// unreadable call (the drone reel, 2026-09-19, twice), so the model is told.
const scrubbedNudge = "Your last reply was a tool call that could not be read — it had no \"name\", or its JSON was broken. " +
	"Write it again as ONE object, {\"name\": \"<tool>\", \"arguments\": {…}}, inside <tool_call></tool_call>."

// emptyRetryPrompt is the system prompt for the empty-step retry: unchanged
// for a bare blank, with the nudge when the blank was scrubbed.
func emptyRetryPrompt(extra string, stop llm.StopReason) string {
	if stop != llm.StopReasonScrubbed {
		return extra
	}
	if extra == "" {
		return scrubbedNudge
	}
	return extra + "\n\n" + scrubbedNudge
}

// nextRungAfterEmpty is the rung to try once the answering rung has
// answered nothing twice: the next one of the agent's ladder, as a tier
// index and its model. Nothing when the person pinned a model (their
// choice stands), when no engine was chosen at start, or when the ladder
// has no further rung that is a different model.
func nextRungAfterEmpty(ctx context.Context, agent string) (int, string) {
	if providers.ModelFromContext(ctx) != "" {
		return 0, ""
	}
	re := providers.ActiveRemoteEngine()
	if re == nil {
		return 0, ""
	}
	cur := providers.TierFromContext(ctx)
	next := re.ModelForTier(agent, cur+1)
	if next == "" || next == re.ModelForTier(agent, cur) {
		return 0, ""
	}
	return cur + 1, next
}
