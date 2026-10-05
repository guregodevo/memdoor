//go:build darwin

package providers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "memdoor"

type keychainProvider struct{}

func NewKeychainProvider() (*keychainProvider, error) {
	if _, err := exec.LookPath("security"); err != nil {
		return nil, fmt.Errorf("macOS security command not found: %w", err)
	}
	return &keychainProvider{}, nil
}

func (p *keychainProvider) Name() string { return "keychain" }

func (p *keychainProvider) Get(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, "security", "find-generic-password",
		"-s", keychainService, "-a", id, "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("keychain: secret %q not found: %w", id, err)
	}
	return strings.TrimRight(string(out), "\n\r"), nil
}

func (p *keychainProvider) Set(ctx context.Context, id string, value string) error {
	// Delete existing entry first (ignore errors if not found)
	_ = p.Delete(ctx, id)

	cmd := exec.CommandContext(ctx, "security", "add-generic-password",
		"-s", keychainService, "-a", id, "-w", value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keychain: failed to store secret %q: %w", id, err)
	}
	return nil
}

func (p *keychainProvider) Delete(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "security", "delete-generic-password",
		"-s", keychainService, "-a", id)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keychain: failed to delete secret %q: %w", id, err)
	}
	return nil
}

func (p *keychainProvider) List(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "security", "dump-keychain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keychain: failed to list secrets: %w", err)
	}

	var keys []string
	lines := strings.Split(string(out), "\n")

	// Parse entries in blocks separated by "class:" lines.
	// For each block, check if it has our service and extract the account name.
	isMemdoor := false
	acctName := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// New entry block
		if strings.HasPrefix(trimmed, "class:") {
			if isMemdoor && acctName != "" {
				keys = append(keys, acctName)
			}
			isMemdoor = false
			acctName = ""
			continue
		}

		// Check for service name in either format
		svcPattern := fmt.Sprintf(`="%s"`, keychainService)
		if (strings.Contains(trimmed, `"svce"`) || strings.Contains(trimmed, "0x00000007")) &&
			strings.Contains(trimmed, svcPattern) {
			isMemdoor = true
		}

		// Extract account name
		if strings.Contains(trimmed, `"acct"<blob>="`) {
			start := strings.Index(trimmed, `"acct"<blob>="`) + len(`"acct"<blob>="`)
			end := strings.LastIndex(trimmed, `"`)
			if start < end {
				acctName = trimmed[start:end]
			}
		}
	}

	// Don't forget the last block
	if isMemdoor && acctName != "" {
		keys = append(keys, acctName)
	}

	return keys, nil
}
