package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// A BROWSER SIGN-IN (Sign in with ChatGPT, Sign in with OpenRouter) runs here,
// in the gateway: its loopback listener is the gateway's, and the CLI or the
// window opens the link and polls. Each provider's handler starts its own
// (POST {"action":"login"} → {id, auth_url}); waiting, a pasted link and
// cancelling are the same for all:
//
//   POST … {"action":"login/wait","id"}         → the finish reply | {pending} | {error, again}
//   POST … {"action":"login/paste","id","value"} → {delivered}
//   POST … {"action":"login/cancel","id"}        → {cancelled}

type signIn struct {
	deliver func(string) error
	cancel  context.CancelFunc
	done    chan signInOutcome
}

// signInOutcome is how a sign-in ended: the reply once signed in (finish),
// or why not; again asks the caller to start a fresh one.
type signInOutcome struct {
	finish func(ctx context.Context) any
	err    error
	again  bool
}

var signIns = struct {
	sync.Mutex
	m map[string]*signIn
}{m: map[string]*signIn{}}

// signInWaitFor is how long one login/wait holds before answering pending.
var signInWaitFor = 25 * time.Second

// startSignIn keeps a sign-in under a new id; wait runs until the browser
// answers, the sign-in is cancelled, or it times out.
func startSignIn(deliver func(string) error, wait func(ctx context.Context) signInOutcome) string {
	ctx, cancel := context.WithCancel(context.Background())
	s := &signIn{deliver: deliver, cancel: cancel, done: make(chan signInOutcome, 1)}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	signIns.Lock()
	signIns.m[id] = s
	signIns.Unlock()
	go func() { s.done <- wait(ctx) }()
	return id
}

// signInStep answers login/wait, login/paste and login/cancel for any
// provider's sign-in.
func signInStep(w http.ResponseWriter, r *http.Request, action, id, value string) {
	ok := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	fail := func(status int, err error) { http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), status) }
	signIns.Lock()
	s := signIns.m[id]
	signIns.Unlock()
	if s == nil {
		fail(http.StatusNotFound, fmt.Errorf("no sign-in %q in progress", id))
		return
	}
	switch action {
	case "login/paste":
		if err := s.deliver(value); err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(map[string]bool{"delivered": true})
	case "login/cancel":
		s.cancel()
		ok(map[string]bool{"cancelled": true})
	default:
		select {
		case out := <-s.done:
			signIns.Lock()
			delete(signIns.m, id)
			signIns.Unlock()
			s.cancel()
			if out.err != nil {
				ok(map[string]any{"error": out.err.Error(), "again": out.again})
				return
			}
			ok(out.finish(r.Context()))
		case <-time.After(signInWaitFor):
			ok(map[string]bool{"pending": true})
		case <-r.Context().Done():
		}
	}
}
