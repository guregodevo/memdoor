package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// sessRepo keeps sessions by token hash and users by id, for VerifyToken.
type sessRepo struct {
	*memRepo
	byHash map[string]*Session
	byID   map[string]*User
}

func (r *sessRepo) CreateSession(_ context.Context, s *Session, hash string) error {
	r.byHash[hash] = s
	return nil
}

func (r *sessRepo) GetSessionByToken(_ context.Context, hash string) (*Session, error) {
	if s, ok := r.byHash[hash]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (r *sessRepo) GetUserByID(_ context.Context, id string) (*User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

// A FREE ACCOUNT NEVER SIGNS IN (#88): the machine's own user's session does
// not run out, and one issued with 30 days before that is still taken; any
// other user's lapses at 30 days as before. Mutation check: drop
// isMachineUser from VerifyToken and the lapsed machine session is refused.
func TestTheMachinesOwnSessionDoesNotRunOut(t *testing.T) {
	saved := MachineUser
	t.Cleanup(func() { MachineUser = saved })
	MachineUser = func() string { return "Greg@localhost" }

	repo := &sessRepo{memRepo: newMemRepo(), byHash: map[string]*Session{}, byID: map[string]*User{}}
	own := &User{BaseUser: domain.BaseUser{ID: "u-own", Email: "greg@localhost"}}
	hand := &User{BaseUser: domain.BaseUser{ID: "u-hand", Email: "greg@gmail.com"}}
	repo.byID[own.ID], repo.byID[hand.ID] = own, hand
	s := NewService(repo, nil, NewTokenGenerator(), quietLogger{}, "", "ws").(*service)

	ownToken, err := s.openSession(context.Background(), own)
	if err != nil {
		t.Fatal(err)
	}
	if exp := repo.byHash[s.tokenGenerator.HashToken(ownToken)].ExpiresAt; exp.Before(time.Now().Add(50 * 365 * 24 * time.Hour)) {
		t.Fatalf("the machine's own session expires %v", exp)
	}
	handToken, _ := s.openSession(context.Background(), hand)
	if exp := repo.byHash[s.tokenGenerator.HashToken(handToken)].ExpiresAt; exp.After(time.Now().Add(SessionTTL + time.Minute)) {
		t.Fatalf("an account made by hand keeps 30 days: %v", exp)
	}

	// Both lapsed, as a session from before this change would be.
	for _, sess := range repo.byHash {
		sess.ExpiresAt = time.Now().Add(-time.Hour)
	}
	if u, err := s.VerifyToken(context.Background(), ownToken); err != nil || u.ID != own.ID {
		t.Fatalf("a lapsed session of the machine's own user is taken: %v", err)
	}
	if _, err := s.VerifyToken(context.Background(), handToken); err == nil {
		t.Fatal("a lapsed session of an account made by hand is refused")
	}

	MachineUser = func() string { return "" }
	if _, err := s.VerifyToken(context.Background(), ownToken); err == nil {
		t.Fatal("with no generated login on the machine, a lapsed session is refused")
	}
}
