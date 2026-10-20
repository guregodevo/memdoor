package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// memRepo is the part of the repository a verified-email sign-in touches.
type memRepo struct {
	Repository
	users    map[string]*User // by email
	verified map[string]bool
	sessions int
}

func newMemRepo() *memRepo {
	return &memRepo{users: map[string]*User{}, verified: map[string]bool{}}
}

func (r *memRepo) GetUserByEmail(_ context.Context, email string) (*User, string, error) {
	if u, ok := r.users[email]; ok {
		return u, "hash", nil
	}
	return nil, "", errors.New("not found")
}

func (r *memRepo) EmailExists(_ context.Context, email string) (bool, error) {
	_, ok := r.users[email]
	return ok, nil
}

func (r *memRepo) GetUserByUsername(_ context.Context, username string) (*User, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *memRepo) CreateUser(_ context.Context, u *User, _ string) error {
	r.users[u.Email] = u
	return nil
}

func (r *memRepo) MarkEmailVerified(_ context.Context, id string) error {
	r.verified[id] = true
	return nil
}

func (r *memRepo) CreateSession(_ context.Context, _ *Session, _ string) error {
	r.sessions++
	return nil
}

func (r *memRepo) CreateVerificationToken(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

func (r *memRepo) UpdateLastLogin(_ context.Context, _ string) error { return nil }

type quietLogger struct{}

func (quietLogger) Debug(string, ...any) {}
func (quietLogger) Info(string, ...any)  {}
func (quietLogger) Warn(string, ...any)  {}
func (quietLogger) Error(string, ...any) {}

func TestLoginVerifiedEmailMakesTheAccountOnFirstSightAndSignsInAfter(t *testing.T) {
	repo := newMemRepo()
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "https://memdoor.ai", "ws-1")

	var asked string
	workspaceFor := func(_ context.Context, username string) (string, error) {
		asked = username
		return "ws-" + username, nil
	}
	u, token, _, err := s.LoginVerifiedEmail(context.Background(), "Jane.Doe+dev@example.com", "Jane Doe", workspaceFor)
	if err != nil || token == "" {
		t.Fatalf("first sight: %v %q", err, token)
	}
	if !u.EmailVerified || u.Username != "jane-doe-dev" || u.Name != "Jane Doe" || u.WorkspaceID != "ws-jane-doe-dev" || asked != "jane-doe-dev" {
		t.Fatalf("the new account: %+v", u.BaseUser)
	}
	again, token2, created, err := s.LoginVerifiedEmail(context.Background(), "Jane.Doe+dev@example.com", "", workspaceFor)
	if err != nil || token2 == "" || created || again.ID != u.ID || len(repo.users) != 1 {
		t.Fatalf("second time signs the same account in: %v %+v", err, again)
	}
	if repo.sessions != 2 {
		t.Fatalf("two sessions, got %d", repo.sessions)
	}
	if _, _, _, err := s.LoginVerifiedEmail(context.Background(), "no-at-sign", "", nil); err == nil {
		t.Fatal("no address, no account")
	}
}

func TestLoginVerifiedEmailVerifiesAKnownUnverifiedAccount(t *testing.T) {
	repo := newMemRepo()
	existing := &User{BaseUser: domain.BaseUser{ID: "human:1", Email: "a@example.com", Username: "a", EmailVerified: false}}
	repo.users[existing.Email] = existing
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "", "")
	u, _, created, err := s.LoginVerifiedEmail(context.Background(), "a@example.com", "A", nil)
	if err != nil || created || !u.EmailVerified || !repo.verified["human:1"] || len(repo.users) != 1 {
		t.Fatalf("the provider's word verifies the address: %v %+v %v", err, u, repo.verified)
	}
}

func TestUsernameFromEmailAndCollisions(t *testing.T) {
	cases := map[string]string{
		"john@example.com":  "john",
		"J.R.R@example.com": "j-r-r",
		"a@example.com":     "user-a",
		"-_x_-@example.com": "user-x",
		"averyveryverylongaddressthatgoesonandon@example.com": "averyveryverylongaddressthatgo",
	}
	for in, want := range cases {
		if got := UsernameFromEmail(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
	repo := newMemRepo()
	repo.users["x@a.com"] = &User{BaseUser: domain.BaseUser{Email: "x@a.com", Username: "john"}}
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "", "").(*service)
	got := s.freeUsername(context.Background(), "john")
	if got == "john" || !strings.HasPrefix(got, "john-") || len(got) != len("john-")+4 {
		t.Fatalf("a taken name gets a short suffix: %q", got)
	}
}

// A sign-up with an email and a password and nothing else gets a username
// from the address and that username as its name (what the CLI's --help
// promised and the service refused, 2026-10-20).
func TestRegisterDerivesUsernameAndNameFromTheEmail(t *testing.T) {
	repo := newMemRepo()
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "", "ws-1")
	u, err := s.Register(context.Background(), "Bob.Builder@example.com", "", "longenough1", "", "ws-bob")
	if err != nil || u.Username != "bob-builder" || u.Name != "bob-builder" || u.WorkspaceID != "ws-bob" {
		t.Fatalf("derived: %v %+v", err, u)
	}
	again, err := s.Register(context.Background(), "bob.builder@other.example", "", "longenough1", "", "ws-2")
	if err != nil || again.Username == "bob-builder" || !strings.HasPrefix(again.Username, "bob-builder-") {
		t.Fatalf("a taken username is made free: %v %+v", err, again)
	}
	if _, err := s.Register(context.Background(), "x@example.com", "taken", "longenough1", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(context.Background(), "y@example.com", "taken", "longenough1", "", ""); err == nil {
		t.Fatal("a username asked for by name is not taken from someone else")
	}
}
