package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Diff rendering.
//
// An edit frame exists to answer one question: what changed? Foreground-only
// +/- lines force the reader to hunt for the sign character at the start of a
// line that is already indented by the code's own whitespace. So added and
// removed lines get a tinted background running the width of the frame, a
// gutter bar, and a header that says which file and how much moved — the shape
// every diff viewer converged on.

// A tinted band is the brightest thing in a transcript, and green-on-green /
// red-on-red at full chroma is what hurts after an hour of patches. The band
// stays — it is what makes a diff readable without hunting for the sign — but
// it is NEUTRAL now, and the hue that says added or removed sits in the text
// and the gutter instead of behind them.
const (
	diffAddBG = "236" // neutral dark grey, both sides
	diffDelBG = "236"
	diffAddFG = colOK  // sage
	diffDelFG = colErr // dusty rose
)

// fileEcho pulls the post-edit file out of an apply_patch result — the tool
// appends "<path> is now:\n<content>" so the model patches current text. It is
// also what lets the diff carry REAL line numbers instead of invented ones.
func fileEcho(output string) string {
	i := strings.Index(output, " is now:\n")
	if i < 0 {
		return ""
	}
	return output[i+len(" is now:\n"):]
}

// numberRows walks the patch against the file it produced and assigns each
// displayed row its line number in that file. A removed line has no line in the
// new file, so it takes the number of the position it occupied. Any row that
// cannot be located gets no number rather than a wrong one.
func numberRows(body []string, newContent string) []int {
	nums := make([]int, len(body))
	if newContent == "" {
		return nums
	}
	newLines := strings.Split(strings.TrimRight(newContent, "\n"), "\n")
	at := 0 // how far into the new file we have matched
	same := func(a, b string) bool {
		return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
	}
	// The file usually continues where we left off, so check that position
	// FIRST and only search ahead when it doesn't match. Searching first breaks
	// on blank lines — every empty line matches every other one, so the pointer
	// leaps down the file and the rows after it lose their numbers.
	find := func(text string) int {
		if at < len(newLines) && same(newLines[at], text) {
			return at
		}
		for j := at + 1; j < len(newLines) && j < at+40; j++ {
			if same(newLines[j], text) {
				return j
			}
		}
		return -1
	}

	// First pass: rows that still exist in the file can be located directly.
	for i, line := range body {
		switch {
		case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "@@"):
			// resolved in the second pass / no line of its own
		case strings.HasPrefix(line, "+"):
			if j := find(strings.TrimPrefix(line, "+")); j >= 0 {
				nums[i] = j + 1
				at = j + 1
			}
		default:
			if j := find(strings.TrimPrefix(line, " ")); j >= 0 {
				nums[i] = j + 1
				at = j + 1
			}
		}
	}

	// Second pass: a removed line has no line in the new file, so it takes the
	// number of the line that replaced it — the next resolved row — which is
	// where a reader looking at the file would find the change. Only when
	// nothing follows does it fall back to the row before it.
	for i, line := range body {
		if !strings.HasPrefix(line, "-") {
			continue
		}
		for j := i + 1; j < len(body); j++ {
			if nums[j] > 0 {
				nums[i] = nums[j]
				break
			}
		}
		if nums[i] == 0 {
			for j := i - 1; j >= 0; j-- {
				if nums[j] > 0 {
					nums[i] = nums[j] + 1
					break
				}
			}
		}
	}
	return nums
}

