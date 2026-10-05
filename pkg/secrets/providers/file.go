package providers

import (
	"context"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fileProvider struct{}

func NewFileProvider() *fileProvider {
	return &fileProvider{}
}

func (p *fileProvider) Name() string { return "file" }

func (p *fileProvider) Get(_ context.Context, id string) (string, error) {
	data, err := os.ReadFile(id)
	if err != nil {
		return "", fmt.Errorf("failed to read secret file %q: %w", id, err)
	}
	return strings.TrimRight(string(data), "\n\r"), nil
}

func (p *fileProvider) Set(_ context.Context, id string, value string) error {
	dir := filepath.Dir(id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory for secret file: %w", err)
	}
	return os.WriteFile(id, []byte(value+"\n"), 0600)
}

func (p *fileProvider) Delete(_ context.Context, id string) error {
	if err := os.Remove(id); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete secret file %q: %w", id, err)
	}
	return nil
}

func (p *fileProvider) List(_ context.Context) ([]string, error) {
	secretsDir := shared.MemdoorHome("secrets")
	entries, err := os.ReadDir(secretsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list secrets directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
