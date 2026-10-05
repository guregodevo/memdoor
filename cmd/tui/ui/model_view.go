package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// model_view: View and the header/thinking/todos/picker/autocomplete/context/footer chrome.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Header
	header := m.renderHeader()

	// Activity bar (sticky at bottom, above footer)
	// Shows when: thinking, tools executing, or subagents running
	activityBar := ""
	// pendingCall belongs in this test as much as in the bar's label: a held
	// half-written call means the transcript has stopped moving, which is
	// precisely when the bar must exist to say why. It was only consulted
	// INSIDE the bar, so once streamed text cleared isThinking the indicator
	// could never show (live, 2026-08-31 12:24).
	// busy() includes the turn itself: between a tool's end and the next
	// round's first token, or while a long tool call is generated with no
	// text streaming, nothing is "thinking" and the bar vanished though the
	// turn ran on ("sometimes it's running and we don't see it", Greg,
	// 2026-09-26). A running turn always has a bar.
	if m.busy() || m.compacting {
		// Blank line above AND below the spinner so it doesn't visually glue to
		// the transcript above or the input box below.
		activityBar = "\n\n" + m.renderThinkingBar() + "\n"
	}

	// Jump bar: shown while the reader is scrolled up and new messages have
	// landed below — Claude-CLI style "there is more" overlay.
	jumpBar := ""
	if m.newBelow > 0 {
		label := fmt.Sprintf("↓ %d new message%s — End jumps to latest", m.newBelow, map[bool]string{true: "s"}[m.newBelow != 1])
		jumpBar = "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color(colRead)).
			Background(lipgloss.Color(colFill)).
			Padding(0, 1).
			Render(label)
	}

	// Todos bar (sticky at bottom, above footer, below activity bar)
	todosBar := ""
	if len(m.todos) > 0 {
		todosBar = "\n" + m.renderTodosBar()
	}

	// Autocomplete dropdown (shows above footer when active)
	autocompleteDropdown := ""
	if m.showAutocomplete {
		autocompleteDropdown = "\n" + m.renderAutocomplete()
	} else if m.showFileMentions {
		autocompleteDropdown = "\n" + m.renderDropdown(m.fileMatches, m.fileIndex)
	}

	// Interactive /model picker (shows above footer when open)
	modelPicker := ""
	if m.mcpPanel != nil {
		modelPicker = "\n" + m.renderMCPPanel()
	} else if m.workflowPanel != nil {
		modelPicker = "\n" + m.renderWorkflowPanel()
	} else if m.routePicker != nil {
		modelPicker = "\n" + m.renderRoutePicker()
	}

	// Footer
	footer := m.renderFooter()

	// Size the viewport to EXACTLY the space left by all the OTHER chrome, so the
	// total output is always m.height lines. A hardcoded margin overflowed whenever
	// a variable-height element appeared (the activity/todos bars, autocomplete,
	// the model picker, the question PICKER, or a wrapped plan-ready footer line) —
	// and output taller than the terminal makes it scroll down continuously. Each
	// of activityBar/todosBar/etc. carries a leading "\n", so strings.Count(…,"\n")
	// is the number of lines it adds; the "- 2" covers the two "\n" separators in
	// the format string (after the header, before the footer). m is a value copy,
	// so setting Height here only affects this render.
	extras := strings.Count(activityBar, "\n") + strings.Count(todosBar, "\n") +
		strings.Count(autocompleteDropdown, "\n") + strings.Count(modelPicker, "\n") +
		strings.Count(jumpBar, "\n")
	vpHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 2 - extras - headroom
	// Outside the altscreen the view is not a screen, it is a block drawn at the
	// bottom of the terminal and repainted in place. Sizing it to the full
	// terminal height would repaint every line on every frame — exactly what
	// moving to scrollback was meant to stop — and pad the live tail with blanks
	// that shove printed history off the top. So the viewport is as tall as its
	// CONTENT, and the terminal-height figure is only the ceiling.
	if n := m.viewport.TotalLineCount(); n < vpHeight {
		vpHeight = n
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	// The model's viewport keeps the height set at resize (terminal minus a
	// fixed margin); the height drawn here is smaller once the real chrome
	// — a wrapped header, the footer, the page strip, an activity bar — is
	// subtracted. A reader following the bottom was scrolled for the taller
	// box, so the last lines fell below the one drawn: a 7-line /model note
	// showed rungs 1–2 and stopped (80×24, 2026-09-26). Re-anchor to the
	// bottom of the box actually drawn.
	following := m.viewport.AtBottom()
	m.viewport.Height = vpHeight
	if following {
		m.viewport.GotoBottom()
	}

	// Return full view
	out := fmt.Sprintf("%s\n%s%s%s%s%s%s\n%s", header, m.viewport.View(), jumpBar, activityBar, todosBar, autocompleteDropdown, modelPicker, footer)

	return out
}

