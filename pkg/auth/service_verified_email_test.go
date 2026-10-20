package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func (r *memRepo) UpdateLastLogin(_ context.Context, _ string) error { return nil }

type quietLogger struct{}

func (quietLogger) Debug(string, ...any) {}
func (quietLogger) Info(string, ...any)  {}
func (quietLogger) Warn(string, ...any)  {}
func (quietLogger) Error(string, ...any) {}

func TestLoginVerifiedEmailMakesTheAccountOnFirstSightAndSignsInAfter(t *testing.T) {
	repo := newMemRepo()
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "https://memdoor.ai", "ws-1")

	u, token, err := s.LoginVerifiedEmail(context.Background(), "Jane.Doe+dev@example.com", "Jane Doe")
	if err != nil || token == "" {
		t.Fatalf("first sight: %v %q", err, token)
	}
	if !u.EmailVerified || u.Username != "jane-doe-dev" || u.Name != "Jane Doe" || u.WorkspaceID != "ws-1" {
		t.Fatalf("the new account: %+v", u.BaseUser)
	}
	again, token2, err := s.LoginVerifiedEmail(context.Background(), "Jane.Doe+dev@example.com", "")
	if err != nil || token2 == "" || again.ID != u.ID || len(repo.users) != 1 {
		t.Fatalf("second time signs the same account in: %v %+v", err, again)
	}
	if repo.sessions != 2 {
		t.Fatalf("two sessions, got %d", repo.sessions)
	}
	if _, _, err := s.LoginVerifiedEmail(context.Background(), "no-at-sign", ""); err == nil {
		t.Fatal("no address, no account")
	}
}

func TestLoginVerifiedEmailVerifiesAKnownUnverifiedAccount(t *testing.T) {
	repo := newMemRepo()
	existing := &User{BaseUser: domain.BaseUser{ID: "human:1", Email: "a@example.com", Username: "a", EmailVerified: false}}
	repo.users[existing.Email] = existing
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "", "")
	u, _, err := s.LoginVerifiedEmail(context.Background(), "a@example.com", "A")
	if err != nil || !u.EmailVerified || !repo.verified["human:1"] || len(repo.users) != 1 {
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
		if got := usernameFromEmail(in); got != want {
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
