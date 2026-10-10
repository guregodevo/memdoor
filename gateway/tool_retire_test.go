package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// Measured 2026-08-29, one live coder turn: a tool that needs a workspace
// failed FIVE times with "no workspace available". Five of eleven tool calls —
// a 45% failure rate from one tool that could never have worked in that
// context.
//
// Neither existing breaker caught it. repeatedFail is keyed on name AND input,
// so one changed argument resets it; consecutiveFail resets on any successful
// call, and the failures were interleaved with successful read_file and context
// calls. A structurally-unavailable tool was therefore retryable forever.
func TestRetiredToolRefusesWithoutRunning(t *testing.T) {
	info, res := retiredToolResult("legacy_tool", "legacy_tool", `{"workspace":"x"}`, "toolu_9", 3, "no workspace available")

	if info.Name != "legacy_tool" {
		t.Errorf("name lost: %q", info.Name)
	}
	if info.Error == "" {
		t.Fatal("a refusal must be an error, or the loop counts it as progress")
	}
	// The message must tell the model to STOP. A bare repeat of the underlying
	// error invites exactly the retry this exists to prevent.
	low := strings.ToLower(info.Error)
	if !strings.Contains(low, "not being run again") {
		t.Errorf("the refusal must instruct the model to stop, got: %s", info.Error)
	}
	if !strings.Contains(low, "same line") {
		t.Errorf("the refusal must say that repeating it changes nothing, got: %s", info.Error)
	}
	// AND IT MUST SAY WHAT WENT WRONG. "Do not call it again" was the whole
	// message, and the same call came back four more times (2026-09-16): the
	// model had nothing to act on. The last real reason is the actionable
	// half — a wrong file name can be fixed, a missing workspace cannot.
	if !strings.Contains(low, "no workspace available") {
		t.Errorf("the refusal must carry the last real reason, got: %s", info.Error)
	}
	if !strings.Contains(info.Error, "legacy_tool") {
		t.Errorf("the refusal must name the tool so the model knows which to drop: %s", info.Error)
	}
	// And it must be flagged as an error result, not a normal one.
	if res.OfToolResult == nil || !res.OfToolResult.IsError {
		t.Error("the tool result must carry IsError, or it reads as a successful call")
	}
}

// DurationMs must stay zero: the tool did not run, and a refusal that reports
// wall time would inflate the very cost metric this change exists to reduce.
func TestRetiredToolCostsNothing(t *testing.T) {
	info, _ := retiredToolResult("bash", "bash", "{}", "id", 5, "")
	if info.DurationMs != 0 {
		t.Errorf("a refused call runs nothing and must record 0ms, got %d", info.DurationMs)
	}
	if info.Output != "" {
		t.Errorf("a refused call produces no output, got %q", info.Output)
	}
}

// A REFUSAL IS NOT A NEW FAILURE.
//
// The count climbed with every repeat — "failed 5 times", then 6 — as if the
// tool were breaking again, when nothing had run at all (live, 2026-09-16).
// The number a refusal reports must be the number of real failures.
func TestRefusingATooldoesNotCountAsFailingAgain(t *testing.T) {
	l := toolFailureLedger{}
	for i := 0; i < 3; i++ {
		l.failed("clip_inspect", "no such file: pourquoi-les-francais.mp4", "")
	}
	if !l.retired("clip_inspect", 3, "") {
		t.Fatal("three real failures must retire the tool")
	}
	before := l.count("clip_inspect")
	// Four refusals later, with nothing run, the count must not have moved.
	for i := 0; i < 4; i++ {
		_, _ = retiredToolResult("clip_inspect", "clip_inspect", "{}", "id", l.count("clip_inspect"), l.reason("clip_inspect"))
	}
	if l.count("clip_inspect") != before {
		t.Fatalf("refusals inflated the count: %d, was %d", l.count("clip_inspect"), before)
	}
	if l.reason("clip_inspect") != "no such file: pourquoi-les-francais.mp4" {
		t.Fatalf("the last real reason was lost: %q", l.reason("clip_inspect"))
	}
	// A success clears both.
	l.succeeded("clip_inspect")
	if l.count("clip_inspect") != 0 || l.reason("clip_inspect") != "" {
		t.Fatal("a success must clear the count and the reason")
	}
}

