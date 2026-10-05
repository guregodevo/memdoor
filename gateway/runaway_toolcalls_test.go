package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// Every limit in the tool loop bounds ITERATIONS — maxAPICalls counts trips
// round the outer loop, and `stuck` is only read after the inner loop over a
// message's content blocks has finished. Nothing bounded the number of
// tool_use blocks inside ONE assistant message.
//
// Measured 2026-08-30: asked to fix a file the harness had
// misnamed out of existence, the model emitted a SINGLE message carrying 583
// read_file blocks. The loop-breaker set `stuck` on the 4th failure and 579
// more executed anyway. maxAPICalls never applied — the entire burn was one
// iteration, all of it paid for. Healthy turns in the same session
// carried 2 and 8 blocks.
func TestAbandonRestStopsAtTheLoopBreaker(t *testing.T) {
	// Block 5 of a reply whose breaker tripped at block 4.
	why, skip := abandonRest(true, 5, 24)
	if !skip {
		t.Fatal("once the loop-breaker has tripped, the REST of that same reply must not run — " +
			"this is the 579 calls that executed after `stuck` was set")
	}
	if !strings.Contains(strings.ToLower(why), "stop") {
		t.Errorf("the refusal must tell the model to stop, got: %s", why)
	}
}

// The budget is the backstop for a runaway whose calls all SUCCEED: `stuck`
// never trips, so nothing else would stop it.
func TestAbandonRestCapsOneReplysToolCalls(t *testing.T) {
	if _, skip := abandonRest(false, 24, 24); skip {
		t.Error("the 24th call is within a 24 budget and must run")
	}
	if _, skip := abandonRest(false, 25, 24); !skip {
		t.Error("the 25th call exceeds the budget and must be refused")
	}
	if _, skip := abandonRest(false, 583, 24); !skip {
		t.Error("the 583rd call — the measured case — must be refused")
	}
}

// A healthy reply must be untouched. Real turns from the same session carried
// 2 and 8 tool calls; a cap that clipped those would break normal work.
func TestAbandonRestLeavesHealthyRepliesAlone(t *testing.T) {
	for _, n := range []int{1, 2, 8, 12} {
		if _, skip := abandonRest(false, n, 24); skip {
			t.Errorf("call %d of a healthy reply must run", n)
		}
	}
}

// An abandoned block still has to be ANSWERED. A tool_use with no matching
// tool_result is a malformed conversation and the next inference rejects it,
// so skipping execution must not mean skipping the result.
func TestAbandonedCallStillProducesAnAnsweredToolResult(t *testing.T) {
	why, skip := abandonRest(true, 9, 24)
	if !skip {
		t.Fatal("precondition")
	}
	blk := llm.NewToolResultBlock("toolu_42", why, true)
	if blk.OfToolResult == nil {
		t.Fatal("an abandoned tool_use must still yield a tool_result block")
	}
	if blk.OfToolResult.ToolUseID != "toolu_42" {
		t.Errorf("the result must reference the tool_use it answers, got %q", blk.OfToolResult.ToolUseID)
	}
	if !blk.OfToolResult.IsError {
		t.Error("a refusal must carry IsError, or the loop counts it as progress")
	}
	if b, _ := json.Marshal(why); len(b) > 300 {
		t.Errorf("the refusal should be short — it is repeated once per abandoned call: %d bytes", len(b))
	}
}

// A reply that is nothing but the model's own reasoning is a turn that did
// nothing. Measured 2026-08-30 across a four-task suite: two of
// four ended this way, the whole stored reply being an announcement plus an
// orphan </think>, after 393 seconds of generation.
//
// The signal is the surviving think marker: replyContent strips <think> blocks
// so the ANSWER reaches the transcript, and keeps the raw text only when
// stripping would leave nothing — so a marker that survived means there was no
// answer beside the reasoning.
func TestReasoningOnlyReplyIsDetected(t *testing.T) {
	if !reasoningOnlyReply("Let's start by reading the stats.go file first to see what's in it.\n</think>") {
		t.Error("the measured failure — an announcement plus an orphan closer — must be detected")
	}
	if !reasoningOnlyReply("<think>I should read the file") {
		t.Error("a reply cut off mid-reasoning must be detected")
	}
}

// And a real answer must NOT be, or every ordinary turn gets retried at double
// the cost.
func TestRealAnswersAreNotTreatedAsReasoningOnly(t *testing.T) {
	for _, ok := range []string{
		"Fixed. Added a sync.Mutex to Tally and locked both Add and Get.",
		"median: 2.5\np50: 2",
		"",
		"I read stats.go; the Median function does not sort its input.",
	} {
		if reasoningOnlyReply(ok) {
			t.Errorf("a real answer must not be retried as reasoning-only: %q", ok)
		}
	}
}

// The retry must fire even when tools ALREADY RAN this turn. The first version
// required ToolsExecuted == 0 and so missed the only case that actually occurs:
// measured 2026-08-30, the model called read_file successfully, then emitted
// 9,234 characters of reasoning that ended "Then run `go run stats.go` with
// bash.</think>" and stopped — never issuing the call it had just planned. One
// tool had run, the guard skipped, and the turn was abandoned after 393s.
func TestReasoningOnlyRetryDoesNotDependOnToolsAlreadyRun(t *testing.T) {
	plan := "1. Median is wrong for even n...\nThen run `go run stats.go` with bash.\n</think>"
	if !reasoningOnlyReply(plan) {
		t.Fatal("a planning block that closes </think> without acting must be detected " +
			"whether or not a tool ran earlier in the turn")
	}
}
