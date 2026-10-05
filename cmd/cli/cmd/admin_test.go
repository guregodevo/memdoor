package cmd

import (
	"context"
	"errors"
	"testing"

	"memdoor/pkg/auth"
)

type fakeUserLookup struct {
	user *auth.User
	err  error
	// byEmail, when set, answers per address (adoption asks for two).
	byEmail map[string]*auth.User
	moved   map[string]string
}

func (f *fakeUserLookup) GetUserByEmail(_ context.Context, email string) (*auth.User, string, error) {
	if f.byEmail != nil {
		if u, ok := f.byEmail[email]; ok {
			return u, "", nil
		}
		return nil, "", errors.New("no such user")
	}
	if f.err != nil {
		return nil, "", f.err
	}
	return f.user, "", nil
}

func (f *fakeUserLookup) SetUserEmail(_ context.Context, userID, email string) error {
	if f.moved == nil {
		f.moved = map[string]string{}
	}
	f.moved[userID] = email
	return nil
}

type fakeSetter struct {
	gotID, gotPw string
	calls        int
	err          error
}

func (f *fakeSetter) AdminSetPassword(_ context.Context, userID, newPassword string) error {
	f.calls++
	f.gotID, f.gotPw = userID, newPassword
	return f.err
}

func userWithID(id string) *auth.User {
	u := &auth.User{}
	u.ID = id
	return u
}

// Adoption moves the engine's user to the account's address and sets the
// password on THAT user (the id is what owns the workspaces); when the
// account already has a user here, that one is taken and nothing moves.
func TestAdoptUserMovesTheEngineUserToTheAccount(t *testing.T) {
	engine := userWithID("u-engine")
	engine.Email = "greg@test.local"
	repo := &fakeUserLookup{byEmail: map[string]*auth.User{"greg@test.local": engine}}
	setter := &fakeSetter{}
	if err := adoptUser(context.Background(), repo, repo, setter, "greg@test.local", "greg@gmail.com", "generated-pass"); err != nil {
		t.Fatal(err)
	}
	if repo.moved["u-engine"] != "greg@gmail.com" {
		t.Fatalf("the engine's user must be moved to the account, got %v", repo.moved)
	}
	if setter.gotID != "u-engine" || setter.gotPw != "generated-pass" {
		t.Fatalf("the password goes on the moved user: %q/%q", setter.gotID, setter.gotPw)
	}

	own := userWithID("u-own")
	own.Email = "coach@example.com"
	repo = &fakeUserLookup{byEmail: map[string]*auth.User{"greg@test.local": engine, "coach@example.com": own}}
	setter = &fakeSetter{}
	if err := adoptUser(context.Background(), repo, repo, setter, "greg@test.local", "coach@example.com", "generated-pass"); err != nil {
		t.Fatal(err)
	}
	if len(repo.moved) != 0 || setter.gotID != "u-own" {
		t.Fatalf("the account's own user is taken as is: moved=%v password on %q", repo.moved, setter.gotID)
	}

	if err := adoptUser(context.Background(), repo, repo, setter, "", "nobody@example.com", "generated-pass"); err == nil {
		t.Fatal("no engine user and no account user: adoption must fail, not invent one")
	}
}
