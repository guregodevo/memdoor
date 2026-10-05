package providers

import (
	"context"

	"memdoor/pkg/llm"
)

// LLMClient abstracts different LLM providers behind a common interface.
// All providers convert their responses to Anthropic's Message format.
type LLMClient interface {
	Messages() MessageService
}

// MessageService matches the calling convention used throughout the codebase.
type MessageService interface {
	New(ctx context.Context, params llm.MessageNewParams) (*llm.Message, error)
}
