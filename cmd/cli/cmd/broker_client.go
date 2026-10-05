package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"memdoor/pkg/shared"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client half of the broker: the CLI signs in and calls memdoor.ai with the
// account token; it never holds a model vendor's key.

const defaultBillingBase = "https://memdoor.ai/billing"

// billingBase is where the billing service lives. Overridable for
// self-hosted/staging via MEMDOOR_BILLING_URL.
func billingBase() string {
	// Only memdoor.ai or this machine: the account token goes there.
	if v := os.Getenv("MEMDOOR_BILLING_URL"); v != "" && shared.AccountHostTrusted(v) {
		return strings.TrimRight(v, "/")
	}
	return defaultBillingBase
}

// brokerToken is the caller's billing credential (issued at signup; the
// MVP accepts a service token from the environment or ~/.memdoor).
func brokerToken() string {
	if v := os.Getenv("MEMDOOR_BILLING_TOKEN"); v != "" {
		return v
	}
	if b, err := os.ReadFile(shared.MemdoorHome("billing-token")); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}

// brokerCallNoAuth is for the login handshake — the only endpoints that
// exist to GET a credential.
func brokerCallNoAuth(method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, billingBase()+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("billing service unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("billing service: HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func brokerCall(method, path string, body any, out any) error {
	token := brokerToken()
	if token == "" {
		// THE NEXT STEP HAS TO BE THE REAL ONE (battle test, 2026-09-27). This
		// said "run `memdoor login`, then `memdoor credits topup`" — a prepaid
		// balance from a product that no longer exists. Someone who just ran `memdoor account subscribe`
		// was sent to top up credits they do not need for a subscription they
		// were trying to buy.
		return fmt.Errorf("not signed in — run `memdoor login you@example.com` first (a six-digit code arrives by email)")
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, billingBase()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("billing service unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("billing service: HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// What is left is the transport: the same call every billing request uses.
