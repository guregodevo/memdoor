package workflow

import "strings"

// ErrorText is a failed task's error as a person reads it. A command task's
// error from mario repeats the whole command before the exit status ("task
// <name>: <command>: exit status 1 — <what it printed>"); the command is in
// the step's file, and read in a window the script looked like the reason
// (#128: team-solve's problem with no prompt read as if its files were not
// found). What failed is the status and what the command printed.
func ErrorText(e string) string {
	if i := strings.Index(e, ": exit status "); i >= 0 {
		return e[i+2:]
	}
	return e
}

// ErrorLines is ErrorText as a window draws it: the status line, then the
// last n lines the command printed, with a mark for what is left out.
func ErrorLines(e string, n int) []string {
	lines := strings.Split(strings.TrimSpace(ErrorText(e)), "\n")
	if len(lines) <= n+1 {
		return lines
	}
	out := []string{lines[0], "…"}
	return append(out, lines[len(lines)-n:]...)
}
