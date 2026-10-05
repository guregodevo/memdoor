package cmd

import (
	"strings"
	"testing"
)

// THE ONBOARDING WALK, BOTH WAYS (Greg, 2026-09-27: "test onboarding positive
// and negative paths"). Walked by hand on a clean HOME with a gateway of its
// own; what the walk found is asserted here so it cannot come back.

// Positive: the last thing setup says is what a stranger will actually type.
func TestSetupNextStepsSayWhatToTypeNext(t *testing.T) {
	out := nextStepsText("stranger")
	for _, want := range []string{
		"OPEN_ROUTER_API_KEY",    // their key, their bill
		"memdoor tui",            // the one command that starts the work
		"workspace use stranger", // what makes it work in a project directory
		"/model",                 // which model answers
		"/usage",                 // what it saved
		"decision model",         // it judges on their key, free
		"cron add --workflow",    // workflows and their schedules are free here
	} {
		if !strings.Contains(out, want) {
			t.Errorf("setup's next steps must mention %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "\n"); n > 8 {
		t.Errorf("less is more: %d lines of next steps\n%s", n, out)
	}
}

// Negative: the retired product must not be what a stranger is told to do.
func TestSetupNextStepsDoNotSellTheOldProduct(t *testing.T) {
	out := strings.ToLower(nextStepsText("stranger"))
	for _, gone := range []string{
		"credits topup",                      // the prepaid product
		"memdoor wiki append", "memdoor ask", // commands of a deleted surface
		"clip", "video", // the video product, moved out of Memdoor
		"149", // the old price
	} {
		if strings.Contains(out, gone) {
			t.Errorf("setup still points at %q:\n%s", gone, out)
		}
	}
}

// Negative: a first run is not an expired session. Someone who has just
// installed this and typed `memdoor tui` used to be told to run
// `memdoor auth login-direct --email … --password …`, for an account they do
// not have, instead of `memdoor setup`.
func TestFirstRunPointsAtSetup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := sessionProblem()
	if err == nil {
		t.Fatal("a machine with no credentials must report a problem")
	}
	msg := err.Error()
	if !strings.Contains(msg, "memdoor setup") {
		t.Errorf("the first run must be sent to setup: %q", msg)
	}
	if strings.Contains(msg, "login-direct") {
		t.Errorf("a stranger has no account to log in to: %q", msg)
	}
}