// setTitle names the conversation after the first thing asked of it, the way
// you would name it yourself looking back at the transcript. It never changes
// after that — a session is about what it started as.
func (m *Model) setTitle(text string) {
	if m.title != "" {
		return
	}
	title := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if title == "" {
		return
	}
	const max = 48
	if len([]rune(title)) > max {
		title = string([]rune(title)[:max-1]) + "…"
	}
	m.title = title
}

// renderHeader renders the header bar
func (m Model) renderHeader() string {
	// Determine connection status (plain text label, color-coded)
	var statusDot string
	var statusColor string
	if m.connected {
		statusDot = "live"
		statusColor = "10" // Green
	} else {
		statusDot = "offline"
		statusColor = "9" // Red
	}

	// Apply color to status dot
	coloredDotStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor))

	// Session key style
	sessionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colDim)). // Gray
		PaddingLeft(2)

	// Header left: status · channel · mode — always visible so the user
	// knows where they are and which permission mode is active.
	// The channel UUID used to sit here. It identified the session to the
	// gateway and told the reader nothing; the TITLE — what you asked for —
	// is what a person recognises a conversation by.
	scopeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colRecall)).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true)
	title := m.title
	if title == "" {
		title = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Italic(true).Render("new session")
	} else {
		title = titleStyle.Render(title)
	}
	left := sessionStyle.Render(fmt.Sprintf("%s %s · %s · %s",
		coloredDotStyle.Render(statusDot), scopeStyle.Render(m.scopeLabel()), title, m.modeLabel()))

	// Right: persistent context-window usage (Claude-style), color-graded by
	// utilization so it warns as the window fills.
	var right string
	if m.contextTokens > 0 {
		usage := formatTokensShort(m.contextTokens)
		color := "245" // dim
		if m.contextLimit > 0 {
			pct := float64(m.contextTokens) / float64(m.contextLimit) * 100
			usage = fmt.Sprintf("%s/%s · %.0f%%", formatTokensShort(m.contextTokens), formatTokensShort(m.contextLimit), pct)
			switch {
			case pct >= 90:
				color = "9" // red
			case pct >= 70:
				color = "11" // yellow
			}
		}
		right = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(usage + " ctx")
	}

	headerLine := left
	if right != "" && m.width > 0 {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
		if gap < 1 {
			gap = 1
		}
		headerLine = left + strings.Repeat(" ", gap) + right
	}

	return lipgloss.NewStyle().
		Width(m.width).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color(colRule)).
		Render(headerLine)
}

// formatTokensShort renders a compact token count for the header: "12.8k", "950".
func formatTokensShort(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

// thinkingVerbs are the rotating gerunds shown while the assistant works —
// Claude-CLI style (single whimsical words), not full sentences.
var thinkingVerbs = []string{
	"Cogitating", "Pondering", "Noodling", "Ruminating", "Percolating",
	"Synthesizing", "Crunching", "Composing", "Brewing", "Conjuring",
	"Considering", "Deliberating", "Wrangling", "Mulling", "Tinkering",
	"Puzzling", "Musing", "Inferring", "Orchestrating", "Marinating",
	"Simmering", "Churning", "Hatching", "Spelunking", "Vibing",
}

// formatElapsed renders a live duration like Claude Code: "12s" under a minute,
// "2m 30s" beyond. Guards negatives so a clock skew never prints garbage.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

// formatTokens converts token count to human-readable format (like Claude Code)
func formatTokens(tokens int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk tokens", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d tokens", tokens)
}

