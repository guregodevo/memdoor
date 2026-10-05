package gateway

// ctxKey is the gateway-package private type used as the key for every
// context.WithValue call inside this package. Using a private named type
// (instead of bare strings) prevents accidental collisions with other
// packages' context values — the Go documentation calls this out
// explicitly in the context package, and staticcheck enforces it via
// the SA1029 rule.
//
// Add a new constant of this type whenever a new value needs to flow
// through the agent execution / job machinery.
type ctxKey string

const (
	ctxRunID           ctxKey = "run_id"
	ctxSessionID       ctxKey = "session_id"
	ctxSession         ctxKey = "session"
	ctxAllowedTools    ctxKey = "allowed_tools"
	ctxSkipCompaction  ctxKey = "skip_compaction"
	ctxParentMessageID ctxKey = "parent_message_id"
)
