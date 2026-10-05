package plan

import (
	"testing"
)

func TestUnknownPlanIsRefusedRatherThanGuessed(t *testing.T) {
	// A plan decides what a workspace may spend. A typo must not resolve to
	// something generous.
	for _, s := range []string{"premium", "unlimited", "prro"} {
		if p, err := Parse(s); err == nil {
			t.Errorf("%q parsed as %q instead of being refused", s, p)
		}
	}
	if p, err := Parse("  ENTERPRISE "); err != nil || p != Enterprise {
		t.Errorf("case and padding should still parse: %v, %v", p, err)
	}
}

func TestNoPlanMeansFree(t *testing.T) {
	// A workspace that never chose has not bought anything: defaulting the
	// other way would open the hosted relay to an unconfigured workspace.
	p, err := Parse("")
	if err != nil || p != Free {
		t.Fatalf("empty plan became %q (%v), want free", p, err)
	}
}

// One workspace on EVERY plan, so no method varies it — this test exists to
// fail if MaxWorkspaces ever comes back. An enterprise buys more capacity and
// more seats inside its workspace, never more workspaces.
func TestNoPlanVariesTheWorkspaceCount(t *testing.T) {
	var p any = Solo
	if _, ok := p.(interface{ MaxWorkspaces() int }); ok {
		t.Fatal("MaxWorkspaces is back — one workspace is an invariant, not a plan feature")
	}
}

// NOTHING PREPAYS ANY MORE (2026-09-05). The prepaid tier is retired:
// what a workspace may do comes from a subscription, so no plan carries a
// balance predicate and nothing is gated on one. This test fails if a
// prepaid gate comes back — which would refuse every subscriber
// the moment their balance read zero, which it always does.
func TestNoPlanIsGatedOnABalance(t *testing.T) {
	for _, p := range []Plan{Free, Pro, Solo, Enterprise} {
		var a any = p
		if _, ok := a.(interface{ PrepaysCredits() bool }); ok {
			t.Fatalf("%s still answers a balance question", p)
		}
	}
}

// Storage keys are frozen, but what one MEANS may change once, with a
// note: rows written under the retired prepaid tier are subscribers now.
func TestTheRetiredPrepaidTierReadsAsPro(t *testing.T) {
	for _, name := range []string{"solo", "SOLO", " Solo ", "team", "pro"} {
		if p, err := Parse(name); err != nil || p != Pro {
			t.Errorf("%q became %q (%v), want pro", name, p, err)
		}
	}
	if p, _ := Parse(""); p != Free {
		t.Error("an unset plan is free")
	}
	if _, err := Parse("gold"); err == nil {
		t.Error("an unknown plan must fail rather than grant something")
	}
}

// The stored values are FROZEN. If this test fails, something renamed a
// storage key — which costs a migration, a CHECK-constraint rebuild and
// another legacy alias, for a word only the price list shows.
//
// Change Label() instead. It is the human-facing name and nothing is stored
// under it, so the price list, the invoice and the marketing can call these
// plans anything at all without touching a row.
func TestStorageKeysAreFrozen(t *testing.T) {
	for want, p := range map[string]Plan{
		"free":       Free,
		"solo":       Solo,
		"enterprise": Enterprise,
	} {
		if got := p.String(); got != want {
			t.Errorf("storage key %q became %q — rename Label(), never the constant", want, got)
		}
	}
}

// And the label is genuinely free to move: it must not be the storage key by
// accident, or the next rename quietly becomes a migration again.
func TestLabelIsSeparateFromTheStoredValue(t *testing.T) {
	if Enterprise.Label() == Enterprise.String() {
		t.Fatal("the label and the storage key are the same string, so renaming one renames both")
	}
	if Enterprise.Label() != "Enterprise" {
		t.Fatalf("enterprise reads as %q", Enterprise.Label())
	}
	// Pro's label may become "Creator", "Studio" or anything else the
	// pricing page needs; the row stays "pro".
	if Pro.String() != "pro" {
		t.Fatalf("the stored key moved: %q", Pro.String())
	}
}
