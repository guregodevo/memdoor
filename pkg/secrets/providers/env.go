package providers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

type envProvider struct{}

func NewEnvProvider() *envProvider {
	return &envProvider{}
}

func (p *envProvider) Name() string { return "env" }

func (p *envProvider) Get(_ context.Context, id string) (string, error) {
	val, ok := os.LookupEnv(id)
	if !ok {
		return "", fmt.Errorf("environment variable %q not set", id)
	}
	return val, nil
}

func (p *envProvider) Set(_ context.Context, id string, value string) error {
	return os.Setenv(id, value)
}

func (p *envProvider) Delete(_ context.Context, id string) error {
	return os.Unsetenv(id)
}

func (p *envProvider) List(_ context.Context) ([]string, error) {
	var keys []string
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}