// renderThinkingBar renders the smart sticky indicator at the bottom
// Shows context-aware status: tooling, thinking, waiting for subagent, etc.
func (m Model) renderThinkingBar() string {
	// Elapsed since the activity (or thinking) began. Guard the zero value: a
	// time.Time{} is year 1, and time.Since(it) overflows int64 — which is what
	// rendered the "9223372036s" garbage. Fall back to thinkingStartTime, and to
	// 0 if neither is set.
	start := m.activityStartTime
	if start.IsZero() {
		start = m.thinkingStartTime
	}
	var elapsed time.Duration
	if !start.IsZero() {
		elapsed = time.Since(start)
	}

	var label string
	switch {
	case m.compacting:
		label = "Compacting context"
		if m.compactedTokens[0] > 0 {
			label = fmt.Sprintf("Compacting context (%s)", formatTokens(m.compactedTokens[0]))
		}
	case len(m.activeTools) > 0:
		if len(m.activeTools) == 1 {
			for toolName := range m.activeTools {
				label = "Running " + toolName
				break
			}
		} else {
			label = fmt.Sprintf("Running %d tools", len(m.activeTools))
		}
	case len(m.subagentWork) > 0:
		// A run this screen spawned, and what it is doing right now. Without
		// it the screen went blank for the whole delegated job: the work
		// happens on another session, so none of its events arrive here.
		label = subagentLabel(m.subagentWork)
	case len(m.activeSubagents) > 0:
		if len(m.activeSubagents) == 1 {
			label = "Delegating to a subagent"
		} else {
			label = fmt.Sprintf("Delegating to %d subagents", len(m.activeSubagents))
		}
	case m.pendingCall != "":
		// The transcript has stopped moving because a half-written tool call is
		// being held back rather than shown as raw JSON. Say so: silence here is
		// indistinguishable from a hang, and "Thinking" is the one thing it is
		// not doing. Measured live 2026-08-30 — six minutes of it.
		if m.pendingCall == pendingCallUnnamed {
			label = "Writing a tool call"
		} else {
			label = "Writing a " + displayName(m.pendingCall) + " call"
		}
		// The byte count is the progress: it grows while the model works, and
		// a number that stops moving is the honest signal something is stuck.
		if m.pendingCallBytes > 0 {
			label += fmt.Sprintf(" — %s so far", humanBytes(m.pendingCallBytes))
		}
	case m.isThinking:
		// Rotate the gerund every ~3s so it's readable, not a flicker.
		label = thinkingVerbs[int(elapsed.Seconds()/3)%len(thinkingVerbs)]
	case m.turnRunning:
		// The turn is alive and nothing more specific is known: the model
		// is generating (a tool call streams no text) or a round is about
		// to start. Say so rather than show nothing.
		label = "Working"
	default:
		return ""
	}

	// Animated spinner glyph + label + dim metadata.
	meta := formatElapsed(elapsed)
	if m.contextTokens > 0 {
		meta += " · " + formatTokens(m.contextTokens)
	}
	meta += " · esc to interrupt"

	frame := brailleFrames[int(elapsed.Milliseconds()/spinnerStep.Milliseconds())%len(brailleFrames)]
	glyph := lipgloss.NewStyle().Foreground(lipgloss.Color(spinnerColor)).Render(frame)
	labelStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Render(label + "…")
	metaStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Render("(" + meta + ")")
	return lipgloss.NewStyle().PaddingLeft(2).Render(glyph + " " + labelStyled + " " + metaStyled)
}

