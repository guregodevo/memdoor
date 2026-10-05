package compaction

import (
	"fmt"
	"log/slog"
	"strings"

	"memdoor/gateway/context"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// CompactionType is why a conversation must shrink. When is decided in one
// place: the preflight check (context/preflight.go) read by
// compactionDecision (gateway/agent_runtime_inference.go); how, by one
// ladder (gateway/agent_runtime_fit.go).
type CompactionType int

const (
	CompactionNone      CompactionType = iota
	CompactionThreshold                // past the threshold: shrink before the turn starts
	CompactionOverLimit                // past the window: the request cannot be sent
)

// Compactor reshapes a conversation to fit a window. It decides nothing
// and sends nothing: see CompactionType.
type Compactor struct {
	tokenCounter *context.TokenCounter
	log          *logs.EventLogger
}

// NewCompactor creates a compactor counting tokens for model.
func NewCompactor(model string, verbose bool) (*Compactor, error) {
	if _, err := context.GetModelLimits(model); err != nil {
		return nil, err
	}
	return &Compactor{
		tokenCounter: context.NewTokenCounter(model, verbose),
		log:          logs.New("Agent"),
	}, nil
}

// CompactionResult contains the result of a compaction operation
type CompactionResult struct {
	BeforeTokens    int     `json:"before_tokens"`
	AfterTokens     int     `json:"after_tokens"`
	MessagesBefore  int     `json:"messages_before"`
	MessagesAfter   int     `json:"messages_after"`
	MessagesRemoved int     `json:"messages_removed"`
	MessagesKept    int     `json:"messages_kept"`
	Summarized      bool    `json:"summarized"`
	SummaryText     string  `json:"summary_text,omitempty"`
	TokensSaved     int     `json:"tokens_saved"`
	PercentageSaved float64 `json:"percentage_saved"`
}

// Compact replaces all but the last keepRecent messages by one summary
// message: what was asked (when the kept messages hold no request), a
// digest of what was asked and answered, and the receipts of the tool calls
// dropped. With keepRecent or fewer messages it returns them unchanged.
// It sends nothing and emits nothing: the caller reports the reshape once.
func (c *Compactor) Compact(messages []llm.MessageParam, keepRecent int) (*CompactionResult, []llm.MessageParam) {
	return c.compact(messages, keepRecent, "")
}

// CompactWritten is Compact with a summary the model wrote of the messages
// it drops (/compact, /handoff) in place of the digest; the receipts stay.
func (c *Compactor) CompactWritten(messages []llm.MessageParam, keepRecent int, written string) (*CompactionResult, []llm.MessageParam) {
	return c.compact(messages, keepRecent, written)
}

func (c *Compactor) compact(messages []llm.MessageParam, keepRecent int, written string) (*CompactionResult, []llm.MessageParam) {
	beforeTokens := c.tokenCounter.CountConversationTokens(messages)
	unchanged := &CompactionResult{
		BeforeTokens: beforeTokens, AfterTokens: beforeTokens,
		MessagesBefore: len(messages), MessagesAfter: len(messages), MessagesKept: len(messages),
	}
	if keepRecent < 1 || len(messages) <= keepRecent {
		return unchanged, messages
	}
	// Never between a tool call and its result.
	splitIndex := findSafeSplitPoint(messages, len(messages)-keepRecent)
	if splitIndex <= 0 {
		return unchanged, messages
	}
	oldMessages := messages[:splitIndex]
	recentMessages := messages[splitIndex:]

	summary := c.createSummary(oldMessages, written)

	// The TASK survives compaction. The pruned window is the oldest messages,
	// and the oldest message of a turn is the user's request; with keep=6 a
	// doer agent lost it after its first few tool calls and reasoned "the
	// user's request was pruned… the deliverable is probably…" — then did
	// work nobody asked for (live 2026-09-02: asked to tighten a video and
	// report, it produced three shorts). The request is folded into the head
	// of the summary message so it is the first thing the model reads.
	//
	// Only when the kept messages hold no request: a conversation carried
	// across turns keeps its latest request, and its OLDEST one labelled
	// "your task" would send the model back to finished work.
	if task := firstTaskText(oldMessages); task != "" && firstTaskText(recentMessages) == "" {
		summary = taskHeader + task + "\n\n" + summary
	}
	summary += receiptsText(oldMessages)

	compacted := make([]llm.MessageParam, 0, len(recentMessages)+1)
	compacted = append(compacted, llm.NewUserMessage(llm.NewTextBlock(summary)))
	compacted = append(compacted, recentMessages...)

	afterTokens := c.tokenCounter.CountConversationTokens(compacted)
	saved := beforeTokens - afterTokens
	result := &CompactionResult{
		BeforeTokens:    beforeTokens,
		AfterTokens:     afterTokens,
		MessagesBefore:  len(messages),
		MessagesAfter:   len(compacted),
		MessagesRemoved: len(oldMessages),
		MessagesKept:    len(recentMessages),
		Summarized:      true,
		SummaryText:     summary,
		TokensSaved:     saved,
		PercentageSaved: float64(saved) / float64(max(beforeTokens, 1)) * 100.0,
	}
	c.log.Debug("Compacted",
		slog.Int("messages_before", len(messages)), slog.Int("messages_after", len(compacted)),
		slog.Int("tokens_before", beforeTokens), slog.Int("tokens_after", afterTokens))
	return result, compacted
}

// createSummary stands in for the messages a compaction drops: how many,
// then the model's summary of them when one was written, else a digest of
// what was asked and answered. Both are carried forward through later
// compactions: a written summary verbatim, digest lines one by one. No
// model call here — the receipts of the work follow it.
func (c *Compactor) createSummary(messages []llm.MessageParam, written string) string {
	if len(messages) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, summaryHead+": %d messages (~%dK tokens) compacted to fit the window]\n",
		len(messages), c.tokenCounter.CountConversationTokens(messages)/1000)
	if written = strings.TrimSpace(written); written != "" {
		// The model read any earlier summary in the messages it summarized.
		sb.WriteString(writtenHeader + "\n" + written + "\n")
		return sb.String()
	}
	if earlier := carriedWritten(messages); earlier != "" {
		sb.WriteString(writtenHeader + "\n" + earlier + "\n")
	}
	if lines := dialogueDigest(messages); len(lines) > 0 {
		sb.WriteString(digestHeader + "\n" + strings.Join(lines, "\n") + "\n")
	}
	return sb.String()
}

