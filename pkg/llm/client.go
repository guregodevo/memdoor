// Package llm defines the duck-typed LLM client interface agent components
// talk to: one shape for chat completions. gateway/providers supplies the
// implementation, the single place where HTTP, auth, retries and
// request/response logging live. A pkg-level package so domain code can
// depend on the interface without importing the gateway.
package llm

import "context"

// Role names are the OpenAI / Anthropic chat roles. The provider
// layer translates between OAI and Anthropic SDK shapes; callers see
// only these.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ChatMessage is one chat turn for the consolidated Client.Complete
// path. Content is plain text only — its callers (one-shot summarize /
// classify jobs) need no tool-use or tool-result blocks. The richer
// Message type (in types.go) is the
// agent-runtime shape that DOES carry tool blocks.
type ChatMessage struct {
	Role    string
	Content string
}

// ChatRequest is the LLM-agnostic chat completion request.
//
// Caller identifies the coarse subsystem ("agent"). Agent is the
// fine-grained identity within that surface — which agent (in the
// user-facing sense: chief, coder, verifier) actually fired
// the call. The provider logs both fields so `memdoor tokens --by
// agent` can attribute cost per agent and operators can route the
// cheap-tier work to a cheaper model without guessing which agent
// dominates the bill. Empty Agent is allowed (provider logs "unknown")
// but every callsite should set a meaningful value.
type ChatRequest struct {
	Caller      string
	Agent       string
	Model       string
	Messages    []ChatMessage
	Temperature float64 // 0 = use provider default
	MaxTokens   int     // 0 = use provider default
}

// ChatResponse carries the assistant's reply. Content is post-
// reasoning-tag-stripped (the provider strips <think>…</think> blocks
// from reasoning models so callers don't have to). FinishReason is
// the provider's own string ("stop", "length", "tool_calls", ...).
// Usage carries token counts pulled from the provider's response —
// providers that don't surface usage leave it zero.
type ChatResponse struct {
	Content      string
	FinishReason string
	Usage        Usage
}

// Client is the single entry point every LLM-using package calls.
// Implementations live in gateway/providers and own all HTTP / auth /
// logging concerns. Callers must NOT make their own /chat/completions
// requests — the consolidation exists so logs, error handling, and
// model fallbacks live in exactly one place.
type Client interface {
	Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
