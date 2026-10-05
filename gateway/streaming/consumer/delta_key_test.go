package consumer

import "testing"

// The emitter names a streamed chunk "delta" (agent_runtime_inference.go:
// {"event":"text_delta","delta":…}). AssistantEventData had only a Text field
// bound to "text", so every token decoded to the zero value and the assembler
// accumulated nothing — the TUI showed a blank screen while the engine
// generated for minutes at 44 tok/s, indistinguishable from a hang.
//
// An unmatched JSON field is not an error, it is a zero value. That is exactly
// why this was invisible.
func TestTextDeltaAccumulatesTheEmittedChunk(t *testing.T) {
	sa := NewStreamAssembler(0, false)
	state := &RunState{}
	var got UpdateResult

	for _, chunk := range []string{"Hello", ", ", "world"} {
		got = UpdateResult{}
		sa.processAssistantEvent(state, AssistantEventData{Event: "text_delta", Delta: chunk}, &got)
	}

	if state.Text != "Hello, world" {
		t.Errorf("the assembler must accumulate the streamed text, got %q", state.Text)
	}
	if got.TextDelta != "world" {
		t.Errorf("the last delta must reach the client, got %q", got.TextDelta)
	}
}

// "text" still works — text_end and the legacy "text" event use it, and an
// older gateway may send a delta under that name.
func TestTextKeyStillAccepted(t *testing.T) {
	sa := NewStreamAssembler(0, false)
	state := &RunState{}
	var got UpdateResult
	sa.processAssistantEvent(state, AssistantEventData{Event: "text_delta", Text: "legacy"}, &got)
	if got.TextDelta != "legacy" || state.Text != "legacy" {
		t.Errorf(`the "text" key must still work: delta=%q state=%q`, got.TextDelta, state.Text)
	}
}

// A final "text" event flagged append carries only what never streamed (the
// receipt line, a note): the assembler adds it under the accumulated text
// instead of overwriting it, and the flag reaches the client.
func TestAnAppendedTextJoinsTheAccumulatedText(t *testing.T) {
	sa := NewStreamAssembler(0, false)
	state := &RunState{}
	sa.processAssistantEvent(state, AssistantEventData{Event: "text_delta", Delta: "The answer."}, &UpdateResult{})
	var got UpdateResult
	sa.processAssistantEvent(state, AssistantEventData{Event: "text", Text: "✓ checked: 1 file changed", Append: true}, &got)
	if state.Text != "The answer.\n\n✓ checked: 1 file changed" {
		t.Errorf("the tail joins the text, got %q", state.Text)
	}
	if !got.TextAppend || got.FullText != "✓ checked: 1 file changed" {
		t.Errorf("the flag and the tail must reach the client: %+v", got)
	}
	// Unflagged: the final text replaces the draft, as before.
	got = UpdateResult{}
	sa.processAssistantEvent(state, AssistantEventData{Event: "text", Text: "Rewritten."}, &got)
	if state.Text != "Rewritten." || got.TextAppend {
		t.Errorf("an unflagged final replaces: %q %+v", state.Text, got)
	}
}
