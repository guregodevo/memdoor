package ui

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// blockCache memoises the rendered form of each message.
//
// renderMessages rebuilt EVERY message on every call, and streaming calls it
// once per token: a transcript of ~90 messages at 45 tokens/second meant
// thousands of renders a second — glamour markdown for every settled reply and
// a full diff for every tool frame — to change one line at the bottom.
//
// A message renders once and is reused until something about it changes. The
// cache is a map, so it survives Model being copied by value the way Bubble Tea
// passes it around.
type blockCache map[int]cachedBlock

type cachedBlock struct {
	key  uint64
	text string
}

// blockKey fingerprints everything renderMessage actually reads. If none of it
// changed, the previous render is still correct — and if any of it did, the key
// changes and the block is rebuilt. Width is included because the render wraps
// to it.
func blockKey(msg Message, width int, expanded bool, live string) uint64 {
	h := fnv.New64a()
	write := func(s string) { _, _ = h.Write([]byte(s)); _, _ = h.Write([]byte{0}) }
	write(msg.Role)
	write(msg.Content)
	write(msg.ToolName)
	write(msg.ToolInput)
	write(msg.ToolOutput)
	write(msg.ToolError)
	write(msg.LiveOutput)
	// ToolDone, or a tool that finishes with nothing to say is cached as the
	// spinner it was: `Bash(touch x)` has the same output (none) and the same
	// error (none) before and after its result, so the key never moved and the
	// frame spun for the rest of the session (live 2026-10-01).
	write(strconv.FormatBool(msg.ToolDone))
	write(strconv.FormatBool(msg.Streamed))
	write(msg.ToolTook.String())
	write(strconv.FormatBool(msg.IsThinking))
	write(strconv.FormatBool(msg.Streaming))
	write(strconv.Itoa(width))
	// expandTools is model state that renderMessage reads (ctrl+o), not message
	// state — but it changes what a tool frame renders, so it belongs in the key.
	write(strconv.FormatBool(expanded))
	// What the window knows about the frame beyond the message itself: a
	// spawned run's trail (toolview_frames.go). Left out, the frame stayed
	// "starting …" for the whole child run (live 2026-10-11).
	write(live)
	return h.Sum64()
}

// renderBlock returns the rendered message, from cache when nothing about it
// has changed.
func (m Model) renderBlock(i int, msg Message) string {
	// A frame that is still running draws a spinner whose glyph comes from the
	// clock: caching it would freeze the animation.
	if msg.Role == "tool_call" && !msg.toolSettled() {
		return strings.TrimRight(m.renderMessage(msg), "\n \t")
	}
	key := blockKey(msg, m.width, m.expandTools, m.trailKey(msg))
	if m.blocks != nil {
		if got, ok := m.blocks[i]; ok && got.key == key {
			return got.text
		}
	}
	text := strings.TrimRight(m.renderMessage(msg), "\n \t")
	if m.blocks != nil {
		m.blocks[i] = cachedBlock{key: key, text: text}
	}
	return text
}
