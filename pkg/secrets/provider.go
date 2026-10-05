package secrets

import "context"

// Provider resolves and manages secrets from a specific backend.
// Implementations: env, keychain, file, exec.
type Provider interface {
	Get(ctx context.Context, id string) (string, error)
	Set(ctx context.Context, id string, value string) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]string, error)
	Name() string
}
