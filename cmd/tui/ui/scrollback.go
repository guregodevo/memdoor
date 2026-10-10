package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// scrollback: settled messages are printed ONCE to the terminal and never
// touched again.
//
// The TUI used to run in the altscreen with the whole transcript inside a
// viewport, which means every frame handed the terminal every message —
// hundreds of lines repainted to change the last one. Memoising the render
// (render_cache.go) removed the markdown cost but not this one: the block
// cache stops us REBUILDING the text, not the terminal REDRAWING it.
//
// So the transcript is split in two. Everything old enough to be final is
// flushed to real terminal scrollback with tea.Println — print-and-forget,
// outside the managed frame, never redrawn. Only the live tail stays in the
// viewport, and only as much of it as fits on screen.
//
// The cost is that printed output is frozen: it does not reflow on resize and
// ctrl+o cannot expand it. That is the same bargain the terminal itself makes
// with its history, and it is what buys a repaint cost that stops growing.

// minLive is how many messages stay in the managed viewport no matter what.
// The newest one must: it is still growing, and freezing a message that can
// still change is what puts two versions of it in the scrollback.
const minLive = 1

// headroom is the rows left empty below the view. An inline (non-altscreen)
// program whose output is exactly as tall as the terminal makes the terminal
// scroll a line on every repaint, and each scroll leaves the PREVIOUS version
// of the tail behind in the history — the same message ends up in the
// scrollback twice, at two different lengths. Measured live 2026-08-30 on a
// 40-row pane: a 40-line view produced 39 lines of tmux history and a thinking
// block printed twice. The view has to stay strictly shorter than the terminal.
const headroom = 2

// liveBudget is how many rows the transcript may occupy: the terminal, less
// the chrome drawn around it, less the headroom. It mirrors View's arithmetic
// deliberately, and conservatively — the variable-height bars (activity,
// todos, autocomplete, picker) are not subtracted, so this over-estimates and
// the viewport's own clamp catches the difference. What matters is that it is
// bounded at roughly one screen instead of growing with the conversation.
func (m Model) liveBudget() int {
	if m.height <= 0 {
		return 0 // no size yet — nothing to measure against, so flush nothing
	}
	b := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderFooter()) - 2 - headroom
	if b < 1 {
		b = 1
	}
	return b
}

// settledCount is how many leading messages get frozen into scrollback. Two
// rules, and the tighter one wins.
//
// The first is correctness: a message that could still change must not be
// printed, and scrollback order has to match transcript order — so the first
// message that can still move blocks everything behind it, even messages that
// are themselves final.
//
// The second is the actual point: whatever does not fit in liveBudget rows is
// flushed. Counting MESSAGES instead of rows does not work — eight tool frames
// can be four lines or four hundred, and the version of this that counted
// messages let the live region grow to full terminal height and tear.
func (m Model) settledCount() int {
	// Before the first WindowSizeMsg there is no budget to measure against, and
	// an unmeasured budget reads as "nothing fits" — which would flush the whole
	// transcript to scrollback before a single frame has been drawn.
	if m.height <= 0 {
		return m.printedThrough
	}

	final := 0
	for i := 0; i < len(m.messages); i++ {
		if !settled(m.messages[i]) || m.spawnLive(m.messages[i]) {
			break
		}
		final = i + 1
	}
	if final <= m.printedThrough {
		return m.printedThrough
	}

	// Walk back from the newest, keeping what fits. +1 per block for the blank
	// line the join puts between them.
	budget, keep := m.liveBudget(), 0
	for i := len(m.messages) - 1; i >= 0; i-- {
		budget -= lipgloss.Height(m.renderBlock(i, m.messages[i])) + 1
		if budget < 0 && keep >= minLive {
			break
		}
		keep++
	}

	n := len(m.messages) - keep
	if n > final {
		n = final
	}
	if n < m.printedThrough {
		n = m.printedThrough
	}
	return n
}

// settled reports whether a message has reached its final rendered form.
func settled(msg Message) bool {
	if msg.Streaming {
		return false
	}
	// A tool frame renders a SPINNER until its result lands, so it is still
	// animating and must not be frozen into scrollback.
	if msg.Role == "tool_call" && !msg.toolSettled() {
		return false
	}
	return true
}

// nextFlush is the flush decision, separated from performing it: it returns the
// text to print and the new watermark, and changes nothing. tea.Println hides
// its payload in an unexported message, so this is also the only seam a test
// can assert the printed bytes through.
func (m Model) nextFlush() (chunk string, through int) {
	n := m.settledCount()
	if n <= m.printedThrough {
		return "", m.printedThrough
	}
	chunk = m.renderRange(m.printedThrough, n)
	if chunk == "" {
		return "", n
	}
	// The seam: renderMessages joins blocks with a blank line, and that join
	// disappears at the boundary between what was printed and what is still
	// live. Re-add it unless the seam is between two tool frames, which sit
	// flush against each other.
	if n < len(m.messages) && !(m.messages[n].Role == "tool_call" && m.messages[n-1].Role == "tool_call") {
		chunk += "\n"
	}
	return chunk, n
}

// flushSettled prints the newly-settled prefix to the terminal and advances the
// watermark. It returns the updated model and the print command, which the
// caller batches with whatever else the update produced.
func (m Model) flushSettled() (Model, tea.Cmd) {
	chunk, through := m.nextFlush()
	if through <= m.printedThrough {
		return m, nil
	}

	// Evict what was just frozen: those indices are never rendered again, so
	// keeping their text alive would grow the cache with the conversation.
	for i := m.printedThrough; i < through; i++ {
		delete(m.blocks, i)
	}
	m.printedThrough = through
	// What was just printed must leave the viewport too, or it keeps the
	// whole transcript and the reader is stuck at its top (2026-09-28).
	if m.viewport.Height > 0 {
		following := m.viewport.AtBottom()
		m.viewport.SetContent(m.renderMessages())
		if following || m.viewport.AtBottom() {
			m.gotoBottom()
		}
	}

	if chunk == "" {
		return m, nil
	}
	return m, tea.Println(chunk)
}

// renderRange renders messages[lo:hi] with the same separator rules
// renderMessages uses, so a block reads identically whether it was printed to
// scrollback or drawn in the viewport.
func (m Model) renderRange(lo, hi int) string {
	var blocks []string
	var roles []string
	for i := lo; i < hi && i < len(m.messages); i++ {
		block := m.renderBlock(i, m.messages[i])
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
		roles = append(roles, m.messages[i].Role)
	}
	return joinBlocks(blocks, roles)
}

// joinBlocks is the transcript's separator rule, in one place: a blank line
// between prose, a single newline between consecutive tool frames.
func joinBlocks(blocks, roles []string) string {
	if len(blocks) == 0 {
		return ""
	}
	out := blocks[0]
	for i := 1; i < len(blocks); i++ {
		sep := "\n\n"
		if roles[i] == "tool_call" && roles[i-1] == "tool_call" {
			sep = "\n"
		}
		out += sep + blocks[i]
	}
	return out
}

// closeOrphanFrames closes tool frames whose result will never arrive (an
// interrupt, a lost connection). A frame left spinning is never settled, and
// the first unsettled frame holds every later message out of scrollback: the
// live region grew with the conversation and scrolling to the latest crawled.
func (m *Model) closeOrphanFrames(reason string) {
	for i := range m.messages {
		if m.messages[i].Role == "tool_call" && !m.messages[i].toolSettled() {
			m.messages[i].ToolError = reason
			m.messages[i].LiveOutput = ""
		}
	}
	m.settleStreaming()
}
