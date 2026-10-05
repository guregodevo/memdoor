package providers

import (
	"fmt"
	"log/slog"
	"sync"

	"memdoor/gateway/logs"
)

// llmLog returns the shared central logger every LLM request goes through.
// Lazily initialized (logs.New panics until the global logger is set up by
// main(), and tests that import this package don't always do that).
//
// One namespace ("LLM") so a single `memdoor logs query --regex LLM` shows the
// complete picture: prompt heads, model, token attribution.
var (
	llmLogMu  sync.Mutex
	llmLogVal *logs.EventLogger
)

func llmLog() *logs.EventLogger {
	llmLogMu.Lock()
	defer llmLogMu.Unlock()
	if llmLogVal != nil {
		return llmLogVal
	}
	defer func() {
		// Don't crash callers if the logger isn't initialized yet (tests,
		// early startup) — fall back to nil and the helpers no-op. Retried on
		// the next call so prod traffic still gets logging once it's up.
		_ = recover()
	}()
	llmLogVal = logs.New("LLM")
	return llmLogVal
}

// llmReqInfo is what every LLM request log carries. Pre-parsed so the call site
// doesn't repeat the truncation / role lookup logic.
//
// Caller is the coarse surface that fired the call ("agent"). Agent is the
// fine-grained identity within it — which buddy — so the meter can attribute
// cost per agent.
type llmReqInfo struct {
	Caller         string
	Agent          string
	Model          string
	MessageCount   int
	ToolCount      int
	ToolNames      []string
	SystemHead     string
	UserHead       string
	SystemPromptLn int
	UserMessageLn  int
}

// logLLMRequest emits the pre-call line: prompt heads + tool palette. The msg
// string embeds the most-grep-able fields so they show up in `memdoor logs
// query` output even when the structured slog attrs are stripped.
func logLLMRequest(info llmReqInfo) {
	log := llmLog()
	if log == nil {
		return
	}
	log.Info(
		fmt.Sprintf("LLM request caller=%s agent=%s model=%s messages=%d tools=%d system_head=%q user_head=%q",
			info.Caller, info.Agent, info.Model, info.MessageCount, info.ToolCount, info.SystemHead, info.UserHead),
		slog.String("caller", info.Caller),
		slog.String("agent", info.Agent),
		slog.String("model", info.Model),
		slog.Int("message_count", info.MessageCount),
		slog.Int("tool_count", info.ToolCount),
		slog.Any("tool_names", info.ToolNames),
		slog.Int("system_prompt_len", info.SystemPromptLn),
		slog.Int("user_message_len", info.UserMessageLn),
		slog.String("system_prompt_head", info.SystemHead),
		slog.String("user_message_head", info.UserHead),
	)
}

// Restored with the OpenAI-compatible provider (oai_native.go), from
// before 13e5cd4f removed it.
// capture.
type llmReqShape struct {
	Caller          string
	Model           string
	BodyBytes       int // size of marshalled JSON body
	MessageCount    int
	ToolCount       int
	SystemPromptLen int
}

func logLLMReqShape(s llmReqShape) {
	log := llmLog()
	if log == nil {
		return
	}
	log.Info(
		fmt.Sprintf("LLM request body caller=%s model=%s body_bytes=%d messages=%d tools=%d system_prompt_chars=%d",
			s.Caller, s.Model, s.BodyBytes,
			s.MessageCount, s.ToolCount, s.SystemPromptLen),
		slog.String("caller", s.Caller),
		slog.String("model", s.Model),
		slog.Int("body_bytes", s.BodyBytes),
		slog.Int("messages", s.MessageCount),
		slog.Int("tools", s.ToolCount),
		slog.Int("system_prompt_chars", s.SystemPromptLen),
	)
}

type llmRespInfo struct {
	Caller       string
	Agent        string // fine-grained agent name; see llmReqInfo.Agent
	Model        string
	Upstream     string // the host that served it, when the vendor names it (OpenRouter's provider)
	FinishReason string
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	ContentLen   int
	ContentHead  string
	ToolCalls    int
	// Cache split (providers that support automatic prefix caching
	// populate these; others leave 0). Surfacing them per-call lets us
	// see prompt-cache effectiveness in `memdoor logs query`.
	CacheHitTokens  int64
	CacheMissTokens int64
	// Time from request start to first SSE chunk containing content.
	// Captured at the OAI client streaming layer, so it includes
	// network round-trip + the provider's prefill. The perceptual
	// latency the user experiences before "the model starts talking."
	// Zero on non-streaming paths or tool-call-only rounds.
	FirstTokenMs float64
}

// logLLMResponse emits the post-call line with token usage and the
// content head. Token counts are the headline metric for billing
// + cost analysis; content_head answers "what did the model
// actually say" without retrieving the full session.
func logLLMResponse(info llmRespInfo) {
	log := llmLog()
	if log == nil {
		return
	}
	cacheStr := ""
	if info.CacheHitTokens > 0 || info.CacheMissTokens > 0 {
		cacheStr = fmt.Sprintf(" cache=%d/%d", info.CacheHitTokens, info.CacheMissTokens)
	}
	// The first token's arrival is measured client-side.
	timingsStr := ""
	if info.FirstTokenMs > 0 {
		timingsStr = fmt.Sprintf(" ttft=%.0fms", info.FirstTokenMs)
	}
	log.Info(
		fmt.Sprintf("LLM response caller=%s agent=%s model=%s finish=%s tokens=%d/%d/%d%s tool_calls=%d%s content_head=%q",
			info.Caller, info.Agent, info.Model, info.FinishReason,
			info.InputTokens, info.OutputTokens, info.TotalTokens,
			cacheStr,
			info.ToolCalls,
			timingsStr,
			info.ContentHead),
		slog.String("caller", info.Caller),
		slog.String("agent", info.Agent),
		slog.String("model", info.Model),
		slog.String("finish_reason", info.FinishReason),
		slog.Int64("input_tokens", info.InputTokens),
		slog.String("upstream", info.Upstream),
		slog.Int64("output_tokens", info.OutputTokens),
		slog.Int64("total_tokens", info.TotalTokens),
		slog.Int64("cache_hit_tokens", info.CacheHitTokens),
		slog.Int64("cache_miss_tokens", info.CacheMissTokens),
		slog.Int("content_len", info.ContentLen),
		slog.Int("tool_calls", info.ToolCalls),
		slog.String("content_head", info.ContentHead),
		slog.Float64("first_token_ms", info.FirstTokenMs),
	)
}
