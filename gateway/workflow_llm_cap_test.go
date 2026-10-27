package gateway

import (
	"testing"

	ctxmgmt "memdoor/gateway/context"
)

// An llm task may write up to the provider's ceiling within its window; an
// agent turn's 16K reserve is its floor, not its cap.
func TestAnLLMTaskWritesUpToTheProvidersCeiling(t *testing.T) {
	windows := map[string]int{"openrouter:qwen/qwen3.8-27b": 1_000_000, "groq:qwen/qwen3.8-27b": 131_072, "small": 32_768}
	outputs := map[string]int{"openrouter:qwen/qwen3.8-27b": 235_929, "groq:qwen/qwen3.8-27b": 16_384, "small": 0}
	ctxmgmt.SetRemoteWindow(func(m string) int { return windows[m] })
	ctxmgmt.SetRemoteOutput(func(m string) int { return outputs[m] })
	defer ctxmgmt.SetRemoteWindow(nil)
	defer ctxmgmt.SetRemoteOutput(nil)

	if got := llmTaskMaxTokens("openrouter:qwen/qwen3.8-27b", 4000); got != 235_929 {
		t.Errorf("the 27B on OpenRouter: %d, want the 235929 ceiling", got)
	}
	if got := llmTaskMaxTokens("groq:qwen/qwen3.8-27b", 4000); got != 16_384 {
		t.Errorf("the 27B on Groq: %d, want its 16384 ceiling", got)
	}
	if got := llmTaskMaxTokens("small", 4000); got != 8_192 {
		t.Errorf("a model with no ceiling: %d, want the agent turn's reserve", got)
	}
	// A prompt that fills most of the window leaves the reply the rest.
	if got := llmTaskMaxTokens("openrouter:qwen/qwen3.8-27b", 2_900_000); got != 1_000_000-2_900_000/3-1024 {
		t.Errorf("a long prompt: %d", got)
	}
}