// subagentLabel says what the spawned runs are doing: the agent, the tool it
// is inside, and how long that tool has been running. The tool's own clock,
// not the turn's — "coder · bash 4m" is the line that separates a long build
// from a hang.
func subagentLabel(work map[string]subagentWorkState) string {
	if len(work) != 1 {
		return fmt.Sprintf("%d agents working", len(work))
	}
	for _, w := range work {
		who := w.agent
		if who == "" {
			who = "a subagent"
		}
		if w.tool == "" {
			return who + " is working"
		}
		line := who + " · " + displayName(w.tool)
		if w.seconds > 0 {
			line += " " + formatElapsed(time.Duration(w.seconds)*time.Second)
		}
		return line
	}
	return "a subagent is working"
}

// humanBytes renders a byte count the way a reader sizes it: B under a KB,
// one-decimal KB above.
func humanBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}

// pendingCallUnnamed marks a call being written whose name has not arrived
// yet. A sentinel rather than a bare bool so pendingCall stays one field with
// one meaning: what is being written, or nothing.
const pendingCallUnnamed = "\x00unnamed"

// brailleFrames animate the spinner glyph (frame derived from elapsed time).
var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// MOTION IS THE LOUDEST THING ON A QUIET SCREEN. The spinner stepped every
// 100ms in the palette's full running amber — ten glyph changes a second in
// the corner of the eye, for every tool, for as long as a build takes. It
// steps every 200ms now and sits a shade back from the tool name it belongs
// to: still plainly alive, no longer waving (Greg, 2026-10-01: colour that
// does not harm eyes).
//
// The spinner is a palette entry, not a hex: a truecolor escape falls back to
// the terminal's default foreground — white — wherever truecolor isn't
// honoured, which is exactly when the "it's working" signal matters.
const (
	spinnerColor = "137" // amber, a shade back from colRun
	spinnerStep  = 200 * time.Millisecond
)

// The TUI palette. Colour here carries meaning — what KIND of work a line is,
// or whether it went well — so a scrolling transcript can be read at a glance
// instead of parsed word by word. Anything decorative stays grey.
// FIVE COLOURS, ALL MUTED (Greg, 2026-10-01: "this should be for
// neurodivergent. Good color rendering that does not harm eyes"). There were
// nine meaning-hues at full saturation — pure red 196, pink 213, amber 214,
// yellow 220, orange 208, magenta 170 — and a scrolling transcript of them
// glares and competes for attention. Now: what a line DOES has three quiet
// hues, and whether it WENT WELL has two, every one of them low-chroma from
// the 256-colour cube (no truecolor escape to fall back from). A tool kind
// that had its own hue borrows the nearest one; its name and glyph already
// say which tool it is.
const (
	colRun    = "179" // soft amber — running a command
	colRead   = "109" // slate blue — reading or searching
	colWrite  = "139" // dusty mauve — changing files
	colPlan   = colRead
	colRecall = colRead
	colAsk    = colRun
	colClip   = colWrite
	colOK     = "108" // sage — it worked
	colWarn   = colRun
	colErr    = "174" // dusty rose — it failed
	colAccent = colRead
	colText   = "252" // soft white — emphasis that does not glare
	colDim    = "245" // grey — arguments, chrome, everything unremarkable

	// HUE SAYS WHAT KIND, LUMINANCE SAYS HOW IMPORTANT. There were eight grey
	// steps (235, 236, 238, 240, 241, 242, 245, 252) doing three jobs, and
	// three of them — 238, 240, 241 — sit at 1.7:1, 2.3:1 and 2.7:1 against a
	// dark terminal. Text at that contrast is not quiet, it is unreadable, and
	// 240 was carrying a tool's own prose. So there are four levels now, each
	// with the job it can actually do:
	//
	//	colText  252  10.8:1  what you are meant to read
	//	colDim   245   4.8:1  secondary text — still readable (WCAG AA)
	//	colFaint 242   3.2:1  ancillary marks: line numbers, a struck-out item
	//	colRule  238   1.7:1  STRUCTURE ONLY — gutters, borders, separators
	//
	// colFill is the one tinted surface: a diff band, a selected row. It sits
	// just off the page so a band reads as a region without becoming a slab.
	colFaint = "242"
	colRule  = "238"
	colFill  = "236"
)

