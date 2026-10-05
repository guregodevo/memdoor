package ui

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// model_render: message rendering, tick commands, message types, and markdown/highlighting.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// renderMessages renders all messages
// gotoBottom scrolls the viewport to the end, but ONLY once it has real dimensions.
// Before the first WindowSizeMsg the viewport Height is 0; GotoBottom then sets YOffset
// past the content and the viewport's visibleLines slices out of range — a hard panic
// that kills the whole TUI (reproduced when the terminal reports a 0x0 size on startup).
// Guarding here covers every call site.
// refreshFollow re-renders the transcript and snaps to the newest output ONLY
// when the user was already at the bottom. A reader who scrolled up to study a
// long answer is never yanked down by streaming deltas or tool events — End
// returns them to live. User-initiated actions still snap unconditionally.
func (m *Model) refreshFollow() {
	wasAtBottom := m.viewport.AtBottom()
	before := len(m.messages)
	m.viewport.SetContent(m.renderMessages())
	if wasAtBottom || m.viewport.AtBottom() {
		m.gotoBottom()
		return
	}
	// Reader is scrolled up: count what lands below and offer the jump bar.
	//
	// Count MESSAGES, not refreshes. This incremented once per call, and
	// streaming calls it once per delta — so a single reply arriving in
	// hundreds of chunks reported "↓ 87 new messages" when three had appeared
	// (2026-08-30). A count that grows with typing speed tells the reader
	// nothing about what they missed.
	if before > m.lastCounted {
		m.newBelow += before - m.lastCounted
	}
	m.lastCounted = before
}

func (m *Model) gotoBottom() {
	m.newBelow = 0
	// Every jump to the bottom is caught up, not just refreshFollow's: a
	// resume loads ~100 messages through SetContent+gotoBottom, and a baseline
	// left at 0 announced the whole history as "↓ 110 new messages" (2026-09-28).
	m.lastCounted = len(m.messages)
	if m.viewport.Height > 0 {
		m.viewport.GotoBottom()
	}
}

// transcriptKeys is the viewport's key map. The input box has focus, so a
// key a prompt can contain must never scroll the transcript — bubbles'
// default binds j/k/b/u/d/f/space, and typing "back" paged it up.
// ctrl+u/ctrl+d stay with the input box (line editing).
func transcriptKeys() viewport.KeyMap {
	km := viewport.DefaultKeyMap()
	km.PageDown = key.NewBinding(key.WithKeys("pgdown"))
	km.PageUp = key.NewBinding(key.WithKeys("pgup"))
	km.HalfPageDown = key.NewBinding(key.WithDisabled())
	km.HalfPageUp = key.NewBinding(key.WithDisabled())
	km.Down = key.NewBinding(key.WithKeys("down"))
	km.Up = key.NewBinding(key.WithKeys("up"))
	km.Left = key.NewBinding(key.WithDisabled())
	km.Right = key.NewBinding(key.WithDisabled())
	return km
}

func (m Model) renderMessages() string {
	if len(m.messages) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colDim)).
			Italic(true).
			PaddingLeft(2).
			PaddingTop(2)
		hint := "No messages yet. Type a message to get started!"
		// The hint is a sentence or two: wrapped to the window, or the
		// window clips it at the right edge ("it's truncated", Greg,
		// 2026-09-19). A sensible line length, never the whole width.
		if w := m.viewport.Width - 4; w > 20 {
			if w > 88 {
				w = 88
			}
			emptyStyle = emptyStyle.Width(w)
		}
		return emptyStyle.Render(hint)
	}

	// Only the LIVE tail is drawn. Everything before printedThrough was already
	// printed to terminal scrollback and must never be redrawn — see
	// scrollback.go. Blocks are joined by role, not uniformly: a blank line
	// between prose is breathing room; a blank line between consecutive tool
	// frames is a gap the eye has to jump, and a run is mostly tool frames.
	return m.renderRange(m.printedThrough, len(m.messages))
}

