package llm

import "context"

// A route key says WHICH conversation a request belongs to, so a provider
// serving from a pool can send it to the machine that already holds its cached
// prefix.
//
// It travels on the context for the same reason the stream callback does: the
// agent runtime plants it without importing the provider package, and the
// provider reads it without knowing or caring who planted it — or whether
// anybody did. A request with no route key is not an error; it means "any
// member will do", which is the right answer for warm-ups, health checks and
// anything else with no conversation behind it.
//
// The key must be STABLE for the life of a conversation. A session identifier
// is the natural choice: it is exactly the unit whose cached prefix — system
// prompt, repo context, history — is worth keeping on one machine.
type routeKey struct{}

// WithRouteKey returns a child context carrying the routing key. An empty key
// is ignored, so callers need not special-case a session they cannot name.
func WithRouteKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, routeKey{}, key)
}
