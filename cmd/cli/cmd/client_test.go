package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPerGatewayCredentialIsolation guards against the cross-gateway clobber:
// logging into a second gateway must not evict the session for the first. Before
// the fix, credentials.json held a single token, so `auth login-direct --gateway
// prod` overwrote the local token and silently logged the user out of local.
// Falsifies if a prod login (or logout) disturbs the local session.
func TestPerGatewayCredentialIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })

	gatewayAddr = "http://localhost:18789"
	if err := saveCredentials(&credentials{Token: "local-token", Email: "greg@test.local"}); err != nil {
		t.Fatalf("save local: %v", err)
	}
	gatewayAddr = "https://memdoor.ai"
	if err := saveCredentials(&credentials{Token: "prod-token", Email: "greg@test"}); err != nil {
		t.Fatalf("save prod: %v", err)
	}

	// The prod login must NOT have evicted the local session.
	gatewayAddr = "http://localhost:18789"
	c, err := loadCredentials()
	if err != nil {
		t.Fatalf("load local after prod login: %v", err)
	}
	if c.Token != "local-token" {
		t.Errorf("local session clobbered by prod login: got %q, want local-token", c.Token)
	}

	gatewayAddr = "https://memdoor.ai"
	if c, err := loadCredentials(); err != nil || c.Token != "prod-token" {
		t.Errorf("prod session wrong: got %v err=%v", c, err)
	}

	// Logging out of prod must leave the local session intact.
	gatewayAddr = "https://memdoor.ai"
	if err := deleteCredentials(); err != nil {
		t.Fatalf("logout prod: %v", err)
	}
	gatewayAddr = "http://localhost:18789"
	if c, err := loadCredentials(); err != nil || c.Token != "local-token" {
		t.Errorf("local session lost after prod logout: got %v err=%v", c, err)
	}
}

// TestLegacyCredentialMigratesToDefaultGateway verifies a pre-existing
// single-token credentials.json (old format) is still honored for the default
// local gateway, so upgrading users aren't logged out.
func TestLegacyCredentialMigratesToDefaultGateway(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })

	// Write a legacy single-token file directly.
	if err := os.MkdirAll(filepath.Dir(credentialsPath()), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(credentialsPath(), []byte(`{"token":"legacy-local","email":"greg@test.local"}`), 0600); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	gatewayAddr = "http://localhost:18789"
	c, err := loadCredentials()
	if err != nil || c.Token != "legacy-local" {
		t.Errorf("legacy token not honored for default gateway: got %v err=%v", c, err)
	}
}

// An expired session is named as such — not as a missing channel — and a
// live one is no problem at all (the mutation check: drop the expiry
// comparison and the second case fails).
func TestExpiredSessionIsNamedNotTheChannel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	gatewayAddr = "http://localhost:18789"

	// No credentials at all is a FIRST RUN, and setup is the answer (onboarding
	// walk, 2026-09-27); an expired session is the case this test is about.
	if err := sessionProblem(); err == nil || !strings.Contains(err.Error(), "memdoor setup") {
		t.Fatalf("no credentials at all: want the setup instruction, got %v", err)
	}
	write := func(exp time.Time) {
		t.Helper()
		if err := saveCredentials(&credentials{Token: "tok", Email: "greg@test.local", ExpiresAt: exp}); err != nil {
			t.Fatal(err)
		}
	}
	write(time.Now().Add(-40 * time.Minute))
	err := sessionProblem()
	if err == nil || !strings.Contains(err.Error(), "expired") || !strings.Contains(err.Error(), "login-direct --email greg@test.local") {
		t.Fatalf("expired token: want the expiry and the login line, got %v", err)
	}
	if strings.Contains(err.Error(), "channel") {
		t.Fatalf("an expired session must not mention a channel: %v", err)
	}
	write(time.Now().Add(24 * time.Hour))
	if err := sessionProblem(); err != nil {
		t.Fatalf("a live session is not a problem, got %v", err)
	}
}
