package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"golang.org/x/term"
)

// Consistent CLI epilogue: after a command succeeds, print one "what to do next"
// line so every step of the journey points at the next — a first-time user
// (e.g. someone onboarding a friend) is never left wondering what to run.
// Implemented centrally via rootCmd.PersistentPostRunE (cobra runs it only on
// RunE success, so we never suggest after an error).
//
// Two layers:
//  1. nextStepSuggestions — a hand-tuned suggestion per journey command. Always
//     printed (even when piped), because these are the moments where the right
//     next step genuinely matters and the text is curated.
//  2. a generic fallback ("memdoor --help") for any other command — printed ONLY
//     when stdout is a real terminal, so it never corrupts piped/captured output.
//
// machineReadablePrefixes lists commands whose stdout is data (tokens, slugs,
// JSON, log streams, shell-prompt fragments) or which run long / have no natural
// next step — they get NO suggestion at all, even on a TTY. setup is excluded
// here because it prints its own rich next-steps block.
var nextStepSuggestions = map[string]string{
	// workspace use is the last step before the first turn, so it points at it.
	"workspace use":     `memdoor tui   (the agent works in this directory)`,
	"auth login-direct": `memdoor tui   (the agent works in this directory)`,
	"auth register":     `memdoor tui   (the agent works in this directory)`,
}

// machineReadablePrefixes are command paths whose stdout must stay clean (data,
// streams) or which have no useful next step. A command matches if its key
// equals a prefix or starts with "<prefix> " — so "auth token" covers itself
// and "logs" covers "logs query" / "logs errors".
var machineReadablePrefixes = []string{
	"tui", // interactive: whatever it printed last (a worktree's hand-back) is the last word
	"auth token",
	"auth whoami",
	"workspace which",
	"messages",
	"logs",
	"completion",
	"__complete",
	"shell-prompt",
	"chrome",
	"gateway",
	"mcp",
}

// commandKey is the command path without the leading "memdoor " — e.g.
// "memdoor workspace use" → "workspace use".
func commandKey(cmd *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()))
}

// isMachineReadable reports whether the command's output should never carry a
// next-step suggestion.
func isMachineReadable(key string) bool {
	for _, p := range machineReadablePrefixes {
		if key == p || strings.HasPrefix(key, p+" ") {
			return true
		}
	}
	return false
}

// printNextStep is rootCmd.PersistentPostRunE: prints the mapped next-step
// suggestion for the command that just succeeded, or a generic fallback on a
// TTY, so the user always knows what to do next.
func printNextStep(cmd *cobra.Command, _ []string) error {
	key := commandKey(cmd)
	if isMachineReadable(key) {
		return nil
	}
	if s, ok := nextStepSuggestions[key]; ok {
		fmt.Fprintf(os.Stdout, "\n→ Next: %s\n", s)
		return nil
	}
	// No curated suggestion — point at the command index, but only interactively
	// so piped/captured stdout is never polluted.
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stdout, "\n→ Next: memdoor --help   (see all commands)\n")
	}
	return nil
}
