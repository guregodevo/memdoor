package tools

import (
	"strings"
	"testing"
)

// The same finding, reworded, is not noted twice; a different finding is.
// The mutation check: skip the lookup and the second append goes in.
func TestTheSameFindingIsNotedOnce(t *testing.T) {
	dir := withNotes(t)
	first := `RULE (2026-09-19): "better quality" = sharper picture → re-encode at high bitrate (CRF 18, ~8 Mbps) with clip_convert, verify with ffprobe`
	if err := AppendNotes(dir, "note", first); err != nil {
		t.Fatal(err)
	}
	reworded := `RULE (2026-09-19): user's "better quality" = sharper picture → re-encode high bitrate (CRF 18, ~8 Mbps) via clip_convert; verify with ffprobe`
	err := AppendNotes(dir, "note", reworded)
	if err == nil || !strings.Contains(err.Error(), "already noted") {
		t.Fatalf("a reworded repeat must be refused as already noted, got %v", err)
	}
	if err := AppendNotes(dir, "note", "PICK: Fireship lines 1-6 (0-47s), the researcher who quit before vesting; CNN dropped"); err != nil {
		t.Fatalf("a different finding is noted: %v", err)
	}
	s, _ := ReadNotes(dir)
	if strings.Count(s, "## note") != 2 {
		t.Fatalf("want two entries, got:\n%s", s)
	}
	// Short entries are never mistaken for each other.
	for _, e := range []string{"done", "ok next"} {
		if err := AppendNotes(dir, "note", e); err != nil {
			t.Fatalf("short entry %q refused: %v", e, err)
		}
	}
}
