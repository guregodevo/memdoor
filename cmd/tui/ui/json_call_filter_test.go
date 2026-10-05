package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A tool call written as bare JSON must not be SHOWN as text. The gateway
// parses it into a real frame, so the reader would see the raw object scroll
// past and then the frame for it — measured 2026-08-30.
func TestBareCallJSONIsHiddenWhileStreaming(t *testing.T) {
	var f jsonCallFilter
	var out string
	for _, chunk := range []string{
		"I'll load the skill.\n",
		`{"arguments": {"command": "new-prog`,
		`ram"}, "name": "skill"}`,
		"\nNow building.",
	} {
		out += f.feed(chunk)
	}
	out += f.flush()

	if strings.Contains(out, "skill") && strings.Contains(out, "arguments") {
		t.Errorf("the raw call must not be shown: %q", out)
	}
	if !strings.Contains(out, "I'll load the skill.") || !strings.Contains(out, "Now building.") {
		t.Errorf("the prose around it must survive: %q", out)
	}
}

// JSON the model is EXPLAINING must still appear — swallowing it would be worse
// than showing a call.
func TestOrdinaryJSONStillShows(t *testing.T) {
	var f jsonCallFilter
	out := f.feed(`The config is {"name": "app", "port": 8080} by default.`) + f.flush()
	if !strings.Contains(out, `"port": 8080`) {
		t.Errorf("ordinary JSON must be shown: %q", out)
	}
}

// An object that never closes must be released, not swallowed — a truncated
// reply would otherwise lose its tail entirely.
func TestUnclosedObjectIsReleased(t *testing.T) {
	var f jsonCallFilter
	out := f.feed(`text {"name": "bash", "arguments": {"command": "ls"`)
	out += f.flush()
	if !strings.Contains(out, "text ") {
		t.Errorf("prose lost: %q", out)
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("an unclosed object must be released rather than swallowed: %q", out)
	}
}

// A brace inside a string must not end the object early.
func TestBraceInStringDoesNotCloseEarly(t *testing.T) {
	var f jsonCallFilter
	out := f.feed(`{"name":"bash","arguments":{"command":"awk '{print $1}' f"}}after`) + f.flush()
	if strings.Contains(out, "awk") {
		t.Errorf("the call must be hidden whole: %q", out)
	}
	if !strings.Contains(out, "after") {
		t.Errorf("text after the call must survive: %q", out)
	}
}

// The filter is a VALUE field on Model and Bubble Tea copies the Model on
// every Update, so the filter is copied mid-object whenever a brace arrives in
// one delta and the rest in the next. Holding the candidate in a
// strings.Builder made that a hard panic — "illegal use of non-zero Builder
// copied by value" — which killed the TUI on the first streamed tool call.
func TestFilterSurvivesTheModelBeingCopiedMidObject(t *testing.T) {
	var m Model
	if got := m.jsonFilter.feed(`before {"na`); got != "before " {
		t.Fatalf("first delta returned %q, want %q", got, "before ")
	}

	// Exactly what Update does: the model is passed and returned by value.
	cp := m
	got := cp.jsonFilter.feed(`me":"bash","arguments":{"command":"ls"}} after`)
	if strings.Contains(got, "bash") {
		t.Errorf("the held call leaked into the display after a copy: %q", got)
	}
	if !strings.Contains(got, "after") {
		t.Errorf("text following the call was lost: %q", got)
	}
}

// The same crash, through the real path: a streaming delta carrying a brace.
func TestStreamingADeltaWithABraceDoesNotPanic(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	for _, delta := range []string{"here goes ", `{"name":`, `"bash","arg`, `uments":{}}`, " done"} {
		m, _ = m.Update(assistantStreamingMsg{content: delta})
	}
	view := m.(Model).renderMessages()
	if strings.Contains(view, `"name"`) {
		t.Errorf("the streamed call was shown as text:\n%s", view)
	}
}

// Holding an open object is correct — a raw tool call must not be shown as
// text — but holding it SILENTLY is what froze the screen. Measured live
// 2026-08-30 23:03: the reply stopped at "I'll make the calls." and nothing
// moved for six minutes while the model went on generating, then hit the
// output cap. From the outside that is indistinguishable from a hang.
//
// So the filter has to say that it is holding something, and what.
func TestTheFilterReportsWhatItIsHolding(t *testing.T) {
	var f jsonCallFilter
	if f.Held() != 0 {
		t.Fatalf("holding %d bytes before anything arrived", f.Held())
	}
	if got := f.feed("I'll make the calls.\n"); got == "" {
		t.Fatal("ordinary text was withheld")
	}
	if f.Held() != 0 {
		t.Errorf("holding %d bytes after plain text", f.Held())
	}

	f.feed(`{"name":"bash","arg`)
	if f.Held() == 0 {
		t.Error("an open object is being held but Held() reports nothing — the screen has no way to show it")
	}
	if got := f.PendingName(); got != "bash" {
		t.Errorf("PendingName() = %q, want bash — the name is already in the buffer", got)
	}

	f.feed(`uments":{"command":"ls"}}`)
	if f.Held() != 0 {
		t.Errorf("still holding %d bytes after the object closed", f.Held())
	}
	if got := f.PendingName(); got != "" {
		t.Errorf("PendingName() = %q with nothing held", got)
	}
}

// The name is not always available yet, and guessing is worse than saying
// nothing — the indicator falls back to a generic label.
func TestPendingNameIsEmptyUntilItIsKnown(t *testing.T) {
	var f jsonCallFilter
	f.feed(`{"argum`)
	if got := f.PendingName(); got != "" {
		t.Errorf("PendingName() = %q before any name arrived", got)
	}
}

// THE NAMELESS HALF OF A SPLIT CALL IS HIDDEN, AND THE COMMA AFTER IT.
// Mutation check: require a name and {"arguments":{…}}, is the reply.
func TestTheNamelessHalfOfASplitCallIsHidden(t *testing.T) {
	var f jsonCallFilter
	var out string
	for _, chunk := range []string{
		"Looking at the source.\n",
		`{"arguments":{"path":"drone.mp4"}},` + "\n",
		`{"name":"clip_inspect"}`,
		"\nDone.",
	} {
		out += f.feed(chunk)
	}
	out += f.flush()
	if strings.Contains(out, "arguments") || strings.Contains(out, ",") {
		t.Fatalf("neither half nor the comma is prose: %q", out)
	}
	if !strings.Contains(out, "Looking at the source.") || !strings.Contains(out, "Done.") {
		t.Fatalf("the prose around it survives: %q", out)
	}
}
