package routing

// BindingResolver resolves which agent should handle a message based on bindings
// Pattern: OpenClaw multi-agent routing with 4-tier priority
// Priority: peer match > account match > channel match > default agent
type BindingResolver struct {
}