// A SELECTED ROW IS NOT A COLOURED SLAB. Pairing the palette's slate on a
// bright foreground gave 1.55:1 — a row you could see and not read. Selection
// is luminance now (a neutral fill 1.7:1 off the page, text at 6.3:1 on it)
// and the → cursor beside it carries the accent. Hue stays for what a line IS.
const (
	colSelBG = colRule
	colSelFG = colText
)

// renderTodosBar renders the todos sticky bar at the bottom
func (m Model) renderTodosBar() string {
	if len(m.todos) == 0 {
		return ""
	}

	// Same glyphs and colours as the checklist frame — one state, one look.
	var todoItems []string
	for _, todo := range m.todos {
		icon, iconColor := statusGlyph(todo.Status)
		iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor))

		// Use activeForm for in_progress, content otherwise
		text := todo.Content
		if todo.Status == "in_progress" && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}

		// Truncate long todo text
		maxLen := 50
		if len(text) > maxLen {
			text = text[:maxLen-3] + "..."
		}

		// The live step is the one the reader is looking for — brighten it and
		// fade what is already done.
		switch todo.Status {
		case "in_progress":
			text = lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true).Render(text)
		case "completed":
			text = lipgloss.NewStyle().Foreground(lipgloss.Color(colFaint)).Render(text)
		}
		todoItems = append(todoItems, fmt.Sprintf("%s %s", iconStyle.Render(icon), text))
	}

	// Join all todos with separator
	todosText := strings.Join(todoItems, " · ")

	// Render with dark background
	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colText)). // Light gray text
		Background(lipgloss.Color(colFill)). // Dark gray background
		Width(m.width).
		PaddingLeft(2)

	return barStyle.Render(todosText)
}

// renderAutocomplete renders the slash command autocomplete dropdown
func (m Model) renderAutocomplete() string {
	return m.renderDropdown(m.filteredCommands, m.autocompleteIndex)
}

// renderDropdown renders a picker of entries above the footer, windowed
// around the selection (slash commands, @file mentions).
func (m Model) renderDropdown(entries []string, index int) string {
	if len(entries) == 0 {
		return ""
	}

	// WINDOW the list. Rendering every match (30+ with skills loaded) makes a box
	// taller than the terminal: the top rows scroll off the screen edge, and the
	// selection can sit outside what is drawn — which reads as "I can't scroll".
	// Show a slice that follows the selection instead, with counts at the edges.
	maxRows := m.height - 12 // header, input box, help line, hint, breathing room
	if maxRows > 12 {
		maxRows = 12
	}
	if maxRows < 3 {
		maxRows = 3
	}
	start := 0
	if len(entries) > maxRows {
		// Keep the selection inside the window, one row of context where possible.
		start = index - maxRows/2
		if start < 0 {
			start = 0
		}
		if start > len(entries)-maxRows {
			start = len(entries) - maxRows
		}
	}
	end := start + maxRows
	if end > len(entries) {
		end = len(entries)
	}

	moreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Italic(true)

	var items []string
	if start > 0 {
		items = append(items, moreStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	for i := start; i < end; i++ {
		cmd := entries[i]
		if i == index {
			// Highlight selected item
			selectedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colSelFG)).
				Background(lipgloss.Color(colSelBG)).
				Bold(true)
			items = append(items, selectedStyle.Render(" "+cmd+" "))
		} else {
			// Normal item
			normalStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colText)) // Light gray
			items = append(items, normalStyle.Render(" "+cmd))
		}
	}
	if end < len(entries) {
		items = append(items, moreStyle.Render(fmt.Sprintf("  ↓ %d more", len(entries)-end)))
	}

	// Add navigation hint
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colDim)). // Dark gray
		Italic(true).
		PaddingLeft(1)
	hint := hintStyle.Render("up/down navigate • Enter/Tab select • Esc close")

	content := strings.Join(items, "\n") + "\n" + hint

	// Render with border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colRead)). // Blue border
		Padding(0, 1).
		Width(m.width - 4)

	return boxStyle.Render(content)
}

