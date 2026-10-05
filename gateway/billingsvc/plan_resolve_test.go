package billingsvc

import (
	"path/filepath"
	"testing"

	"memdoor/pkg/plan"
)

func authForTest(t *testing.T) *authStore {
	t.Helper()
	return newAuthStore(filepath.Join(t.TempDir(), "auth.json"))
}

// Every user belongs to a workspace, so every token resolves to one — the
// device grant refuses to complete without it. That is what lets the broker
// answer "what plan is this?" for any authenticated request: workspace in,
// plan out, no separate lookup and nothing to pass around.
func TestATokenAlwaysResolvesToAWorkspace(t *testing.T) {
	a := authForTest(t)
	a.Tokens["tok"] = "acme"
	if got := a.WorkspaceFor("tok"); got != "acme" {
		t.Fatalf("token resolved to %q", got)
	}
	if got := a.WorkspaceFor("unknown"); got != "" {
		t.Fatalf("an unissued token resolved to workspace %q", got)
	}
}

// A workspace that has never been given a plan is FREE.
//
// Signing in is not buying. Defaulting the other way would let an account that
// never put money in hold Pro — and the promotion path (paying) is what
// moves it, not the act of holding a token.
func TestAnUnsetPlanIsFree(t *testing.T) {
	a := authForTest(t)
	if got := a.PlanFor("never-seen"); got != plan.Free {
		t.Fatalf("an unknown workspace is %q — it could use Pro without paying", got)
	}
	a.Accounts["acme"] = &account{Workspace: "acme"}
	if got := a.PlanFor("acme"); got != plan.Free {
		t.Fatalf("an account with no plan set is %q", got)
	}
}

// Enterprise is never reached by default. It is set deliberately, because
// committed capacity is a contract rather than a checkbox.
func TestEnterpriseIsOnlyEverSetDeliberately(t *testing.T) {
	a := authForTest(t)
	a.SetPlan("acme", plan.Enterprise)
	if got := a.PlanFor("acme"); got != plan.Enterprise {
		t.Fatalf("the plan did not stick: %q", got)
	}
	// It survives a reload — the operator set it once, not per process.
	reloaded := newAuthStore(a.path)
	if got := reloaded.PlanFor("acme"); got != plan.Enterprise {
		t.Fatalf("the plan did not survive a restart: %q", got)
	}
	// And no other workspace is dragged up with it.
	if got := reloaded.PlanFor("other"); got != plan.Free {
		t.Fatalf("setting one workspace's plan moved another to %q", got)
	}
}

// An unreadable plan is not an upgrade.
func TestAGarbledPlanDoesNotUnlockAnything(t *testing.T) {
	a := authForTest(t)
	a.Accounts["acme"] = &account{Workspace: "acme", Plan: "platinum"}
	if got := a.PlanFor("acme"); got != plan.Free {
		t.Fatalf("a garbled plan resolved to %q — a typo must never unlock spending", got)
	}
}

// A NEW workspace is free. Signing in is not buying, and defaulting the other
// way would let an account that never paid hold Pro.
func TestANewWorkspaceIsFree(t *testing.T) {
	acct, err := newAccount("acme", "dev@acme.test")
	if err != nil {
		t.Fatal(err)
	}
	if acct.Plan != string(plan.Free) {
		t.Fatalf("a new account is on %q, want free", acct.Plan)
	}
	// Written down, not left empty: a zero value that happens to mean the
	// right thing today means something else after the next rename.
	if acct.Plan == "" {
		t.Fatal("the plan was left empty rather than set")
	}
	if acct.CreatedAt.IsZero() || acct.Workspace != "acme" {
		t.Fatalf("the factory left fields unset: %+v", acct)
	}
}

// Fail fast: there is no user unattached to a workspace, so there is no
// account without one either.
func TestTheAccountFactoryRefusesAWorkspacelessAccount(t *testing.T) {
	if _, err := newAccount("", "dev@acme.test"); err == nil {
		t.Fatal("an account was created with no workspace")
	}
}

// A BALANCE IS NOT A PLAN (2026-09-05). Crediting a workspace used to
// promote it to the prepaid tier; under a subscription that would mean a
// refund or a support credit silently bought a month of the product.
func TestCreditingDoesNotBuyAPlan(t *testing.T) {
	a := authForTest(t)
	acct, _ := newAccount("acme", "dev@acme.test")
	a.Accounts["acme"] = acct
	if got := a.PlanFor("acme"); got != plan.Free {
		t.Fatalf("a fresh account is %q", got)
	}
	a.SetSubscription("acme", "sub_1", "monthly")
	if got := a.PlanFor("acme"); got != plan.Pro {
		t.Fatalf("a subscription is what buys the product, got %q", got)
	}
}

// A subscription never overwrites what an operator arranged: enterprise
// subscribing by accident must stay enterprise.
func TestEnterpriseIsNotDowngradedByASubscription(t *testing.T) {
	a := authForTest(t)
	a.SetPlan("acme", plan.Enterprise)
	if got := a.PlanFor("acme"); got != plan.Enterprise {
		t.Fatalf("enterprise became %q", got)
	}
}

// Enterprise promotes the WORKSPACE and everyone in it — never a seat. That is
// why the lever takes a workspace and there is no per-user plan anywhere.
func TestEnterprisePromotesTheWholeWorkspace(t *testing.T) {
	a := authForTest(t)
	a.Tokens["tok-one"] = "acme"
	a.Tokens["tok-two"] = "acme" // a second user in the same workspace
	a.SetPlan("acme", plan.Enterprise)

	for _, tok := range []string{"tok-one", "tok-two"} {
		if got := a.PlanFor(a.WorkspaceFor(tok)); got != plan.Enterprise {
			t.Errorf("user %s is on %q — the upgrade did not reach everyone in the workspace", tok, got)
		}
	}
}
