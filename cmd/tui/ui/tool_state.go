package ui

import (
	"strings"

	"memdoor/tools"
)

// WHAT THE BULLET MEANS. One answer for the whole screen, so the marker, a
// test and anything else asking agree — and so the answer is a fact about the
// frame rather than a guess from what happens to be empty.
type toolStatus int

const (
	toolRunning toolStatus = iota
	toolPlanBlocked
	toolFailed
	toolSucceeded
)

func toolState(msg Message) toolStatus {
	switch {
	case !msg.toolSettled():
		return toolRunning
	case msg.ToolError != "" && strings.Contains(msg.ToolError, "plan mode"):
		// Expected read-only guidance, not a failure: never alarming red.
		return toolPlanBlocked
	case msg.ToolError != "":
		return toolFailed
	case strings.HasPrefix(msg.ToolOutput, tools.CommandFailedPrefix):
		// A command that exited non-zero reports through its OUTPUT, so a
		// failed build wore the green bullet while the body under it read
		// "Command FAILED" (live 2026-10-01).
		return toolFailed
	default:
		return toolSucceeded
	}
}
