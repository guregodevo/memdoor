package gateway

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A refusal's reason must fit a close frame (123 bytes): past that the frame
// is not sent, the client reads a drop and reconnects forever.
func TestRefusalReasonFitsACloseFrame(t *testing.T) {
	long := "not your workspace (yours is " + strings.Repeat("é", 100) + ")"
	got := clip(long, 123)
	if len(got) > 123 || !utf8.ValidString(got) {
		t.Fatalf("clip gave %d bytes, valid UTF-8 %v", len(got), utf8.ValidString(got))
	}
	if clip("short", 123) != "short" {
		t.Fatal("a short reason changed")
	}
}
