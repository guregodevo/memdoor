package gateway

import "testing"

// THE TEXT APPENDED AFTER THE ROUNDS REACHES THE WINDOW. What streamed is
// not sent twice; what was added after is; a rewritten final is shown
// whole. Mutation check: return "" when streamed is a prefix and the
// notice vanishes again.
func TestTheTextAppendedAfterTheRoundsReachesTheWindow(t *testing.T) {
	if got, isTail := unstreamedTail("Reading in chunks:\nDone.\n\n(note: nothing was written)", "Reading in chunks:\nDone."); got != "(note: nothing was written)" || !isTail {
		t.Fatalf("only the appended note is new: %q", got)
	}
	if got, _ := unstreamedTail("Reading in chunks:", "Reading in chunks:"); got != "" {
		t.Fatalf("what streamed is not sent again: %q", got)
	}
	if got, isTail := unstreamedTail(emptyReplyNotice, ""); got != emptyReplyNotice || isTail {
		t.Fatalf("a turn that streamed nothing shows its notice: %q", got)
	}
	if got, isTail := unstreamedTail("The repaired answer.", "The draft answer."); got != "The repaired answer." || isTail {
		t.Fatalf("a rewritten final is shown whole: %q", got)
	}
}
