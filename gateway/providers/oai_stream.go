package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"memdoor/pkg/llm"
)

// The streaming callback contract (WithStreamCallback, the ctx key,
// and the producer-side reader) lives in pkg/llm so neither side has
// to import the other. Producers (this package) call
// llm.StreamCallbackFromContext on the incoming ctx; consumers (the
// agent runtime in gateway/agent_adapter) call llm.WithStreamCallback
// to plant a closure. Duck-typed: the contract is just `func(string)`,
// so swapping either side does not affect the other's compile graph.

// streamAccumulator collects SSE chunks from an OAI-shape chat-
// completions response and reconstructs the non-streaming chatResponse
// shape the existing call sites expect. The first delta with content
// fires firstTokenAt so the timing log can report time-to-first-token
// separately from total wall clock.
//
// Behavior:
//   - text deltas: accumulated into content, optionally forwarded to cb
//   - tool-call deltas: indexed by .index and concatenated into a
//     single chatToolCall slot per index; OpenAI sends the function
//     name on the first chunk and the arguments piecemeal across
//     subsequent chunks, so we merge them
//   - usage: snapshotted from the final chunk
//   - finish_reason: last non-empty value wins
type streamAccumulator struct {
	content      strings.Builder
	toolCalls    map[int]*chatToolCall // keyed by .index; nil before first tool delta
	finishReason string
	errMsg       string // the provider's mid-stream error, if it sent one
	usage        chatUsage
	model        string // the served model id, from any chunk that names it
	genID        string // the provider's id for the call, from any chunk (meter)
	provider     string // the upstream host, from any chunk that names it
	firstTokenAt time.Time
	chunkCount   int
}

// failed reports a stream the provider ended with an error instead of an
// answer.
func (a *streamAccumulator) failed() bool { return a.finishReason == "error" || a.errMsg != "" }

// consume reads SSE events from r until [DONE] or EOF. Returns nil
// on a clean end-of-stream; any error from the underlying reader
// surfaces as-is so the caller can wrap with its own context.
// consume reads the stream; guard (may be nil) is the caller's prose
// breaker, asked after every fragment with the text so far and whether a
// tool call has begun — true stops the read, and the reply is marked
// "stream_guard" (llm.StopReasonStreamGuard) for the turn driver to retry.
func (a *streamAccumulator) consume(r io.Reader, cb llm.StreamCallback, guard llm.StreamGuard) error {
	scanner := bufio.NewScanner(r)
	// SSE event payloads can be large (especially the final chunk
	// with full timings + usage); raise the scanner buffer above
	// the 64 KB default.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		// SSE separates events with blank lines and prefixes data
		// lines with "data: ". Anything else (comments starting
		// with ":", or other event types) we ignore.
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		payload = bytes.TrimSpace(payload)
		if len(payload) == 0 {
			continue
		}
		if bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal(payload, &chunk); err != nil {
			// One malformed chunk shouldn't kill the stream — log
			// and continue. The next chunk often parses fine.
			continue
		}
		a.chunkCount++
		if chunk.Error != nil {
			a.errMsg = chunk.Error.Message
			if a.errMsg == "" {
				a.errMsg = fmt.Sprintf("provider error %v", chunk.Error.Code)
			}
			a.finishReason = "error"
		}

		// Reconstruct the message body from delta fragments.
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				if a.firstTokenAt.IsZero() {
					a.firstTokenAt = time.Now()
				}
				a.content.WriteString(ch.Delta.Content)
				if cb != nil {
					cb(ch.Delta.Content)
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				if a.toolCalls == nil {
					a.toolCalls = make(map[int]*chatToolCall)
				}
				existing, ok := a.toolCalls[tc.Index]
				if !ok {
					existing = &chatToolCall{Index: tc.Index}
					a.toolCalls[tc.Index] = existing
				}
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Type != "" {
					existing.Type = tc.Type
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				if len(tc.ExtraContent) > 0 {
					existing.ExtraContent = tc.ExtraContent
				}
				// OpenAI streams the JSON value of `arguments` a few
				// characters at a time; we concatenate fragments.
				existing.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != "" {
				a.finishReason = ch.FinishReason
			}
		}
		if guard != nil && guard(a.content.Len(), a.toolCalls != nil) {
			a.finishReason = streamGuardFinish
			return nil
		}

		if chunk.ID != "" && a.genID == "" {
			a.genID = chunk.ID
		}
		if chunk.Model != "" {
			a.model = chunk.Model
		}
		if chunk.Provider != "" {
			a.provider = chunk.Provider
		}
		if chunk.Usage != nil {
			a.usage = *chunk.Usage
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner: %w", err)
	}
	return nil
}

// build reconstructs the non-streaming chatResponse shape from the
// accumulated chunks so downstream code (which expects the
// single-shot response) keeps working.
func (a *streamAccumulator) build() chatResponse {
	resp := chatResponse{
		ID:       a.genID,
		Usage:    a.usage,
		Model:    a.model,
		Provider: a.provider,
	}
	choice := chatChoice{
		FinishReason: a.finishReason,
	}
	choice.Message.Content = a.content.String()
	if len(a.toolCalls) > 0 {
		// Emit tool calls in index order so downstream tool dispatchers
		// see them the same shape OpenAI does in a non-stream response.
		maxIdx := -1
		for k := range a.toolCalls {
			if k > maxIdx {
				maxIdx = k
			}
		}
		choice.Message.ToolCalls = make([]chatToolCall, 0, len(a.toolCalls))
		for i := 0; i <= maxIdx; i++ {
			if tc, ok := a.toolCalls[i]; ok {
				choice.Message.ToolCalls = append(choice.Message.ToolCalls, *tc)
			}
		}
	}
	resp.Choices = []chatChoice{choice}
	return resp
}

// streamGuardFinish marks a reply the caller's StreamGuard stopped.
const streamGuardFinish = "stream_guard"