// writtenHeader opens a summary the model wrote (/compact, /handoff).
const writtenHeader = "SUMMARY OF THE EARLIER CONVERSATION (written by the model):"

// carriedWritten is the written summary inside an earlier compaction's
// placeholder among messages, up to the next section, or "".
func carriedWritten(messages []llm.MessageParam) string {
	for _, m := range messages {
		for _, b := range m.Content {
			if b.OfText == nil {
				continue
			}
			t := b.OfText.Text
			i := strings.Index(t, writtenHeader)
			if i < 0 {
				continue
			}
			t = t[i+len(writtenHeader):]
			for _, next := range []string{digestHeader, receiptsHeader} {
				if j := strings.Index(t, next); j >= 0 {
					t = t[:j]
				}
			}
			return strings.TrimSpace(t)
		}
	}
	return ""
}

// summaryHead opens a compaction's placeholder; it is never a request.
const summaryHead = "[Earlier in this conversation"

// digestHeader opens the list of what was asked and answered before a
// compaction; a later compaction reads its lines back out.
const digestHeader = "ASKED AND ANSWERED EARLIER (oldest first):"

const (
	digestLineChars = 200
	digestMaxLines  = 40
)

// dialogueDigest is one clipped line per request and per answer in the
// dropped messages, with an earlier digest carried forward.
func dialogueDigest(dropped []llm.MessageParam) []string {
	var lines []string
	for _, m := range dropped {
		for _, b := range m.Content {
			if b.OfText == nil {
				continue
			}
			text := b.OfText.Text
			if i := strings.Index(text, digestHeader); i >= 0 {
				for _, l := range strings.Split(text[i+len(digestHeader):], "\n") {
					if strings.HasPrefix(l, "- ") {
						lines = append(lines, l)
					}
				}
				continue
			}
			if strings.HasPrefix(text, summaryHead) || strings.HasPrefix(text, taskHeader) ||
				strings.Contains(text, receiptsHeader) || strings.TrimSpace(text) == "" {
				continue
			}
			who := "asked"
			if m.Role == llm.MessageParamRoleAssistant {
				who = "answered"
			}
			lines = append(lines, "- "+who+": "+clip(text, digestLineChars))
		}
	}
	if len(lines) > digestMaxLines {
		lines = lines[len(lines)-digestMaxLines:]
	}
	return lines
}