// renderMessage renders a single message
func (m Model) renderMessage(msg Message) string {
	// Tool call message (Claude Code style - no timestamp). The tool/bash line is
	// grey (like claude-source renders the command), not blue.
	if msg.Role == "tool_call" {
		toolStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colDim))

		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colDim))

		// Status marker: a SPINNING braille glyph while the tool runs, a white ⏺
		// when done, red ⏺ on a real error (plan-mode blocks get plan blue). The
		// running frame advances because thinkingTickMsg re-renders the viewport
		// while a tool is active. Done is a fact the result sets, not a guess
		// from empty output: `Bash(touch x)` answers nothing and is finished.
		var marker string
		switch toolState(msg) {
		case toolRunning:
			frame := brailleFrames[int(time.Now().UnixMilli()/spinnerStep.Milliseconds())%len(brailleFrames)]
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color(spinnerColor)).Render(frame) // running — spinner
		case toolPlanBlocked:
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color(colPlan)).Render("⏺") // plan-mode block — soft blue
		case toolFailed:
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color(colErr)).Render("⏺") // error — red
		default:
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color(colOK)).Render("⏺") // done — green
		}

		// Format tool call with input. The NAME is coloured by what kind of work
		// the tool does; its arguments stay dim, so a long transcript reads as a
		// column of colour-coded verbs rather than a wall of grey.
		view := viewFor(msg.ToolName)
		toolCall := view.Label(msg.ToolInput, m.width)
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(view.Colour())).Bold(true)
		styledCall := toolStyle.Render(toolCall)
		if i := strings.Index(toolCall, "("); i > 0 {
			styledCall = nameStyle.Render(toolCall[:i]) + toolStyle.Render(toolCall[i:])
		}

		var output strings.Builder
		output.WriteString(marker + " " + styledCall)

		// STREAMING TOOL (bash): while it runs, show a fixed-height, auto-following
		// tail of its live output (newest lines pinned to the bottom, older ones
		// scroll off the top) plus an Elapsed counter — so you watch a long or
		// tail-like command happen instead of staring at a spinner. On completion
		// LiveOutput is cleared and the compact result below takes over.
		if !msg.toolSettled() {
			dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
			if msg.LiveOutput != "" {
				lines := strings.Split(strings.TrimRight(msg.LiveOutput, "\n"), "\n")
				total := len(lines)
				if len(lines) > liveTailLines {
					lines = lines[len(lines)-liveTailLines:]
				}
				output.WriteString("\n" + dim.Render(fmt.Sprintf("  ⎿ live · %d lines", total)))
				for _, ln := range lines {
					if m.width > 10 && len([]rune(ln)) > m.width-6 {
						ln = string([]rune(ln)[:m.width-9]) + "..."
					}
					output.WriteString("\n" + dim.Render("     "+ln))
				}
			}
			if msg.Streamed {
				output.WriteString("\n" + dim.Render("  Elapsed: "+formatElapsed(time.Since(msg.Timestamp))))
			}
		}

		// The view decides how its own result reads — a change renders as a diff,
		// everything else falls through to the generic body below.
		viewBody := view.Body(ToolRender{
			Input:  msg.ToolInput,
			Output: msg.ToolOutput,
			Err:    msg.ToolError,
			Width:  m.width,
			Expand: m.expandTools,
		})
		if viewBody != "" {
			output.WriteString("\n" + viewBody)
		}

		// Add result if available
		if msg.ToolOutput != "" || msg.ToolError != "" {
			output.WriteString("\n")
			switch {
			case toolState(msg) == toolPlanBlocked:
				// Read-only guidance in plan mode — soft blue, no red "Error:".
				planStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colPlan))
				output.WriteString(planStyle.Render("  ⏸ " + msg.ToolError))
			case msg.ToolError != "":
				errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
				dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
				output.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", msg.ToolError)))
				if msg.ToolInput != "" {
					output.WriteString("\n")
					input := msg.ToolInput
					if len(input) > 150 {
						input = input[:147] + "..."
					}
					output.WriteString(dimStyle.Render(fmt.Sprintf("     Input: %s", input)))
				}
			case viewBody != "":
				// The view already said what happened — a diff, a checklist, a
				// summary. The raw result underneath is the model's working copy
				// (the echoed file, the "now run the build" instruction) and
				// repeating it buries the thing worth reading.
			case msg.ToolOutput == "":
				output.WriteString(resultStyle.Render("  (No content)"))
			case msg.ToolName == "edit_file":
				output.WriteString(renderEditResult(msg.ToolOutput))
			default:
				// Tool frames are EXPANDABLE (ctrl+o) — an 8-line cap hides exactly
				// what the babysitting user needs (a question's context, the todo
				// honesty note, a long build error).
				output.WriteString(resultBlock(m.formatToolOutput(msg.ToolName, msg.ToolOutput), 8, m.expandTools))
			}
		}

		// Took: wall time of a finished streaming tool call.
		if msg.Streamed && msg.ToolTook > 0 {
			took := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
			output.WriteString("\n" + took.Render("  Took: "+formatElapsed(msg.ToolTook)))
		}

		return output.String()
	}

	if msg.Role == "thinking" {
		// The model's reasoning, shown dim and italic under a small header so it
		// reads as context rather than as the answer. Kept SHORT: reasoning runs
		// to thousands of tokens and the answer is what the reader came for.
		const maxLines = 6
		lines := strings.Split(msg.Content, "\n")
		clipped := false
		if len(lines) > maxLines {
			lines, clipped = lines[:maxLines], true
		}
		body := strings.Join(lines, "\n")
		if clipped {
			body += "\n…"
		}
		head := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Bold(true).PaddingLeft(2).Render("✻ thinking")
		text := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Italic(true).PaddingLeft(4).Render(body)
		return head + "\n" + text
	}

	if msg.Role == "workflow" {
		// The DAG block (workflow_panel.go): drawn already, row by row, in
		// the palette's colours — no markdown pass over box glyphs.
		return msg.Content
	}

	if msg.Role == "system" {
		// Notes carry their verdict in the first glyph (✓ done, ⚠ heads-up,
		// 🛑/✗ stopped). Tint the whole line to match; anything unmarked stays
		// dim, so colour still means something.
		colour := colDim
		switch {
		case strings.HasPrefix(msg.Content, "✓"), strings.HasPrefix(msg.Content, "⚡"):
			colour = colOK
		case strings.HasPrefix(msg.Content, "⚠"), strings.HasPrefix(msg.Content, "⏳"):
			colour = colWarn
		case strings.HasPrefix(msg.Content, "🛑"), strings.HasPrefix(msg.Content, "✗"):
			colour = colErr
		}
		note := lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Italic(true).PaddingLeft(2)
		return note.Render(m.formatMarkdown(msg.Content))
	}

	if msg.Role == "user" {
		// Understated, Claude-style: a dim "> " prompt. The user's own text is
		// highlighted brighter (near-white) so it stands out from agent output.
		prefix := lipgloss.NewStyle().Foreground(lipgloss.Color(colAccent)).Bold(true).Render("> ")
		content := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true).Render(msg.Content)
		return prefix + content
	}

	// Assistant message (thinking is now in sticky bar, not in messages)
	if msg.IsThinking {
		// Skip rendering - thinking is shown in sticky bar
		return ""
	}

	// Regular assistant text — plain, indented, no leading glyph.
	//
	// Markdown is rendered while STREAMING too. It was briefly skipped for a
	// partial reply, on the theory that the renderer was too expensive to run
	// per token — but the real cost was rebuilding the WHOLE transcript each
	// repaint, which the block cache removes (render_cache.go: 1.5ms -> 90us).
	// Skipping it only showed the reader raw "**bold**" and backticks, because
	// the model writes markdown.
	body := m.renderMarkdown(msg.Content)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colText)).
		PaddingLeft(2).
		Render(body)
}

