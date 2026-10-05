package cmd

import (
	"strings"
	"testing"
	"time"
)

// A real payload from GET /api/channels/{id}/messages, kept verbatim. The
// transcript parser reads nested and prefixed fields (content.text, an
// author_id that carries the role), and getting either wrong fails silently —
// json leaves unmatched fields zero, so a resumed session just opens blank.
const channelMessagesPayload = `{"messages":[
  {"id":"2259","channel_id":"8488c86b","author_id":"human:975c6438-4c44-481f-8d90-8b5ee00dda46",
   "author_name":"greg","content":{"text":"@coder remember the number 4291. just say ok","mentions":["agent:coder"]},
   "created_at":"2026-08-20T10:24:13+02:00"},
  {"id":"2260","channel_id":"8488c86b","author_id":"agent:coder",
   "author_name":"coder","content":{"text":"OK"},
   "created_at":"2026-08-20T10:25:11+02:00"}
]}`

func TestParseTranscript(t *testing.T) {
	msgs := parseTranscript([]byte(channelMessagesPayload))
	if len(msgs) != 2 {
		t.Fatalf("parsed %d messages, want 2 — the payload shape changed and resume opens blank", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("first message role = %q, want user (author_id carries it)", msgs[0].Role)
	}
	// The mention prefix is how the turn is addressed, not what was typed.
	if msgs[0].Content != "remember the number 4291. just say ok" {
		t.Errorf("first message = %q; the @agent prefix should be stripped", msgs[0].Content)
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "OK" {
		t.Errorf("second message = %q/%q, want assistant/OK", msgs[1].Role, msgs[1].Content)
	}
	// Oldest first: a transcript reads down the screen.
	if !strings.Contains(msgs[0].Content, "4291") {
		t.Error("messages are not oldest-first")
	}
}

func TestParseTranscriptSurvivesJunk(t *testing.T) {
	for _, in := range []string{"", "{}", `{"messages":[]}`, `{"messages":[{"author_id":"agent:x","content":{}}]}`, "not json"} {
		if got := parseTranscript([]byte(in)); len(got) != 0 {
			t.Errorf("parseTranscript(%q) returned %d messages, want none", in, len(got))
		}
	}
}

// The conversation index is what `memdoor resume` reads. A turn must name a new
// conversation, and later turns must keep it — not duplicate it.
func TestRecordSessionAccumulates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	rec := sessionRecord{
		ChannelID:   "chan-1",
		ChannelName: "tui-0820-102350",
		Title:       "fix the median bug",
		Workspace:   "acme",
		Dir:         "/tmp/project",
	}
	recordSession(rec)

	// A later turn carries a different first line; the title must not change.
	second := rec
	second.Title = "and now run the tests"
	recordSession(second)

	all := sessionsFor("")
	if len(all) != 1 {
		t.Fatalf("index holds %d conversations, want 1 — turns are being recorded as separate sessions", len(all))
	}
	got := all[0]
	if got.Title != "fix the median bug" {
		t.Errorf("title = %q; a conversation keeps the name of what it started as", got.Title)
	}
	if got.Turns != 2 {
		t.Errorf("turns = %d, want 2", got.Turns)
	}
	if got.StartedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Error("timestamps missing — the list sorts and displays by them")
	}
}

// Resume defaults to the directory you are in; --all is what widens it.
func TestSessionsForFiltersByDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	recordSession(sessionRecord{ChannelID: "a", Title: "here", Dir: "/tmp/here"})
	recordSession(sessionRecord{ChannelID: "b", Title: "elsewhere", Dir: "/tmp/elsewhere"})

	here := sessionsFor("/tmp/here")
	if len(here) != 1 || here[0].Title != "here" {
		t.Fatalf("directory filter returned %d rows, want just this directory's", len(here))
	}
	if len(sessionsFor("")) != 2 {
		t.Error("unfiltered listing should hold both")
	}
}

// Most recent first: resume --last must take the conversation you were just in.
func TestSessionsForOrdersByRecency(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	recordSession(sessionRecord{ChannelID: "old", Title: "older"})
	time.Sleep(5 * time.Millisecond)
	recordSession(sessionRecord{ChannelID: "new", Title: "newer"})

	all := sessionsFor("")
	if len(all) != 2 || all[0].Title != "newer" {
		t.Fatalf("most recent is %q, want \"newer\"", all[0].Title)
	}
}

func TestShortSessionKey(t *testing.T) {
	cases := map[string]string{
		"workspace:hackernews:channel:8488c86b-5f13-48de-a4e9-233c11d561e9": "hackernews · channel 8488c86b",
		"agent:researcher:cron:poll-1":                                      "agent:researcher:cron:poll-1",
		"":                                                                  "",
	}
	for in, want := range cases {
		if got := shortSessionKey(in); got != want {
			t.Errorf("shortSessionKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindSessionByID(t *testing.T) {
	recs := []sessionRecord{
		{ChannelID: "8488c86b-5f13-48de-a4e9-233c11d561e9", Title: "one"},
		{ChannelID: "8488ffff-0000-4000-8000-000000000000", Title: "two"},
		{ChannelID: "b0e37dd0-7404-45b6-bc91-c6c03389d971", Title: "three"},
	}
	if r, err := findSessionByID(recs, "b0e37dd0"); err != nil || r.Title != "three" {
		t.Fatalf("8-char prefix: got %q, %v", r.Title, err)
	}
	if r, err := findSessionByID(recs, recs[0].ChannelID); err != nil || r.Title != "one" {
		t.Fatalf("full id: got %q, %v", r.Title, err)
	}
	if _, err := findSessionByID(recs, "8488"); err == nil || !strings.Contains(err.Error(), "2 conversations") {
		t.Fatalf("ambiguous prefix must say so, got %v", err)
	}
	if _, err := findSessionByID(recs, "8488c"); err != nil {
		t.Fatalf("a prefix that singles one out resolves, got %v", err)
	}
	if _, err := findSessionByID(recs, "ffffffff"); err == nil || !strings.Contains(err.Error(), "no conversation") {
		t.Fatalf("unknown id must say so, got %v", err)
	}
	if _, err := findSessionByID(recs, "b0"); err == nil {
		t.Fatal("a 2-character prefix is refused")
	}
	if got := shortID(recs[0].ChannelID); got != "8488c86b" {
		t.Fatalf("shortID = %q", got)
	}
}