func (m Model) renderFooter() string {
	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderForeground(lipgloss.Color(colRule)).
		PaddingLeft(1)
	// THE RULES MUST LINE UP (battle test, 2026-09-27). The header renders at
	// Width(m.width); this box was sized to its content, so its rules came out
	// three columns short and the window had a ragged right edge at every
	// terminal size.
	// Width is padding-inclusive in lipgloss, and the top and bottom borders
	// span exactly it, so the same number as the header's is what lines up.
	if m.width > 1 {
		inputStyle = inputStyle.Width(m.width)
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colDim)).
		PaddingLeft(2)

	// /connect takes over the footer while it runs (connect_flow.go).
	if m.connect != nil {
		return m.renderConnect(inputStyle, helpStyle)
	}

	// Interactive question picker takes over the footer while the agent waits.
	if q := m.pendingQuestion; q != nil {
		// Constrain to the terminal width so a long question/option WRAPS instead of
		// rendering one over-long line that the terminal cuts off ("truncated question").
		// -4 leaves room for the border + left padding of inputStyle.
		qw := m.width - 4
		if qw < 20 {
			qw = 20
		}
		sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colAsk)).Bold(true).Width(qw)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Width(qw)
		var b strings.Builder
		b.WriteString(sel.Render(q.question) + "\n")
		for i, opt := range q.options {
			line := fmt.Sprintf("  %d. %s", i+1, opt)
			if i == q.index {
				b.WriteString(sel.Render(">"+line[1:]) + "\n")
			} else {
				b.WriteString(dim.Render(line) + "\n")
			}
		}
		b.WriteString(helpStyle.Render("up/down move · enter select · 1-9 pick"))
		return inputStyle.Render(b.String())
	}

	// Always-auto: the coder writes and runs without asking; the only stop is
	// the ask_user_question picker. The mode badge lives in the header — it does
	// not need saying twice on one screen.
	// Only keys that are non-obvious and useful RIGHT NOW. Dropped from the old
	// line: "questions interrupt via picker" (an explanation, and the picker
	// announces itself), "wheel/PgUp scroll" (every terminal does that), and
	// "End: latest" (the jump bar says exactly that, exactly when it matters).
	keys := "/ commands · @ files · ctrl+v image · alt+enter newline · ctrl+o expand · ctrl+c quit"
	if len(pageAgents()) > 1 {
		// ctrl+t only appears when there is a second page to switch to.
		keys = "/ commands · @ files · ctrl+v image · alt+enter newline · ctrl+t page · ctrl+o expand · ctrl+c quit"
	}
	// busy() includes the turn itself: between a tool's end and the next
	// token — a brain retrying a 503 for a minute — nothing was "thinking"
	// and the bottom of the window went blank as if the turn were over
	// (run 10, 2026-09-20). A running turn always says esc stops it.
	if m.busy() {
		keys = "esc interrupt · ctrl+o expand"
	}
	// The suggestion is the box's grey placeholder while the box is empty,
	// and the one key that acts on it is named under the box.
	if text, send := m.hint(); text != "" {
		m.input.Placeholder = text
		if send != "" && keys == "" {
			keys = "tab sends it"
		}
	}
	help := helpStyle.Render(keys)
	if status := m.status.line(); status != "" {
		line := lipgloss.NewStyle().PaddingLeft(2).Render(status)
		help = line + "\n" + help
	}

	// Input sits in the bordered box (delineated above and below); the mode/help
	// line goes BELOW the box, not inside it.
	return inputStyle.Render(m.input.View()) + "\n" + help
}
