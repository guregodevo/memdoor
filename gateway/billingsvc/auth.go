package billingsvc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"memdoor/pkg/plan"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sign-in is by email code (auth_email.go): the code proves the inbox, the
// inbox is the account. The device flow that used to live here (`memdoor
// login --browser`: a code shown in the terminal, approved on /activate with
// an email typed into a form) was removed on 2026-10-05: the form proved
// nothing about the email, so anyone who knew an address could mint a token
// for its workspace. Tokens are opaque, kept in a JSON file beside the
// billing db.

// account is the registered owner of a billing workspace. Registration
// IS the first login: whoever claims an unclaimed workspace owns it, and
// the binding is permanent (a later claim by a different email is
// refused — otherwise a guessed workspace name would hand over someone
// else's credits).
type account struct {
	Email     string    `json:"email"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`

	// Plan is what this workspace has bought. It lives HERE, in the billing
	// service's own store, rather than on the gateway's workspaces row: a
	// self-hosted gateway carries no billing code at all, so an entitlement
	// kept there could never be enforced, and the billing service is the
	// process that bills. Empty means free — see authStore.PlanFor.
	Plan string `json:"plan,omitempty"`

	// Billing is the subscription period behind a pro plan ("monthly" or
	// "yearly") and SubscriptionID the Stripe subscription that pays it, so
	// its cancellation can find the workspace it ends.
	Billing        string `json:"billing,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
}

// AUTHENTICATION, WORKSPACE AND BILLING ARE ONE THING.
//
// A user always signs in TO a workspace — the email code lands on one — so a
// token IS a workspace binding, and there is no user in the
// system unattached to one. Every authenticated request therefore carries a
// workspace, which is why the broker can answer "what plan is this?" for any
// of them without a second lookup or an extra parameter.
//
// That is why these three live in one store rather than three. The plan is not
// on the gateway's workspaces row: a self-hosted gateway carries no billing
// code at all, so an entitlement kept there could never be enforced, and two
// places holding the same fact is two places to disagree about who may spend
// money. The process that bills is the one that decides.
//
//	token  →  workspace  →  plan  →  what the seat unlocks (remote control)
type authStore struct {
	mu       sync.Mutex
	path     string
	Tokens   map[string]string     `json:"tokens"`   // token → workspace
	Accounts map[string]*account   `json:"accounts"` // workspace → owner
	Codes    map[string]*emailCode `json:"codes"`    // email → outstanding sign-in code
}

func newAuthStore(path string) *authStore {
	a := &authStore{path: path, Tokens: map[string]string{}, Accounts: map[string]*account{}, Codes: map[string]*emailCode{}}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, a)
		if a.Tokens == nil {
			a.Tokens = map[string]string{}
		}
		if a.Accounts == nil {
			a.Accounts = map[string]*account{}
		}
		if a.Codes == nil {
			a.Codes = map[string]*emailCode{}
		}
	}
	return a
}

func (a *authStore) flushLocked() {
	b, _ := json.MarshalIndent(a, "", "  ")
	_ = os.WriteFile(a.path, b, 0o600)
}

// PlanFor is the plan RECORDED against a workspace, or free.
//
// FREE IS THE DEFAULT. Signing in is not buying: an account that has never
// subscribed holds no seat, which is exactly what the free plan is. Defaulting
// the other way would give a workspace that never paid what a seat buys.
//
// Enterprise is never reached by default or by paying: it is set deliberately,
// because committed capacity is a contract rather than a checkbox.
func (a *authStore) PlanFor(workspace string) plan.Plan {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.planForLocked(workspace)
}

func (a *authStore) planForLocked(workspace string) plan.Plan {
	acct := a.Accounts[workspace]
	if acct == nil || acct.Plan == "" {
		return plan.Free
	}
	p, err := plan.Parse(acct.Plan)
	if err != nil {
		return plan.Free // an unreadable plan is not an upgrade
	}
	return p
}

// newAccount is the ONLY way an account comes into being.
//
// A fail-fast factory: it refuses the arguments it cannot work without and
// initialises every field, so no code path can produce an account whose plan
// is an accident. A new workspace is FREE — signing in is not buying — and it
// is written down rather than left empty, because a zero value that happens to
// mean the right thing today is a zero value that means something else after
// the next rename.
func newAccount(workspace, email string) (*account, error) {
	if workspace == "" {
		return nil, fmt.Errorf("an account needs a workspace: every user signs in to one")
	}
	return &account{
		Email:     email,
		Workspace: workspace,
		Plan:      string(plan.Free),
		CreatedAt: time.Now(),
	}, nil
}

// SetPlan records a workspace's plan. Operator-only by construction: nothing
// on the user-facing surface calls it, because enterprise is arranged in a
// conversation rather than bought from a button.
func (a *authStore) SetPlan(workspace string, p plan.Plan) {
	a.mu.Lock()
	defer a.mu.Unlock()
	acct := a.Accounts[workspace]
	if acct == nil {
		var err error
		if acct, err = newAccount(workspace, ""); err != nil {
			return
		}
		a.Accounts[workspace] = acct
	}
	acct.Plan = string(p)
	a.flushLocked()
}

// emailFor is the owner's email of a workspace, "" when nobody claimed it.
func (a *authStore) emailFor(workspace string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if acct := a.Accounts[workspace]; acct != nil {
		return acct.Email
	}
	return ""
}

// WorkspaceFor resolves a bearer token to its workspace ("" = unknown).
func (a *authStore) WorkspaceFor(token string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Tokens[token]
}

func randCode(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func authStorePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "billing-auth.json")
}

// SetSubscription is a paid subscription arriving: the workspace is pro
// from here, on the period the customer chose.
func (a *authStore) SetSubscription(workspace, subscriptionID, period string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	acct := a.accountLocked(workspace)
	if acct == nil {
		return
	}
	acct.Plan = string(plan.Pro)
	acct.Billing = period
	acct.SubscriptionID = subscriptionID
	a.flushLocked()
}

// EndSubscription is that subscription ending: the workspace it paid for
// goes back to free. Returns the workspace, or "" when none matched.
func (a *authStore) EndSubscription(subscriptionID string) string {
	if subscriptionID == "" {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for ws, acct := range a.Accounts {
		if acct.SubscriptionID == subscriptionID {
			acct.Plan = string(plan.Free)
			acct.Billing, acct.SubscriptionID = "", ""
			a.flushLocked()
			return ws
		}
	}
	return ""
}

// accountLocked finds or creates the account a plan change lands on.
func (a *authStore) accountLocked(workspace string) *account {
	acct := a.Accounts[workspace]
	if acct == nil {
		var err error
		if acct, err = newAccount(workspace, ""); err != nil {
			return nil
		}
		a.Accounts[workspace] = acct
	}
	return acct
}