// watchdogStallTimeout is how long the TUI waits with NO run event before it
// assumes a completion event was lost and clears the spinner. It must exceed the
// longest silent stretch of a healthy run — a slow model writing a large tool
// call, which streams no text deltas — or it fires on live runs.
// liveTailLines is the fixed height of a streaming bash tool's live-output
// pane: the last N lines are shown, older ones scroll off the top.
const liveTailLines = 10

const watchdogStallTimeout = 8 * time.Minute

// pickDisplayKey chooses which input parameter to show in a tool-call label.
// It prefers a human-meaningful key, then falls back to the alphabetically-first
// key — never a random one, so the same call renders the same label every turn.
func pickDisplayKey(inputMap map[string]interface{}) string {
	// A null or empty value says nothing about the call — and %v on a nil
	// prints "<nil>", which rendered a live frame as skill(path: "<nil>")
	// (2026-08-31). Only keys with something to SHOW are candidates; a call
	// with nothing showable falls through to the bare (...) form.
	showable := func(k string) bool {
		v, ok := inputMap[k]
		if !ok || v == nil {
			return false
		}
		if s, isStr := v.(string); isStr && s == "" {
			return false
		}
		return true
	}
	for _, k := range []string{"query", "text", "message", "content", "name", "slug", "title", "path", "url", "command"} {
		if showable(k) {
			return k
		}
	}
	keys := make([]string, 0, len(inputMap))
	for k := range inputMap {
		if showable(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0]
	}
	return ""
}

