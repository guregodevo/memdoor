package llm

import "context"

// StreamCallback is the per-delta hook callers can attach to a request
// context. Provider implementations (OAI-compatible client today) call
// it for every text fragment the upstream emits as it arrives, so the
// agent runtime can broadcast real-time WebSocket deltas without
// changing the synchronous Client.Complete / MessageService.New shape.
//
// Lives in pkg/llm to keep the wire duck-typed:
//   - Producers (gateway/providers) read it from ctx via
//     StreamCallbackFromContext — they only need to know "is anyone
//     listening" and "what should I pass each delta to?"
//   - Consumers (gateway/agent_adapter) plant their own implementation
//     via WithStreamCallback — they don't need to import providers.
//
// Either side can be swapped without touching the other. The contract
// is intentionally just `func(string)` so neither side has to import a
// streaming-event type from the other's package.
type StreamCallback func(delta string)

// streamCallbackKey is the unexported context key. Unexported struct
// type per the standard Go ctx-key pattern so callers can't collide
// from outside this package.
type streamCallbackKey struct{}

// WithStreamCallback returns a child ctx that carries the given
// callback. Passing nil returns the parent ctx unchanged — same effect
// as not calling this at all. Provider call sites check for the key's
// presence; absence means "non-streaming behavior" (accumulate
// silently, no per-delta hook).
func WithStreamCallback(ctx context.Context, cb StreamCallback) context.Context {
	if cb == nil {
		return ctx
	}
	return context.WithValue(ctx, streamCallbackKey{}, cb)
}

// StreamCallbackFromContext returns the callback planted by
// WithStreamCallback, or nil when none is present. Provider code calls
// this once per request to decide whether to fire deltas.
func StreamCallbackFromContext(ctx context.Context) StreamCallback {
	if cb, ok := ctx.Value(streamCallbackKey{}).(StreamCallback); ok {
		return cb
	}
	return nil
}

// NoticeFunc tells the person something about the request in flight that
// is not the model's text: the provider went silent or refused, and the
// request is being asked again. Without it the screen showed only a clock.
type NoticeFunc func(text string)

type noticeKey struct{}

// WithNotice returns a child ctx carrying fn; nil returns ctx unchanged.
func WithNotice(ctx context.Context, fn NoticeFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, noticeKey{}, fn)
}

// Notify sends text to the NoticeFunc on ctx, if one was planted.
func Notify(ctx context.Context, text string) {
	if fn, ok := ctx.Value(noticeKey{}).(NoticeFunc); ok {
		fn(text)
	}
}

type utilityCallKey struct{}

// WithUtilityCall marks this inference as a background utility generation —
// memory flush, compaction summarize, retrieval-query reformulation: work that
// serves the pipeline, not a conversation. Providers with a prefix-reusing
// engine session route marked calls to a throwaway session, so
// a utility prompt never evicts the KV prefix an agent's next turn would
// reuse. Observed before this existed: a ~19.5K-token compaction prompt ran on
// the main session mid-turn and the interrupted turn re-paid its entire
// prefill. Do NOT set it on real agent turns — they belong on the main
// session precisely so they reuse each other's prefixes.
func WithUtilityCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, utilityCallKey{}, true)
}

// StreamGuard is the mid-stream circuit breaker a caller can attach to a
// request context, mirroring StreamCallback's duck-typed wire. The provider
// consults it as the reply streams — byteCount is how much text has arrived,
// sawToolCall whether a tool-call has started anywhere in it — and a true
// return stops the read: the reply comes back marked StopReasonStreamGuard
// for the caller to retry.
//
// It exists because a runaway generation burns real time and money BEFORE any
// after-the-fact guard can see it: measured three times on 2026-08-31, a
// coder turn spending its whole 16k output cap on tag-free prose — roughly
// seven minutes and a truncation cycle per occurrence. Only the reader of the
// stream can stop that while it is happening.
type StreamGuard func(byteCount int, sawToolCall bool) bool

type streamGuardKey struct{}

// WithStreamGuard returns a child ctx carrying the guard; nil returns the
// parent unchanged, and absence means "never stop early".
func WithStreamGuard(ctx context.Context, g StreamGuard) context.Context {
	if g == nil {
		return ctx
	}
	return context.WithValue(ctx, streamGuardKey{}, g)
}

// StreamGuardFromContext returns the guard planted by WithStreamGuard, or nil.
func StreamGuardFromContext(ctx context.Context) StreamGuard {
	if g, ok := ctx.Value(streamGuardKey{}).(StreamGuard); ok {
		return g
	}
	return nil
}

// forcedToolCallKey marks a request whose reply MUST be a tool call: the
// provider prefills the assistant turn with the call's opening marker, so
// the model continues inside it and cannot answer in prose. Set by the
// turn driver on the retry after a reasoning-only or guard-stopped reply.
type forcedToolCallKey struct{}

// WithForcedToolCall returns a child ctx that forces the next reply to open
// on a tool call.
func WithForcedToolCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, forcedToolCallKey{}, true)
}

// ForcedToolCallFromContext reports whether the reply must open on a tool call.
func ForcedToolCallFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(forcedToolCallKey{}).(bool)
	return v
}
