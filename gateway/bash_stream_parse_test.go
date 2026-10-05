package gateway

import (
	"context"
	"strings"
	"testing"
)

// The streaming bash path parsed its input with a bare unmarshal — no empty
// check, error discarded — while the one-shot path had a careful parser that
// absorbs split commands, unwraps envelopes, and refuses an empty command
// with guidance. Measured live 2026-08-31: the coder sent bash {} and got a
// SILENT empty success (`bash -c ""`, exit 0, empty output) — a wasted round
// trip that taught the model nothing. Two parsers is how the paths drift;
// the streamer now uses the same one.
func TestAStreamedEmptyBashCallIsRefusedWithGuidance(t *testing.T) {
	out, err := bashStreamer{}.Stream(context.Background(), []byte(`{}`), func(string) {})
	if err == nil && !strings.Contains(out, "command") {
		t.Fatalf("an empty bash call ran silently: out=%q err=%v", out, err)
	}
	msg := out
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "command") {
		t.Errorf("the refusal does not name what is missing: %q", msg)
	}
}

// The envelope shape gets the same which-mistake-you-made message as the
// one-shot path — same parser, so the two cannot disagree.
func TestAStreamedEnvelopeBashCallNamesTheMistake(t *testing.T) {
	out, err := bashStreamer{}.Stream(context.Background(), []byte(`{"name":"bash"}`), func(string) {})
	msg := out
	if err != nil {
		msg = err.Error()
	}
	if !strings.Contains(msg, "envelope") {
		t.Errorf("the envelope mistake is not named: %q", msg)
	}
}

// A split command is reassembled, exactly as the one-shot path does it.
func TestAStreamedSplitCommandIsReassembled(t *testing.T) {
	out, err := bashStreamer{}.Stream(context.Background(), []byte(`{"command":"echo","args":"reassembled"}`), func(string) {})
	if err != nil {
		t.Fatalf("split command refused: %v", err)
	}
	if !strings.Contains(out, "reassembled") {
		t.Errorf("the reassembled command did not run: %q", out)
	}
}
