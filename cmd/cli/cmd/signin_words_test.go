package cmd

import (
	"strings"
	"testing"
)

// The first run's OpenRouter choice says what the sign-in will ask: on a host
// with no browser (an ssh session, a Linux box with no display) it is a link
// to open elsewhere and a code to paste back, not "nothing to copy"
// (2026-11-18, a first run on Cloud Shell).
func TestOpenRouterSignInSaysWhatItAsks(t *testing.T) {
	t.Setenv("SSH_TTY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", ":0")

	t.Setenv("SSH_CONNECTION", "")
	if got := openRouterSignInHow(); !strings.Contains(got, "nothing to copy") {
		t.Fatalf("with a browser here: %q", got)
	}
	if g := brainGate("none"); !strings.Contains(g, "nothing to copy") && strings.Contains(g, "openrouter-login") {
		t.Fatalf("the no-model hint with a browser here: %q", g)
	}

	t.Setenv("SSH_CONNECTION", "10.0.0.2 50000 10.0.0.1 22")
	got := openRouterSignInHow()
	if strings.Contains(got, "nothing to copy") || !strings.Contains(got, "paste back the code") {
		t.Fatalf("over ssh, no browser here: %q", got)
	}
}
