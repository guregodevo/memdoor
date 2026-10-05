package gateway

import (
	"fmt"
	"strings"
)

// A recap after a lot of work.
//
// "We need a recap when there is a lot of work, like here in Claude"
// (Greg, 2026-09-19). A turn that ran a dozen tools and ended on "Done."
// leaves the person to scroll the frames for what was made. The last
// reply of a long turn must stand on its own: what was made (file,
// length), what changed, what is next. When it does not, the turn is
// asked for it once, with the same nudge mechanics the other retries use.
const (
	// recapAfterTools is how much work makes a recap due.
	recapAfterTools = 8
	// recapMinChars is the shortest ending that can be a recap.
	recapMinChars = 240
)

// recapWanted says whether a turn's final text is too thin for the work
// it did.
func recapWanted(tools int, text string) bool {
	return tools >= recapAfterTools && len(strings.TrimSpace(text)) < recapMinChars
}

// recapNudge is the one line at the top of the system prompt for the
// retry: what a recap is, for someone who saw none of the frames.
func recapNudge(tools int) string {
	return fmt.Sprintf("Your turn ran %d tools and ended in a line. End instead with a recap for someone who saw none of it: "+
		"what was made (each file with its length), what changed and why, and what is next — a few short lines, plain, no tool names.", tools)
}