// wrapText wraps text to fit within the specified width
func (m Model) wrapText(text string, width int) string {
	if width <= 0 {
		width = 80 // Default width
	}
	// ANSI-aware word wrap that ALSO hard-breaks any token longer than width.
	// The old version measured len(bytes) — which counts the escape codes
	// formatMarkdown injects for bold/links/code AND mis-sizes wide runes — and
	// never broke a long unbreakable word, so styled lines and long URLs/paths/
	// identifiers ran off the right edge. ansi.Wrap measures DISPLAY width and
	// breaks over-long tokens, which is what stops the overflow.
	return ansi.Wrap(text, width, "")
}

// Messages for tea.Msg

// assistantThinkingMsg starts the spinner and, when the model actually
// reasoned, carries that reasoning so it can be SHOWN. Before 2026-08-30 it was
// an empty struct: the gateway stripped the <think> block before anything could
// display it, so a reasoning model's work was invisible and a turn that was only
// reasoning looked like the model had done nothing.
type assistantThinkingMsg struct{ text string }

type assistantResponseMsg struct {
	content string
	// appended: content is only what did not stream — it goes under the
	// streamed text; replacing with it erased the whole answer (2026-10-04).
	appended bool
}

type contextUpdateMsg struct {
	tokens  int
	limit   int
	percent float64
	parts   contextParts
}

// contextParts is what the window holds, as the gateway counted it.
type contextParts struct {
	system, tools, conversation, toolOutput int
	compactAt                               int    // where compaction starts; 0 when unknown
	compactRule                             string // where compactAt comes from
	keepRecent                              int    // what /compact keeps word for word
}

// routeResultMsg is /model's answer: shown like a note, and the footer is
// refreshed at once so the rung it names is the rung it shows.
type routeResultMsg struct {
	summary string
	err     error
}

type SetProgramMsg struct {
	Program *tea.Program
}

type animationTickMsg time.Time

// executionFailedMsg is the gateway's execution.failed broadcast: the turn
// died (a stream EOF, a provider error) and will produce nothing more. It had
// NO handler — the bar counted a dead run forever and the error reached the
// reader nowhere (live, 2026-08-31 15:15).
type executionFailedMsg struct {
	errText string
}

type thinkingTickMsg time.Time

type assistantStreamingMsg struct {
	content string
}

type todoUpdateMsg struct {
	todos []TodoItem
}

type subagentStartMsg struct {
	sessionID string
	task      string
}

type subagentEndMsg struct {
	sessionID string
}

// runCompleteMsg signals the MAIN agent run finished (lifecycle complete/end/
// error). It's the safety net that clears the spinner when no assistant message
// follows — e.g. a tool-only turn whose final text was empty.
// Without it the spinner hung forever waiting for a message that never came.
type runCompleteMsg struct{ model string }

// compactionMsg signals the agent is trimming/summarizing its context window.
// active=true on start, false on completion; before/after are token counts
// (0 when unknown). Surfaced so the user sees WHY a turn paused.
// providerNoticeMsg is a line about the request in flight that is not the
// model's text (llm.Notify): a retry after silence or a refusal.
type providerNoticeMsg struct{ text string }

type compactionMsg struct {
	active bool
	before int
	after  int
}

// TodoItem represents a todo list item
type TodoItem struct {
	Content    string
	ActiveForm string
	Status     string
}