// CountConversationTokens counts tokens in conversation messages
func (c *Compactor) CountConversationTokens(messages []llm.MessageParam) int {
	return c.tokenCounter.CountConversationTokens(messages)
}

// CountSystemPromptTokens counts tokens in system prompt
func (c *Compactor) CountSystemPromptTokens(systemPrompt string) int {
	return c.tokenCounter.CountSystemPromptTokens(systemPrompt)
}

// CountToolSchemaTokens counts tokens in tool schemas
func (c *Compactor) CountToolSchemaTokens(toolSchemas []llm.ToolParam) int {
	return c.tokenCounter.CountToolSchemaTokens(toolSchemas)
}

// findSafeSplitPoint finds a split point that doesn't break assistant-user message pairs
// Pattern: OpenClaw approach - prevent orphaned tool_result blocks
// Anthropic requires that assistant messages with tool_use be immediately followed by user messages with tool_result
func findSafeSplitPoint(messages []llm.MessageParam, desiredSplit int) int {
	if desiredSplit <= 0 || desiredSplit >= len(messages) {
		return desiredSplit
	}

	// Check if split would break an assistant-user pair
	// If the message at desired split is assistant with > 0 content, keep the next user message too
	if desiredSplit < len(messages) && messages[desiredSplit-1].Role == "assistant" {
		// Check if next message is user (likely contains tool_result)
		if desiredSplit < len(messages) && messages[desiredSplit].Role == "user" {
			// Move split point forward to include the user message
			return desiredSplit + 1
		}
	}

	return desiredSplit
}

// keepIntactToolResults is how many trailing messages keep their tool-result
// bodies verbatim during elision — the model is actively working from those.
const keepIntactToolResults = 6

// elideToolResultThreshold: outputs at or under this length are left alone —
// stubbing a two-line result saves nothing and loses signal.
const elideToolResultThreshold = 400

// ShedToolResults is elideOldToolResults for callers outside this package:
// the last rung of the overflow ladder, where a turn that still does not fit
// after emergency compaction sheds its tool payloads instead of dying.
// keepLast 0 sheds every one of them.
func ShedToolResults(messages []llm.MessageParam, keepLast int) ([]llm.MessageParam, bool) {
	return elideOldToolResults(messages, keepLast)
}

// elideOldToolResults replaces large tool-result bodies in all but the last
// keepLast messages with a short stub. Structure (roles, tool_use/tool_result
// pairing) is preserved — only the payload text shrinks, so the provider-side
// prompt cache prefix stays valid up to the first elided message.
func elideOldToolResults(messages []llm.MessageParam, keepLast int) ([]llm.MessageParam, bool) {
	cutoff := len(messages) - keepLast
	if cutoff <= 0 {
		return messages, false
	}
	changed := false
	out := make([]llm.MessageParam, len(messages))
	copy(out, messages)
	for i := 0; i < cutoff; i++ {
		var newBlocks []llm.ContentBlockParamUnion
		blockChanged := false
		for _, b := range out[i].Content {
			if b.OfToolResult != nil {
				total := 0
				for _, cu := range b.OfToolResult.Content {
					if cu.OfText != nil {
						total += len(cu.OfText.Text)
					}
				}
				if total > elideToolResultThreshold {
					stub := *b.OfToolResult
					stub.Content = []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{
						Type: "text",
						Text: fmt.Sprintf(elidedStub+"%d chars; recall {\"id\": %q} returns it word for word)", total, b.OfToolResult.ToolUseID),
					}}}
					newBlocks = append(newBlocks, llm.ContentBlockParamUnion{OfToolResult: &stub})
					blockChanged = true
					continue
				}
			}
			newBlocks = append(newBlocks, b)
		}
		if blockChanged {
			out[i] = llm.MessageParam{Role: out[i].Role, Content: newBlocks}
			changed = true
		}
	}
	return out, changed
}

