package gateway

import (
	"crypto/sha256"
	"errors"
	"memdoor/pkg/plan"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Who may attach a terminal to the remote-control relay. The TUI dials the
// relay host (memdoor.ai by default) with the memdoor.ai ACCOUNT token, which
// only the billing service can read: this gateway's own logins know nothing
// of it. So a token this gateway issued names its user, and any other token
// is asked of billing's GET /v1/me, whose workspace is the account.
//
// On the hosted relay remote control is Pro (Greg, 2026-10-04: "Remote should
// be pro only"; it was free on every plan from 2026-09-29): billing's answer
// carries the plan, and an account without a seat is refused with what to do.
// A gateway relaying for its own users (a token it issued, someone hosting
// Memdoor themselves) is not asked: they pay for that hosting.

// errRemoteIsPro is what a free account hears from the hosted relay.
var errRemoteIsPro = errors.New("remote control is a Pro feature on memdoor.ai: memdoor account subscribe, then /remote again")

const (
	relayAccountPrefix   = "acct:"
	relayAccountTTL      = 5 * time.Minute
	relayAccountTimeout  = 5 * time.Second
	relayAccountCacheMax = 1024
)

type relayAccountEntry struct {
	id    string
	until time.Time
}

type relayAuth struct {
	local   wsTokenValidator
	billing string
	client  *http.Client
	now     func() time.Time

	mu    sync.Mutex
	cache map[[sha256.Size]byte]relayAccountEntry
}

// newRelayAuth checks the gateway's own tokens with local, and every other
// token against the billing service at billingBase (brokerBaseURL()).
func newRelayAuth(local wsTokenValidator, billingBase string) wsTokenValidator {
	if local == nil {
		panic("remote relay auth: a local token validator is required")
	}
	if billingBase == "" {
		panic("remote relay auth: a billing base URL is required")
	}
	return &relayAuth{
		local:   local,
		billing: strings.TrimRight(billingBase, "/"),
		client:  &http.Client{Timeout: relayAccountTimeout},
		now:     time.Now,
		cache:   map[[sha256.Size]byte]relayAccountEntry{},
	}
}

func (a *relayAuth) ValidateToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("no token")
	}
	if id, err := a.local.ValidateToken(token); err == nil && id != "" {
		return id, nil
	}
	key := sha256.Sum256([]byte(token))
	if id, ok := a.cached(key); ok {
		return id, nil
	}
	id, err := a.account(token)
	if err != nil {
		return "", err
	}
	a.remember(key, id)
	return id, nil
}

// account asks billing who the token is. Only a clear answer lets a
// terminal in: an unreachable billing service refuses, it never waves through.
func (a *relayAuth) account(token string) (string, error) {
	workspace, p, err := billingAccount(a.client, a.billing, token)
	if err != nil {
		return "", err
	}
	if p == plan.Free {
		return "", errRemoteIsPro
	}
	return relayAccountPrefix + workspace, nil
}

func (a *relayAuth) cached(key [sha256.Size]byte) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.cache[key]
	if !ok || !a.now().Before(e.until) {
		delete(a.cache, key)
		return "", false
	}
	return e.id, true
}

func (a *relayAuth) remember(key [sha256.Size]byte, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if len(a.cache) >= relayAccountCacheMax {
		for k, e := range a.cache {
			if !now.Before(e.until) {
				delete(a.cache, k)
			}
		}
		if len(a.cache) >= relayAccountCacheMax {
			a.cache = map[[sha256.Size]byte]relayAccountEntry{}
		}
	}
	a.cache[key] = relayAccountEntry{id: id, until: now.Add(relayAccountTTL)}
}