// tickAnimation returns a command that sends a tick message for spinner animation
func (m Model) tickAnimation() tea.Cmd {
	// One re-render per spinner step: ticking faster than the glyph changes
	// redraws the transcript for nothing.
	return tick(spinnerStep, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

// tickThinking returns a command that sends a tick message to refresh thinking indicator
func (m Model) tickThinking() tea.Cmd {
	return tick(1*time.Second, func(t time.Time) tea.Msg {
		return thinkingTickMsg(t)
	})
}

// renderMarkdown renders markdown with syntax highlighting for code blocks and basic formatting
func (m Model) renderMarkdown(text string) string {
	contentStyle := lipgloss.NewStyle().PaddingLeft(2)

	// Detect code blocks first (to avoid processing markdown inside them)
	// Match: ```lang\ncode``` or ```\ncode``` or even ```lang code```
	codeBlockRegex := regexp.MustCompile("```(\\w+)?\\s*\\n([\\s\\S]*?)```")

	parts := []string{}
	lastEnd := 0

	matches := codeBlockRegex.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		// Add text before code block (with markdown formatting)
		if match[0] > lastEnd {
			formatted := m.formatMarkdown(text[lastEnd:match[0]])
			parts = append(parts, contentStyle.Render(formatted))
		}

		// Extract language and code
		lang := ""
		if match[2] != -1 && match[3] != -1 {
			lang = text[match[2]:match[3]]
		}
		code := text[match[4]:match[5]]

		// Render code with syntax highlighting, then wrap so long lines don't blow
		// the box past the right edge. The box chrome (margin 2 + border 2 +
		// padding 4) eats ~8 cols, so wrap the content to width-12 for headroom.
		codeW := m.width - 12
		if codeW < 20 {
			codeW = 20
		}
		highlighted := m.wrapText(m.highlightCode(code, lang), codeW)

		// Add language label if present
		label := ""
		if lang != "" {
			labelStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colDim)).
				Italic(true).
				PaddingLeft(2)
			label = labelStyle.Render(lang) + "\n"
		}

		codeStyle := lipgloss.NewStyle().
			Background(lipgloss.Color(colFill)).
			Foreground(lipgloss.Color(colText)).
			Padding(0, 2).
			MarginLeft(2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colRule))

		parts = append(parts, label+codeStyle.Render(highlighted))

		lastEnd = match[1]
	}

	// Add remaining text (with markdown formatting)
	if lastEnd < len(text) {
		formatted := m.formatMarkdown(text[lastEnd:])
		parts = append(parts, contentStyle.Render(formatted))
	}

	if len(parts) == 0 {
		return contentStyle.Render(m.formatMarkdown(text))
	}

	return strings.Join(parts, "\n")
}

// pageURL turns a markdown link target into a clickable URL: a full http(s) URL
// passes through; anything else returns "" (rendered as plain styled text, not
// a link).
func (m Model) pageURL(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return ""
}

// osc8 wraps already-styled text in an OSC 8 hyperlink escape so supporting
// terminals make it clickable. Zero-width in terminals that ignore it.
//
// Inside tmux, OSC 8 is dropped unless wrapped in tmux's DCS passthrough — so
// when $TMUX is set we wrap the sequence in \ePtmux;…\e\\ (doubling every ESC in
// the payload, per the tmux protocol). The pane needs allow-passthrough on,
// which the TUI enables at startup. Outside tmux this is a plain OSC 8.
func osc8(url, text string) string {
	seq := "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
	if os.Getenv("TMUX") == "" {
		return seq
	}
	payload := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;" + payload + "\x1b\\"
}

// formatMarkdown applies basic markdown formatting (bold, italic, headers, lists)
// formatToolOutput renders a tool's result. Commands and code are shown as
// they are: markdown turned `grep --include="*.tsx" --include="*.ts"` into
// an italic `".tsx" --include="."` on screen (2026-09-28), and does the same
// to `func (m *Model) f(x *T)`. Only prose tools get markdown.
func (m Model) formatToolOutput(tool, text string) string {
	switch viewFor(tool).(type) {
	case commandView, readView, judgedView:
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			lines[i] = m.wrapText(l, m.width-8)
		}
		return strings.Join(lines, "\n")
	}
	return m.formatMarkdown(text)
}