// taskHeader opens the compaction placeholder; the task follows it up to the
// first blank line, so a later compaction can read it back out.
const taskHeader = "YOUR TASK (kept through compaction — this is what the user asked for): "

// firstTaskText returns the text of the earliest user message that is a
// plain request (text only, not a tool_result), or the task carried by an
// earlier compaction placeholder, or "" when there is none.
func firstTaskText(messages []llm.MessageParam) string {
	for _, m := range messages {
		if m.Role != llm.MessageParamRoleUser {
			continue
		}
		var text string
		plain := true
		for _, b := range m.Content {
			switch {
			case b.OfText != nil:
				text += b.OfText.Text
			case b.OfToolResult != nil:
				plain = false
			}
		}
		text = strings.TrimSpace(text)
		if !plain || text == "" {
			continue
		}
		if strings.HasPrefix(text, summaryHead) {
			continue
		}
		if strings.HasPrefix(text, taskHeader) {
			task := strings.TrimPrefix(text, taskHeader)
			if i := strings.Index(task, "\n\n"); i >= 0 {
				task = task[:i]
			}
			return strings.TrimSpace(task)
		}
		return text
	}
	return ""
}

// receiptsHeader opens the ledger of tool results a compaction dropped. The
// numbers a tool returned are the truth of the turn; without them the model
// re-derives them from memory (the cut "from 60–90s" was 78–108s, "31.5s →
// 30.0s" was 34s → 31.5s, live 2026-09-03) and re-verifies files it has
// already verified.
const receiptsHeader = "RECEIPTS (tool results dropped by compaction — these are the real numbers; do not re-derive or re-verify them):"

const (
	receiptsKeep      = 12  // most recent dropped tool calls carried
	receiptInputChars = 100 // of the call's input JSON
	receiptOutChars   = 160 // of the result
)

// receiptsText renders the ledger for dropped messages: receipts an earlier
// compaction carried, then one line per dropped tool call and its result.
func receiptsText(dropped []llm.MessageParam) string {
	lines := toolReceipts(dropped)
	if len(lines) == 0 {
		return ""
	}
	return "\n" + receiptsHeader + "\n" + strings.Join(lines, "\n") + "\n"
}

func toolReceipts(dropped []llm.MessageParam) []string {
	var lines []string
	calls := map[string]string{} // tool_use id → "name(input)"
	for _, m := range dropped {
		for _, b := range m.Content {
			switch {
			case b.OfText != nil && m.Role == llm.MessageParamRoleUser:
				// An earlier placeholder: carry its ledger forward.
				if i := strings.Index(b.OfText.Text, receiptsHeader); i >= 0 {
					for _, l := range strings.Split(b.OfText.Text[i+len(receiptsHeader):], "\n") {
						if strings.HasPrefix(l, "- ") {
							lines = append(lines, l)
						}
					}
				}
			case b.OfToolUse != nil:
				calls[b.OfToolUse.ID] = b.OfToolUse.Name + "(" + clip(compactJSON(b.OfToolUse.Input), receiptInputChars) + ")"
			case b.OfToolResult != nil:
				call, ok := calls[b.OfToolResult.ToolUseID]
				if !ok {
					continue
				}
				var out strings.Builder
				for _, c := range b.OfToolResult.Content {
					if c.OfText != nil {
						out.WriteString(c.OfText.Text)
					}
				}
				status := " → "
				if b.OfToolResult.IsError {
					status = " → ERROR: "
				}
				lines = append(lines, "- "+call+status+clip(out.String(), receiptOutChars))
			}
		}
	}
	if len(lines) > receiptsKeep {
		lines = lines[len(lines)-receiptsKeep:]
	}
	return lines
}

// compactJSON strips the quotes and braces that carry no information for a
// one-line receipt: {"path":"a.mp4","start":78} → path:a.mp4 start:78.
func compactJSON(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	s = strings.NewReplacer("{", "", "}", "", "\"", "", "\n", " ").Replace(s)
	s = strings.ReplaceAll(s, ",", " ")
	return strings.Join(strings.Fields(s), " ")
}

// clip collapses whitespace and cuts to n characters with an ellipsis.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
