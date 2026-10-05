package gateway

import (
	"context"
	"testing"

	"memdoor/gateway/providers"
	"memdoor/pkg/decision"
)

type readyStub bool

func (r readyStub) ID() string  { return "systemone" }
func (r readyStub) Ready() bool { return bool(r) }
func (r readyStub) Evaluate(context.Context, string, decision.Request) (decision.Result, error) {
	return decision.Result{}, nil
}

func TestAutoDecisionModel(t *testing.T) {
	// The person's own key first, seat or not (2026-10-03: Jev is not Pro).
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	if got := autoDecisionModel(readyStub(true), true); got != "systemone" {
		t.Fatalf("a seat with its own key judges on that key: %q", got)
	}
	// A seat supplies no key (2026-10-04): signed in with no key of your
	// own, decisions are off — never the broker.
	if got := autoDecisionModel(nil, false); got != "" {
		t.Fatalf("signed in, no own key: off, never the broker, got %q", got)
	}
	t.Setenv("MEMDOOR_BILLING_TOKEN", "")
	t.Setenv("HOME", t.TempDir())
	if got := autoDecisionModel(readyStub(true), true); got != "systemone" {
		t.Fatalf("no seat, own key: systemone, got %q", got)
	}
	if got := autoDecisionModel(nil, false); got != "" {
		t.Fatalf("neither: off, got %q", got)
	}
}

// On a company's vendor key (providers/vendor.go) nothing but the vendor may be
// contacted: not memdoor.ai's broker, and not OpenRouter on the coding key.
// Only a decision key the gateway was given explicitly judges.
func TestNoBrokerDecisionsOnAVendorKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: providers.VendorEngineName, Vendor: providers.VendorAnthropic, Endpoint: "https://api.anthropic.com", APIKey: "k", Model: "claude-sonnet-5"}); err != nil {
		t.Fatal(err)
	}
	if got := autoDecisionModel(nil, false); got != "" {
		t.Fatalf("vendor key + seat, no own decision key: decisions off, got %q", got)
	}
	if got := autoDecisionModel(readyStub(true), true); got != "" {
		t.Fatalf("vendor key: the OpenRouter coding key must not judge, got %q", got)
	}
	if got := autoDecisionModel(readyStub(true), false); got != "systemone" {
		t.Fatalf("a decision key of the gateway's own still serves: got %q", got)
	}
}
