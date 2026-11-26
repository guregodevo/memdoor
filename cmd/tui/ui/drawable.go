package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// drawable is a tool's output made safe to draw. The terminal obeys a control
// byte wherever it appears: a backspace or carriage return moves the cursor
// out of the frame, an escape sequence can clear the screen or change its
// mode. Escape sequences and control characters are removed; a carriage
// return ends the line, so a progress bar reads as its successive states.
func drawable(s string) string {
	if !strings.ContainsFunc(s, undrawable) {
		return s
	}
	s = strings.ReplaceAll(ansi.Strip(s), "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\r':
			return '\n'
		case undrawable(r):
			return -1
		}
		return r
	}, s)
}

func undrawable(r rune) bool {
	return r != '\n' && r != '\t' && unicode.IsControl(r)
}
