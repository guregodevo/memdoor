package gateway

import "testing"

// The real reply, 2026-08-30 23:03: a full page of planning that ends by
// announcing the calls, with no <tool_call> anywhere. The turn ended silently
// and nothing was built. The think-marker rule could not see it — the model
// wrote its reasoning as plain prose, so no marker survived to signal it.
const announcedButMadeNoCall = `The user wants a program that fetches the latest news from Google and summarizes it. This is a new-program task.

Options:
1. Scrape Google News — Google's HTML is JS-heavy and the structure changes.
2. Use the Google News API — requires an API key.

Let me start:
1. Load the new-program skill
2. Check python3 version

These are independent, so I can run them in parallel.

I'll make the calls.


I'll start by loading the relevant skills and checking the environment in parallel.`

func TestAReplyThatAnnouncesCallsAndMakesNoneIsCaught(t *testing.T) {
	if !reasoningOnlyReply(announcedButMadeNoCall) {
		t.Error("a reply that ends by announcing the calls, having made none, was accepted as the answer")
	}
}

// The original signal still works: reasoning wrapped in think markers.
func TestThinkMarkerRepliesAreStillCaught(t *testing.T) {
	for _, s := range []string{
		"Let's start by reading the stats.go file first to see what's in it.\n</think>",
		"<think>I should read the file",
		"Then run `go run stats.go` with bash.</think>",
	} {
		if !reasoningOnlyReply(s) {
			t.Errorf("think-marker reply not caught: %q", s)
		}
	}
}

// A finished answer must NOT be retried: the work is done, and retrying costs a
// whole inference and replaces a good answer with a second guess.
func TestAFinishedAnswerIsNotRetried(t *testing.T) {
	for _, s := range []string{
		"Done — hello.go is written and prints Hello. Run it with `go run hello.go`.",
		"The build passes and all 66 packages are green.",
		"I added the helper and verified it compiles. Let me know if you want it widened.",
		"news.py now fetches the RSS feed and prints a summary; I ran it and it works.",
		"",
		"The file already contained that function, so nothing needed changing.",
	} {
		if reasoningOnlyReply(s) {
			t.Errorf("a finished answer would be thrown away and retried: %q", s)
		}
	}
}

// The tail restriction earns its keep here: a long answer that NARRATES the
// work ("I'll start with the RSS feed…") and then reports it finished. Scanning
// the whole reply flags the narration, throws a good answer away, and pays for
// a second inference that can only be worse — the work is already done.
func TestALongAnswerThatNarratesThenFinishesIsNotRetried(t *testing.T) {
	reply := "I'll start with the RSS feed, since it needs no API key.\n\n" +
		"Let me check what Python is available first, then write the fetcher.\n\n" +
		"First, I fetched https://news.google.com/rss and parsed the items with\n" +
		"xml.etree — no third-party packages, so it runs anywhere. Then I scored\n" +
		"sentences by word frequency for the extractive summary, because there is\n" +
		"no model available offline and a frequency ranking is honest about what\n" +
		"it is doing.\n\n" +
		"Done: news.py fetches the feed and prints a five-item digest. I ran it and\n" +
		"it printed today's headlines with a two-sentence summary under each."

	if len(reply) < 400 {
		t.Fatalf("the fixture is only %d chars — too short to test the tail rule", len(reply))
	}
	if reasoningOnlyReply(reply) {
		t.Error("a finished answer was flagged because of its narration, and would be thrown away")
	}
}
