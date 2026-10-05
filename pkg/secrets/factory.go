package secrets

import (
	"runtime"

	"memdoor/pkg/secrets/providers"
)

// DefaultResolver creates a Resolver with all available providers for the current platform.
func DefaultResolver() Resolver {
	providerMap := map[string]Provider{
		"env":  providers.NewEnvProvider(),
		"file": providers.NewFileProvider(),
		"exec": providers.NewExecProvider(),
	}

	if runtime.GOOS == "darwin" {
		if kc, err := providers.NewKeychainProvider(); err == nil {
			providerMap["keychain"] = kc
		}
	}

	return NewResolver(providerMap)
}