// THE SAME WALL THREE TIMES, NOT THREE DIFFERENT WALLS.
//
// Retirement is for a tool that cannot work in this turn at all, and the
// evidence is the SAME answer coming back. Live 2026-09-17: clip_stitch
// said "no transcript for meli_1.mp4 — call transcribe first", the
// transcript was made, it said the same of meli_2, that was made too — and
// the third call, which would have worked, was refused. Each failure had
// been fixed before the next arrived.
func TestADifferentFailureIsProgressNotRepetition(t *testing.T) {
	l := toolFailureLedger{}
	l.failed("clip_stitch", "no transcript for meli_1.mp4", "")
	l.failed("clip_stitch", "no transcript for meli_2.mp4", "")
	l.failed("clip_stitch", "no transcript for meli_3.mp4", "")
	if l.retired("clip_stitch", 3, "") {
		t.Fatal("a tool that fixed each problem in turn was retired")
	}
	if l.count("clip_stitch") != 1 {
		t.Fatalf("the count should have reset with each new reason, got %d", l.count("clip_stitch"))
	}

	// THE SAME WALL still retires it — that is what the rule is for.
	stuck := toolFailureLedger{}
	for i := 0; i < 3; i++ {
		stuck.failed("legacy_tool", "no workspace available", "")
	}
	if !stuck.retired("legacy_tool", 3, "") {
		t.Fatal("three identical failures must still retire the tool")
	}
	if stuck.reason("legacy_tool") != "no workspace available" {
		t.Fatalf("reason: %q", stuck.reason("legacy_tool"))
	}

	// And a success still clears everything.
	stuck.succeeded("legacy_tool")
	if stuck.retired("legacy_tool", 3, "") || stuck.count("legacy_tool") != 0 {
		t.Fatal("a success must clear the count")
	}
}

// Three refusals on one file retire apply_patch for that file only: live
// 2026-09-30 they retired it for README.md too, and the model went on
// editing through bash scripts that no check covers.
func TestApplyPatchRetiresPerFile(t *testing.T) {
	patch := func(path string) []byte {
		b, _ := json.Marshal(map[string]string{"input": "*** Begin Patch\n*** Update File: " + path + "\n@@\n-a\n+b\n*** End Patch"})
		return b
	}
	l := toolFailureLedger{}
	broken := failureKey("apply_patch", patch("tools/apply_patch.go"))
	for i := 0; i < 3; i++ {
		l.failed(broken, "patch would BREAK apply_patch.go", "")
	}
	if !l.retired(broken, 3, "") {
		t.Fatal("the failing file must be retired")
	}
	if other := failureKey("apply_patch", patch("README.md")); other == broken || l.retired(other, 3, "") {
		t.Fatalf("another file must still be editable (key %q)", other)
	}
	hashline, _ := json.Marshal(map[string]string{"input": "[tools/apply_patch.go#1A2B]\nreplace 3 \"abc\":\n+x"})
	if got := failureKey("apply_patch", hashline); got != broken {
		t.Fatalf("a line-anchored edit of the same file is the same wall: %q vs %q", got, broken)
	}
	if got := failureKey("bash", []byte(`{"command":"ls"}`)); got != "bash" {
		t.Fatalf("other tools count per tool: %q", got)
	}
	_, res := retiredToolResult("apply_patch", broken, "{}", "id", 3, "patch would BREAK")
	if msg := res.OfToolResult.Content[0].OfText.Text; !strings.Contains(msg, "on tools/apply_patch.go") || !strings.Contains(msg, "other files are not affected") {
		t.Fatalf("the refusal must name the file: %s", msg)
	}
}
