//go:build !darwin

package providers

import (
	"context"
	"fmt"
)

type keychainProvider struct{}

func NewKeychainProvider() (*keychainProvider, error) {
	return nil, fmt.Errorf("keychain provider is only available on macOS")
}

func (p *keychainProvider) Name() string { return "keychain" }

func (p *keychainProvider) Get(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("keychain provider is only available on macOS")
}

func (p *keychainProvider) Set(_ context.Context, _, _ string) error {
	return fmt.Errorf("keychain provider is only available on macOS")
}

func (p *keychainProvider) Delete(_ context.Context, _ string) error {
	return fmt.Errorf("keychain provider is only available on macOS")
}

func (p *keychainProvider) List(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("keychain provider is only available on macOS")
}