func (m Model) formatMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var formatted []string

	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colText))
	italicStyle := lipgloss.NewStyle().Italic(true)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colWrite))
	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colRead)).Underline(true)

	for _, line := range lines {
		// A line that opens with a verdict glyph is tinted to match it.
		if trimmed := strings.TrimLeft(line, " \t"); trimmed != "" {
			var verdict string
			switch {
			case strings.HasPrefix(trimmed, "✓"), strings.HasPrefix(trimmed, "⚡"):
				verdict = colOK
			case strings.HasPrefix(trimmed, "⚠"), strings.HasPrefix(trimmed, "⏳"):
				verdict = colWarn
			case strings.HasPrefix(trimmed, "🛑"), strings.HasPrefix(trimmed, "✗"):
				verdict = colErr
			}
			if verdict != "" {
				formatted = append(formatted, lipgloss.NewStyle().
					Foreground(lipgloss.Color(verdict)).
					Render(m.wrapText(stripInlineMarks(line), m.width-8)))
				continue
			}
		}
		// Headers
		if strings.HasPrefix(line, "# ") {
			formatted = append(formatted, m.wrapText(headerStyle.Render(strings.TrimPrefix(line, "# ")), m.width-8))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			formatted = append(formatted, m.wrapText(headerStyle.Render(strings.TrimPrefix(line, "## ")), m.width-8))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			formatted = append(formatted, m.wrapText(headerStyle.Render(strings.TrimPrefix(line, "### ")), m.width-8))
			continue
		}

		// Lists
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = "• " + line[2:]
		}

		// Links: [label](target). Show just the styled label (not the raw
		// "(slug.md)" clutter) and make it an OSC 8 terminal hyperlink so a full
		// URL is clickable. Done before bold/italic so the brackets/parens
		// aren't mangled. Terminals without OSC 8 still show the styled label.
		linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
		line = linkRegex.ReplaceAllStringFunc(line, func(match string) string {
			sm := linkRegex.FindStringSubmatch(match)
			label, target := sm[1], sm[2]
			styled := linkStyle.Render(label)
			if url := m.pageURL(target); url != "" {
				return osc8(url, styled)
			}
			return styled
		})

		// Bold: **text**
		boldRegex := regexp.MustCompile(`\*\*([^\*]+)\*\*`)
		line = boldRegex.ReplaceAllStringFunc(line, func(match string) string {
			content := strings.Trim(match, "*")
			return boldStyle.Render(content)
		})

		// Italic: *text*
		italicRegex := regexp.MustCompile(`\*([^\*]+)\*`)
		line = italicRegex.ReplaceAllStringFunc(line, func(match string) string {
			content := strings.Trim(match, "*")
			return italicStyle.Render(content)
		})

		// Inline code: `text`
		inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
		line = inlineCodeRegex.ReplaceAllStringFunc(line, func(match string) string {
			content := strings.Trim(match, "`")
			codeStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(colFill)).
				Foreground(lipgloss.Color(colWarn))
			return codeStyle.Render(content)
		})

		// Wrap text to viewport width — but NOT lines containing an OSC 8
		// hyperlink: wrapText measures byte length and splits on spaces, which
		// would cut through the invisible escape sequence and break the link.
		// Link lines are short (a citation); let the terminal soft-wrap them.
		wrappedLine := line
		if !strings.Contains(line, "\x1b]8;;") {
			wrappedLine = m.wrapText(line, m.width-8) // Leave margin for padding
		}
		// Paths and URLs become clickable AFTER wrapping, so the link escape
		// is never split across lines (model_links.go).
		formatted = append(formatted, linkifyLine(wrappedLine))
	}

	return strings.Join(formatted, "\n")
}

// stripInlineMarks removes the markdown emphasis marks from a line that is
// styled wholesale (a verdict line), so it doesn't show literal ** or `.
func stripInlineMarks(line string) string {
	line = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(line, "$1")
	line = strings.ReplaceAll(line, "`", "")
	return line
}

// highlightCode applies syntax highlighting to code
func (m Model) highlightCode(code, lang string) string {
	if lang == "" {
		lang = "text"
	}

	var highlighted strings.Builder
	err := quick.Highlight(&highlighted, code, lang, "terminal256", "monokai")
	if err != nil {
		// Fallback to plain text
		return code
	}

	return highlighted.String()
}
