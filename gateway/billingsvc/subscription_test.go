package billingsvc

import (
	"testing"

	"memdoor/pkg/plan"
)

// A pro subscription is paying, and paying is the upgrade: the checkout
// that starts it makes the workspace pro on the period chosen; the
// deletion that ends it sends the workspace back to free.
func TestASubscriptionStartsAndEndsThePlan(t *testing.T) {
	a := authForTest(t)
	started := `{"type":"checkout.session.completed","data":{"object":{"id":"cs_1","mode":"subscription",
		"client_reference_id":"acme","subscription":"sub_9","payment_status":"paid","metadata":{"period":"yearly"}}}}`
	handled, _, err := applySubscriptionEvent(a, []byte(started))
	if !handled || err != nil {
		t.Fatalf("subscription checkout not applied: handled=%v err=%v", handled, err)
	}
	if a.PlanFor("acme") != plan.Pro || a.Accounts["acme"].Billing != "yearly" {
		t.Fatalf("workspace after subscribing: %+v", a.Accounts["acme"])
	}
	// A credit top-up checkout is not a subscription and is left to the
	// credits path.
	topup := `{"type":"checkout.session.completed","data":{"object":{"id":"cs_2","mode":"payment","client_reference_id":"acme","amount_total":2500,"payment_status":"paid"}}}`
	if handled, _, _ := applySubscriptionEvent(a, []byte(topup)); handled {
		t.Fatal("a payment-mode checkout must not be treated as a subscription")
	}
	ended := `{"type":"customer.subscription.deleted","data":{"object":{"id":"sub_9"}}}`
	if handled, _, err := applySubscriptionEvent(a, []byte(ended)); !handled || err != nil {
		t.Fatalf("subscription end not applied: handled=%v err=%v", handled, err)
	}
	if a.PlanFor("acme") != plan.Free {
		t.Fatalf("workspace after cancelling: %q", a.PlanFor("acme"))
	}
	if _, _, err := applySubscriptionEvent(a, []byte(ended)); err == nil {
		t.Fatal("ending a subscription nobody holds must be reported, not swallowed")
	}
}
