package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Checklist rendering.
//
// todo_write returns a numbered "1. [x] …" list plus a follow-up line. Rendered
// as plain text it is the densest frame in a run and the one most worth reading
// at a glance: it says what the agent thinks it is doing and how far along it
// is. So it draws as a checklist — one glyph per state, the live item picked
// out, everything done faded back, and the progress said once at the top.

// Status glyphs, shared by the tool frame and the sticky bar so the same state
// looks the same in both places.
const (
	glyphDone    = "✔"
	glyphActive  = "▸"
	glyphPending = "○"
)

// statusGlyph maps a todo status to its glyph and colour.
func statusGlyph(status string) (glyph, colour string) {
	switch status {
	case "completed":
		return glyphDone, colOK
	case "in_progress":
		return glyphActive, colPlan
	default:
		return glyphPending, colDim
	}
}

var checklistItemRE = regexp.MustCompile(`^\s*\d+\.\s*\[([ ~x])\]\s*(.+)$`)
var checklistProgressRE = regexp.MustCompile(`^\((\d+)/(\d+) done\)\s*(.*)$`)

// renderChecklist draws a todo_write / todo_read result. Returns "" when the
// output isn't a checklist, so the caller falls back to the generic body.
func renderChecklist(out string, expand bool) string {
	type item struct{ status, text string }
	var items []item
	var doneN, totalN int
	var followup string

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if m := checklistItemRE.FindStringSubmatch(line); m != nil {
			status := "pending"
			switch m[1] {
			case "x":
				status = "completed"
			case "~":
				status = "in_progress"
			}
			items = append(items, item{status, strings.TrimSpace(m[2])})
			continue
		}
		if m := checklistProgressRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			fmt.Sscanf(m[1], "%d", &doneN)
			fmt.Sscanf(m[2], "%d", &totalN)
			followup = strings.TrimSpace(m[3])
		}
	}
	if len(items) == 0 {
		return ""
	}
	if totalN == 0 {
		totalN = len(items)
	}

	var (
		head    = lipgloss.NewStyle().Foreground(lipgloss.Color(colPlan)).Bold(true)
		barOn   = lipgloss.NewStyle().Foreground(lipgloss.Color(colOK))
		barOff  = lipgloss.NewStyle().Foreground(lipgloss.Color(colRule))
		doneTxt = lipgloss.NewStyle().Foreground(lipgloss.Color(colFaint)).Strikethrough(true)
		liveTxt = lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true)
		todoTxt = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
		nextTxt = lipgloss.NewStyle().Foreground(lipgloss.Color(colPlan)).Italic(true)
		dim     = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Italic(true)
	)

	// A short bar makes progress readable without counting glyphs.
	const barWidth = 10
	filled := 0
	if totalN > 0 {
		filled = doneN * barWidth / totalN
	}
	bar := barOn.Render(strings.Repeat("▰", filled)) + barOff.Render(strings.Repeat("▱", barWidth-filled))

	var b strings.Builder
	b.WriteString("  " + frameConnector + " " + head.Render("Checklist") + "  " + bar + "  " +
		head.Render(fmt.Sprintf("%d/%d", doneN, totalN)))

	max := 12
	if expand {
		max = 200
	}
	shown := 0
	for _, it := range items {
		if shown >= max {
			break
		}
		shown++
		glyph, colour := statusGlyph(it.status)
		text := it.text
		switch it.status {
		case "completed":
			text = doneTxt.Render(text)
		case "in_progress":
			text = liveTxt.Render(text)
		default:
			text = todoTxt.Render(text)
		}
		b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(glyph) + " " + text)
	}
	if len(items) > shown {
		b.WriteString("\n  " + dim.Render(fmt.Sprintf("… %d more (ctrl+o)", len(items)-shown)))
	}

	// The follow-up is the tool's whole point — it names the next action, and
	// the agent is told never to stop while one exists. Keep it visible.
	if followup != "" {
		b.WriteString("\n  " + nextTxt.Render("→ "+followup))
	}
	return b.String()
}
