package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Resolver resolves SecretRef values by dispatching to the appropriate Provider.
type Resolver interface {
	Resolve(ctx context.Context, ref SecretRef) (string, error)
	// ResolveString handles both plain strings and JSON-encoded SecretRefs.
	// If value looks like a SecretRef JSON object, it parses and resolves it.
	// Otherwise returns the plain string as-is (backward compatible).
	ResolveString(ctx context.Context, value string) (string, error)
}

type resolver struct {
	providers map[string]Provider
}

// NewResolver creates a Resolver backed by the given providers.
func NewResolver(providers map[string]Provider) Resolver {
	return &resolver{providers: providers}
}

func (r *resolver) Resolve(ctx context.Context, ref SecretRef) (string, error) {
	provider, ok := r.providers[ref.Source]
	if !ok {
		return "", fmt.Errorf("unknown secret provider: %q", ref.Source)
	}
	value, err := provider.Get(ctx, ref.ID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret %q from %q: %w", ref.ID, ref.Source, err)
	}
	return value, nil
}

func (r *resolver) ResolveString(ctx context.Context, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{") {
		return value, nil
	}

	var ref SecretRef
	if err := json.Unmarshal([]byte(trimmed), &ref); err != nil {
		return value, nil
	}
	if ref.Source == "" || ref.ID == "" {
		return value, nil
	}

	return r.Resolve(ctx, ref)
}
