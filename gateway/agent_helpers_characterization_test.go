package gateway

import (
	"testing"
)

// Characterization tests for the pure helpers in agent_adapter.go that had no
// existing coverage. They lock the CURRENT behavior so the file split (which
// relocates these functions into concern-named files) cannot change what they
// do. The honesty guards, fencing, citation/quote validation, and workdir
// confinement are already covered by mutation_claim_test.go,
// prompt_injection_test.go, citation_guard_test.go, and coder_workdir_test.go.

func TestLooksLikeCorrection(t *testing.T) {
	if !looksLikeCorrection("No, that's wrong") || !looksLikeCorrection("actually I meant X") {
		t.Error("missed a correction")
	}
	if looksLikeCorrection("thanks, that helps") {
		t.Error("false positive")
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("short string altered: %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello..." {
		t.Errorf("truncateStr = %q, want hello...", got)
	}
}

func TestTranscriptKey(t *testing.T) {
	if got := transcriptKey("workspace:w:channel:c", "chief"); got != "workspace:w:channel:c:agent:chief" {
		t.Errorf("transcriptKey = %q", got)
	}
	if got := transcriptKey("agent:coder:step-1", "coder"); got != "agent:coder:step-1" {
		t.Errorf("transcriptKey should not touch an agent key: %q", got)
	}
	if got := transcriptKey("workspace:w:channel:c", ""); got != "workspace:w:channel:c" {
		t.Errorf("empty agent must leave the key unchanged: %q", got)
	}
}
