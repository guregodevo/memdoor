package tools

import (
	"strings"
	"testing"
)

// "patch contains no file sections" describes the parser's state, not the
// model's mistake, and the model cannot act on it. Measured live 2026-08-30: it
// fired three times in one turn — the patch argument never arrived at all (a
// call-envelope bug, since fixed) — and the model spent two attempts rewriting
// a patch that was already correct, because the message pointed at the patch.
func TestAnEmptyPatchSaysTheArgumentWasEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\n", "  \n \t\n"} {
		_, err := parsePatch(in)
		if err == nil {
			t.Fatalf("parsePatch(%q) returned no error", in)
		}
		msg := err.Error()
		for _, want := range []string{"empty", "input"} {
			if !strings.Contains(strings.ToLower(msg), want) {
				t.Errorf("parsePatch(%q): message does not mention %q: %s", in, want, msg)
			}
		}
		if !strings.Contains(msg, "*** Update File:") {
			t.Errorf("parsePatch(%q): message does not show the required first line: %s", in, msg)
		}
	}
}

// A model narrates before it patches. That preamble is not a malformed hunk
// header, it is prose in front of a perfectly good patch — measured live as
// `invalid hunk header "The Todo_write tool is broken / unavailable, so I'll
// skip it..."`. Skip to the first file section instead of refusing.
func TestProseBeforeThePatchIsSkipped(t *testing.T) {
	patch := "I'll add the helper now.\n" +
		"Here is the change:\n" +
		"*** Update File: news.py\n" +
		"@@\n" +
		" def _text(el, tag):\n" +
		"+    # a comment\n"

	hunks, err := parsePatch(patch)
	if err != nil {
		t.Fatalf("prose before the patch was refused: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("parsed %d hunks, want 1", len(hunks))
	}
	if hunks[0].path != "news.py" {
		t.Errorf("path is %q, want news.py", hunks[0].path)
	}
}

// But prose with NO patch anywhere is a real mistake, and the message has to
// say what was missing rather than quote the prose back as a bad header.
func TestProseWithNoPatchNamesWhatIsMissing(t *testing.T) {
	_, err := parsePatch("I think we should refactor this later.\nNo patch here.")
	if err == nil {
		t.Fatal("prose with no patch was accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, "*** Add File:") || !strings.Contains(msg, "*** Update File:") {
		t.Errorf("message does not show the forms that were missing: %s", msg)
	}
	if !strings.Contains(msg, "no ") && !strings.Contains(msg, "No ") {
		t.Errorf("message does not say the section header is absent: %s", msg)
	}
}

// The preamble skip must not swallow a genuinely broken FIRST hunk: a patch
// that starts with a header keeps reporting its own errors.
func TestAMalformedHunkIsStillReported(t *testing.T) {
	_, err := parsePatch("*** Update File: a.go\n")
	if err == nil {
		t.Fatal("an update section with no chunks was accepted")
	}
	if !strings.Contains(err.Error(), "a.go") {
		t.Errorf("the error does not name the file: %s", err)
	}
}
