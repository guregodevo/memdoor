package gateway

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	marioapi "github.com/guregodevo/mario/api"
)

// Where a run's state lives is decided by the account: none or free → the
// local table (nil here); Pro → mario-state on memdoor.ai; Pro with a store
// that does not answer → the run does not start, and never goes local.
func TestWhereARunsStateLives(t *testing.T) {
	billing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "pro":
			_, _ = w.Write([]byte(`{"workspace":"acme","plan":"pro"}`))
		case "free":
			_, _ = w.Write([]byte(`{"workspace":"solo","plan":"free"}`))
		default:
			http.Error(w, "who?", http.StatusUnauthorized)
		}
	}))
	defer billing.Close()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer up.Close()
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()

	tok := func(v string) func() string { return func() string { return v } }
	fresh := func(billingURL, stateURL, token string) *hostedState {
		h := newHostedState(billingURL, stateURL, tok(token))
		h.planPath = filepath.Join(t.TempDir(), "plan")
		return h
	}

	if repo, err := fresh(billing.URL, up.URL, "").repository(); repo != nil || err != nil {
		t.Fatalf("no account: local, got %v %v", repo, err)
	}
	if repo, err := fresh(billing.URL, up.URL, "free").repository(); repo != nil || err != nil {
		t.Fatalf("free: local, got %v %v", repo, err)
	}
	h := fresh(billing.URL, up.URL, "pro")
	repo, err := h.repository()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.(*marioapi.RESTWorkflowRepository); !ok {
		t.Fatalf("pro: the REST repository, got %T", repo)
	}
	if again, _ := h.repository(); again != repo {
		t.Fatal("the hosted repository is opened once and kept")
	}
	_, err = fresh(billing.URL, downURL, "pro").repository()
	if err == nil || !strings.Contains(err.Error(), downURL) || !strings.Contains(err.Error(), "does not start") {
		t.Fatalf("pro with the store down must refuse the run and name the store: %v", err)
	}
	// A store that answers but not OK (503) is down too.
	sick := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer sick.Close()
	if _, err := fresh(billing.URL, sick.URL, "pro").repository(); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("pro with the store answering 503 must refuse the run: %v", err)
	}

	// A new token on the same gateway is a new account: nothing cached
	// carries over, and the repository is rebuilt with the new token.
	var current = "pro"
	h2 := newHostedState(billing.URL, up.URL, func() string { return current })
	h2.planPath = filepath.Join(t.TempDir(), "plan")
	first, _ := h2.repository()
	current = "free"
	if repo, err := h2.repository(); repo != nil || err != nil {
		t.Fatalf("after the token changed to a free account: local, got %v %v", repo, err)
	}
	current = "pro"
	if again, _ := h2.repository(); again == first {
		t.Fatal("a token that changed must not keep the repository built with the old one")
	}

	// BILLING DOWN: the plan it last named stands — pro stays strict, free
	// stays local — and an account it never named cannot start a run.
	gone := httptest.NewServer(http.NotFoundHandler())
	goneURL := gone.URL
	gone.Close()
	remembered := fresh(billing.URL, up.URL, "pro")
	if _, err := remembered.repository(); err != nil {
		t.Fatal(err)
	}
	later := newHostedState(goneURL, up.URL, tok("pro"))
	later.planPath = remembered.planPath
	if repo, err := later.repository(); err != nil || repo == nil {
		t.Fatalf("pro remembered on disk, billing down: the hosted store, got %v %v", repo, err)
	}
	freeRemembered := fresh(billing.URL, up.URL, "free")
	_, _ = freeRemembered.repository()
	laterFree := newHostedState(goneURL, up.URL, tok("free"))
	laterFree.planPath = freeRemembered.planPath
	if repo, err := laterFree.repository(); err != nil || repo != nil {
		t.Fatalf("free remembered on disk, billing down: local, got %v %v", repo, err)
	}
	if _, err := fresh(goneURL, up.URL, "pro").repository(); err == nil || !strings.Contains(err.Error(), "plan is not known") {
		t.Fatalf("billing down and never answered: the run must not start, got %v", err)
	}
	// THE PLAN ON DISK BELONGS TO THE ACCOUNT: with billing down, a new
	// token on a machine where a Pro account left its plan must not inherit
	// it (the review workflow found this by running a probe, 2026-10-05).
	other := newHostedState(goneURL, up.URL, tok("someone-else"))
	other.planPath = remembered.planPath
	if _, err := other.repository(); err == nil || !strings.Contains(err.Error(), "plan is not known") {
		t.Fatalf("another account must not inherit the remembered plan, got %v", err)
	}
	// A gateway nobody wired (tests, &Server{}) is local only.
	var none *hostedState
	if repo, err := none.repository(); repo != nil || err != nil {
		t.Fatalf("nil hosted state: local, got %v %v", repo, err)
	}
}
