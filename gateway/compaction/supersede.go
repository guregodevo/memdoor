package compaction

import (
	"strings"

	"memdoor/pkg/llm"
)

// ElideSpent gives up tool output nothing will read again, and costs no
// model call:
//   - a call made again later (the same tool, the same input: a file read
//     twice, `go test` run twice) supersedes the earlier one's result, which
//     is stubbed; the call stays, so the model knows it looked;
//   - before the current turn, a call that failed, or a search that found
//     nothing, is dropped with its result: the turn after it either fixed
//     it or moved on, and a failed call's input can be a whole patch.
//
// Every call and result of the current turn (from the last request on)
// stays as it was.
func ElideSpent(messages []llm.MessageParam) ([]llm.MessageParam, bool) {
	turnStart := lastRequest(messages)
	type callInfo struct{ key, name string }
	calls := map[string]callInfo{} // tool_use id → call
	last := map[string]string{}    // call key → id of its latest call
	for _, m := range messages {
		for _, b := range m.Content {
			if b.OfToolUse != nil {
				key := llm.ToolCallKey(b.OfToolUse.Name, b.OfToolUse.Input)
				calls[b.OfToolUse.ID] = callInfo{key, b.OfToolUse.Name}
				last[key] = b.OfToolUse.ID
			}
		}
	}
	// Which results are superseded, which calls are spent.
	superseded, drop := map[string]bool{}, map[string]bool{}
	for i, m := range messages {
		for _, b := range m.Content {
			r := b.OfToolResult
			if r == nil || isStub(r) {
				continue
			}
			c, ok := calls[r.ToolUseID]
			switch {
			case i < turnStart && ok && (r.IsError || foundNothing(c.name, r)):
				drop[r.ToolUseID] = true
			case ok && last[c.key] != r.ToolUseID:
				superseded[r.ToolUseID] = true
			}
		}
	}
	if len(superseded)+len(drop) == 0 {
		return messages, false
	}
	out := make([]llm.MessageParam, 0, len(messages))
	for _, m := range messages {
		blocks := make([]llm.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch {
			case b.OfToolUse != nil && drop[b.OfToolUse.ID], b.OfToolResult != nil && drop[b.OfToolResult.ToolUseID]:
				continue
			case b.OfToolResult != nil && superseded[b.OfToolResult.ToolUseID]:
				stub := *b.OfToolResult
				stub.Content = []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{Type: "text", Text: supersededStub}}}
				b = llm.ContentBlockParamUnion{OfToolResult: &stub}
			}
			blocks = append(blocks, b)
		}
		if len(blocks) > 0 {
			out = append(out, llm.MessageParam{Role: m.Role, Content: blocks})
		}
	}
	return out, true
}

// searchTools are the tools whose empty answer means "nothing there".
var searchTools = map[string]bool{"grep": true, "jgrep": true, "glob": true, "locate": true}

// foundNothing reports whether r is a search's (or a silent command's)
// empty answer: "", "No matches.", "No files found matching pattern: …",
// "(no output)".
func foundNothing(tool string, r *llm.ToolResultBlockParam) bool {
	var text string
	for _, c := range r.Content {
		if c.OfText != nil {
			text += c.OfText.Text
		}
	}
	text = strings.TrimSpace(text)
	switch {
	case text == "(no output)":
		return true
	case !searchTools[tool]:
		return false
	}
	return text == "" || text == "No matches." || strings.HasPrefix(text, "No files found matching pattern")
}

// The texts a stubbed result carries.
const (
	supersededStub = "(superseded: the same call was made again later — its latest result is below)"
	elidedStub     = "(old tool output elided — " // elideOldToolResults, followed by its size
)

// isStub reports whether a result was already stubbed.
func isStub(r *llm.ToolResultBlockParam) bool {
	if len(r.Content) != 1 || r.Content[0].OfText == nil {
		return false
	}
	t := r.Content[0].OfText.Text
	return t == supersededStub || strings.HasPrefix(t, elidedStub)
}

// lastRequest is the index of the last message the person typed (a user
// message with text, not a tool result); len(messages) when there is none.
func lastRequest(messages []llm.MessageParam) int {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != llm.MessageParamRoleUser || len(m.Content) == 0 {
			continue
		}
		if m.Content[0].OfText != nil {
			return i
		}
	}
	return len(messages)
}

// KeepForTokens is how many of the last messages fit in budget tokens, at
// least minKeep; the count never splits a tool call from its result (the
// compaction's split point moves to keep them together).
func (c *Compactor) KeepForTokens(messages []llm.MessageParam, budget, minKeep int) int {
	used, keep := 0, 0
	for i := len(messages) - 1; i >= 0; i-- {
		used += c.tokenCounter.CountConversationTokens(messages[i : i+1])
		if used > budget {
			break
		}
		keep++
	}
	return max(keep, min(minKeep, len(messages)))
}