// renderDiff draws an apply_patch body with line numbers from the file the
// patch produced. Capped unless the frame is expanded (ctrl+o), and the cap is
// announced rather than silent.
func renderDiff(patch, newContent string, expand, failed bool) string {
	lines := strings.Split(strings.TrimRight(patch, "\n"), "\n")

	added, removed := 0, 0
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			added++
		case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			removed++
		}
	}

	path, op := patchTarget(patch)
	if path == "" {
		path = "(patch)"
	}
	if op == "" {
		op = "Edit"
	}

	var (
		head    = lipgloss.NewStyle().Foreground(lipgloss.Color(colWrite)).Bold(true)
		plus    = lipgloss.NewStyle().Foreground(lipgloss.Color(colOK)).Bold(true)
		minus   = lipgloss.NewStyle().Foreground(lipgloss.Color(colErr)).Bold(true)
		dim     = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
		addLine = lipgloss.NewStyle().Foreground(lipgloss.Color(diffAddFG)).Background(lipgloss.Color(diffAddBG))
		delLine = lipgloss.NewStyle().Foreground(lipgloss.Color(diffDelFG)).Background(lipgloss.Color(diffDelBG))
		ctxLine = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
		gutter  = lipgloss.NewStyle().Foreground(lipgloss.Color(colRule))
		lineNo  = lipgloss.NewStyle().Foreground(lipgloss.Color(colFaint))
	)

	var b strings.Builder
	if failed {
		// Say so in the header, where the eye lands, not only in the error line
		// below the hunk.
		b.WriteString("  " + frameConnector + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color(colErr)).Bold(true).Render(op+" "+path+" — not applied"))
	} else {
		b.WriteString("  " + frameConnector + " " + head.Render(op+" "+path))
		if added > 0 || removed > 0 {
			b.WriteString("  " + plus.Render(fmt.Sprintf("+%d", added)) + " " + minus.Render(fmt.Sprintf("−%d", removed)))
		}
	}

	// The body is everything but the `*** …` control lines, which the header
	// already said.
	var body []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "*** ") {
			body = append(body, line)
		}
	}
	// A leading or trailing empty row is a gutter bar with nothing beside it.
	for len(body) > 0 && strings.TrimSpace(body[0]) == "" {
		body = body[1:]
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	// An added file is numbered by construction; an update is numbered against
	// the file the patch produced.
	if newContent == "" && op == "Add" {
		newContent = strings.Join(strippedAdds(body), "\n")
	}
	nums := numberRows(body, newContent)

	max := 14
	if expand {
		max = 400
	}
	shown := 0
	for i, line := range body {
		if shown >= max {
			break
		}
		shown++
		num := "    "
		if nums[i] > 0 {
			num = fmt.Sprintf("%4d", nums[i])
		}
		b.WriteString("\n  " + gutter.Render("▏") + lineNo.Render(num) + " ")
		switch {
		case strings.HasPrefix(line, "+"):
			b.WriteString(addLine.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(delLine.Render(line))
		default:
			b.WriteString(ctxLine.Render(line))
		}
	}

	if len(body) > shown {
		b.WriteString("\n  " + dim.Render(fmt.Sprintf("… +%d lines (ctrl+o)", len(body)-shown)))
	}
	return b.String()
}

// strippedAdds is the content an "Add File" patch creates — every line is new,
// so the file it produces is the patch body with the + markers removed.
func strippedAdds(body []string) []string {
	out := make([]string, 0, len(body))
	for _, l := range body {
		out = append(out, strings.TrimPrefix(l, "+"))
	}
	return out
}

// unwrapPatch digs the patch text out of however the model wrapped it: one or
// more JSON envelopes ({"input":…}, {"name","arguments":{"input":…}}), a
// trailing brace remnant, or a half-escaped single line of literal \n.
func unwrapPatch(raw string) string {
	patch := raw
	for range [3]int{} {
		var env struct {
			Input     string `json:"input"`
			Arguments *struct {
				Input string `json:"input"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(patch), &env); err != nil {
			break
		}
		switch {
		case env.Arguments != nil && env.Arguments.Input != "":
			patch = env.Arguments.Input
		case env.Input != "":
			patch = env.Input
		default:
		}
		if strings.HasPrefix(strings.TrimSpace(patch), "*** ") {
			break
		}
	}
	if i := strings.Index(patch, "*** "); i > 0 {
		patch = patch[i:]
	}
	// Shed a trailing JSON wrapper so its real newlines don't mask the patch's
	// own literal \n escapes.
	if k := strings.LastIndex(patch, "\""); k >= 0 {
		if tail := strings.TrimSpace(patch[k+1:]); tail != "" && strings.Trim(tail, "}] \t\r\n") == "" {
			patch = strings.TrimRight(patch[:k], " \t\r\n")
		}
	}
	if strings.Count(patch, "\n") < 2 && strings.Contains(patch, "\\n") {
		patch = strings.ReplaceAll(patch, "\\n", "\n")
		patch = strings.ReplaceAll(patch, "\\t", "\t")
		patch = strings.ReplaceAll(patch, "\\\"", "\"")
	}
	return patch
}

// renderEditResult draws an edit_file RESULT — the tool's own line-numbered
// hunk, whose first line is a "Updated <path>" summary.
func renderEditResult(out string) string {
	var (
		head    = lipgloss.NewStyle().Foreground(lipgloss.Color(colWrite)).Bold(true)
		addLine = lipgloss.NewStyle().Foreground(lipgloss.Color(diffAddFG)).Background(lipgloss.Color(diffAddBG))
		delLine = lipgloss.NewStyle().Foreground(lipgloss.Color(diffDelFG)).Background(lipgloss.Color(diffDelBG))
		ctxLine = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
		gutter  = lipgloss.NewStyle().Foreground(lipgloss.Color(colRule))
	)

	var b strings.Builder
	for i, line := range strings.Split(out, "\n") {
		if i == 0 {
			b.WriteString("  " + head.Render(line))
			continue
		}
		b.WriteString("\n  " + gutter.Render("▏"))
		switch {
		case strings.HasPrefix(line, "+"):
			b.WriteString(addLine.Render(" " + line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(delLine.Render(" " + line))
		default:
			b.WriteString(ctxLine.Render(" " + line))
		}
	}
	return b.String()
}
