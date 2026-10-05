package auth

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// ---------- tokens.go: pure crypto helpers ----------

func TestTokenGenerator(t *testing.T) {
	tg := NewTokenGenerator()

	t.Run("GenerateToken is unique and non-trivial", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			tok, err := tg.GenerateToken()
			if err != nil {
				t.Fatalf("GenerateToken: %v", err)
			}
			if len(tok) < 32 {
				t.Fatalf("token too short: %q", tok)
			}
			if seen[tok] {
				t.Fatalf("duplicate token generated: %q", tok)
			}
			seen[tok] = true
		}
	})

	t.Run("HashToken is deterministic and not the identity", func(t *testing.T) {
		h1 := tg.HashToken("abc")
		h2 := tg.HashToken("abc")
		if h1 != h2 {
			t.Errorf("HashToken not deterministic: %q vs %q", h1, h2)
		}
		if h1 == "abc" {
			t.Error("HashToken returned the plaintext")
		}
		if tg.HashToken("abc") == tg.HashToken("abd") {
			t.Error("different tokens hashed to the same value")
		}
	})

	t.Run("password hash round-trips and rejects wrong password", func(t *testing.T) {
		hash, err := tg.HashPassword("correct horse battery staple")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if hash == "correct horse battery staple" {
			t.Error("password stored in plaintext")
		}
		if !tg.CheckPassword("correct horse battery staple", hash) {
			t.Error("CheckPassword rejected the correct password")
		}
		if tg.CheckPassword("wrong", hash) {
			t.Error("CheckPassword accepted a wrong password")
		}
	})
}

// ---------- in-memory Repository fake ----------

type verifRec struct {
	userID    string
	expiresAt time.Time
}
type resetRec struct {
	userID    string
	expiresAt time.Time
	used      bool
}

type fakeRepo struct {
	users       map[string]*User  // id -> user
	byEmail     map[string]string // email -> id
	byUsername  map[string]string // username -> id
	pw          map[string]string // id -> password hash
	sessions    map[string]*Session
	verifTokens map[string]verifRec
	resetTokens map[string]resetRec
	lastLogin   map[string]time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:       map[string]*User{},
		byEmail:     map[string]string{},
		byUsername:  map[string]string{},
		pw:          map[string]string{},
		sessions:    map[string]*Session{},
		verifTokens: map[string]verifRec{},
		resetTokens: map[string]resetRec{},
		lastLogin:   map[string]time.Time{},
	}
}

func (r *fakeRepo) CreateUser(_ context.Context, u *User, passwordHash string) error {
	if _, ok := r.byEmail[u.Email]; ok {
		return fmt.Errorf("email exists")
	}
	r.users[u.ID] = u
	r.byEmail[u.Email] = u.ID
	r.byUsername[u.Username] = u.ID
	r.pw[u.ID] = passwordHash
	return nil
}

func (r *fakeRepo) GetUserByEmail(_ context.Context, email string) (*User, string, error) {
	id, ok := r.byEmail[email]
	if !ok {
		return nil, "", sql.ErrNoRows
	}
	return r.users[id], r.pw[id], nil
}

func (r *fakeRepo) GetUserByUsername(_ context.Context, username string) (*User, error) {
	id, ok := r.byUsername[username]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return r.users[id], nil
}

func (r *fakeRepo) GetUserByID(_ context.Context, id string) (*User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (r *fakeRepo) UpdateUser(_ context.Context, u *User) error { r.users[u.ID] = u; return nil }

func (r *fakeRepo) SetUserEmail(_ context.Context, id, email string) error {
	if u, ok := r.users[id]; ok {
		u.Email = email
	}
	return nil
}
func (r *fakeRepo) MarkEmailVerified(_ context.Context, id string) error {
	if u, ok := r.users[id]; ok {
		u.EmailVerified = true
	}
	return nil
}

func (r *fakeRepo) CreateSession(_ context.Context, s *Session, tokenHash string) error {
	r.sessions[tokenHash] = s
	return nil
}

func (r *fakeRepo) GetSessionByToken(_ context.Context, tokenHash string) (*Session, error) {
	s, ok := r.sessions[tokenHash]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return s, nil
}

func (r *fakeRepo) DeleteSession(_ context.Context, tokenHash string) error {
	delete(r.sessions, tokenHash)
	return nil
}

func (r *fakeRepo) DeleteAllUserSessions(_ context.Context, userID string) error {
	for h, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, h)
		}
	}
	return nil
}

func (r *fakeRepo) UpdateLastLogin(_ context.Context, userID string) error {
	r.lastLogin[userID] = time.Now()
	return nil
}

func (r *fakeRepo) CreateVerificationToken(_ context.Context, userID, token string, exp time.Time) error {
	r.verifTokens[token] = verifRec{userID, exp}
	return nil
}

func (r *fakeRepo) GetVerificationToken(_ context.Context, token string) (string, time.Time, error) {
	v, ok := r.verifTokens[token]
	if !ok {
		return "", time.Time{}, sql.ErrNoRows
	}
	return v.userID, v.expiresAt, nil
}

func (r *fakeRepo) DeleteVerificationToken(_ context.Context, token string) error {
	delete(r.verifTokens, token)
	return nil
}

func (r *fakeRepo) DeleteUserVerificationTokens(_ context.Context, userID string) error {
	for t, v := range r.verifTokens {
		if v.userID == userID {
			delete(r.verifTokens, t)
		}
	}
	return nil
}

func (r *fakeRepo) CreatePasswordResetToken(_ context.Context, userID, token string, exp time.Time) error {
	r.resetTokens[token] = resetRec{userID: userID, expiresAt: exp}
	return nil
}

func (r *fakeRepo) GetPasswordResetToken(_ context.Context, token string) (string, time.Time, bool, error) {
	v, ok := r.resetTokens[token]
	if !ok {
		return "", time.Time{}, false, sql.ErrNoRows
	}
	return v.userID, v.expiresAt, v.used, nil
}

func (r *fakeRepo) MarkPasswordResetTokenUsed(_ context.Context, token string) error {
	if v, ok := r.resetTokens[token]; ok {
		v.used = true
		r.resetTokens[token] = v
	}
	return nil
}

func (r *fakeRepo) DeletePasswordResetToken(_ context.Context, token string) error {
	delete(r.resetTokens, token)
	return nil
}

func (r *fakeRepo) UpdatePassword(_ context.Context, userID, passwordHash string) error {
	r.pw[userID] = passwordHash
	return nil
}

func (r *fakeRepo) EmailExists(_ context.Context, email string) (bool, error) {
	_, ok := r.byEmail[email]
	return ok, nil
}

// ---------- stub logger ----------

type stubLogger struct{}

func (stubLogger) Info(string, ...any)  {}
func (stubLogger) Warn(string, ...any)  {}
func (stubLogger) Error(string, ...any) {}
func (stubLogger) Debug(string, ...any) {}

func newTestService() (*fakeRepo, Service) {
	repo := newFakeRepo()
	svc := NewService(repo, nil, NewTokenGenerator(), stubLogger{}, "https://example.test", "ws-test")
	return repo, svc
}

// ---------- service: Register ----------

func TestRegister_Validation(t *testing.T) {
	_, svc := newTestService()
	ctx := context.Background()

	cases := []struct {
		name                          string
		email, username, pass, dnName string
	}{
		{"missing email", "", "alice", "password1", "Alice"},
		{"missing username", "a@b.c", "", "password1", "Alice"},
		{"missing password", "a@b.c", "alice", "", "Alice"},
		{"missing name", "a@b.c", "alice", "password1", ""},
		{"short password", "a@b.c", "alice", "short", "Alice"},
		{"username too short", "a@b.c", "ab", "password1", "Alice"},
		{"username bad chars", "a@b.c", "alice!", "password1", "Alice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Register(ctx, c.email, c.username, c.pass, c.dnName); err == nil {
				t.Errorf("expected validation error, got nil")
			}
		})
	}
}

func TestRegister_HashesPasswordAndRejectsDuplicates(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()

	u, err := svc.Register(ctx, "alice@example.test", "alice", "password1", "Alice")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.ID == "" || u.WorkspaceID != "ws-test" {
		t.Errorf("unexpected user: id=%q ws=%q", u.ID, u.WorkspaceID)
	}

	// Password must be stored hashed, never plaintext.
	stored := repo.pw[u.ID]
	if stored == "password1" || stored == "" {
		t.Fatalf("password not hashed: %q", stored)
	}
	if !NewTokenGenerator().CheckPassword("password1", stored) {
		t.Error("stored hash does not verify against the original password")
	}

	// Duplicate email rejected.
	if _, err := svc.Register(ctx, "alice@example.test", "alice2", "password1", "Alice"); err == nil {
		t.Error("expected duplicate-email rejection")
	}
	// Duplicate username rejected.
	if _, err := svc.Register(ctx, "other@example.test", "alice", "password1", "Alice"); err == nil {
		t.Error("expected duplicate-username rejection")
	}
}

// ---------- service: Login + VerifyToken ----------

func TestLogin_And_VerifyToken(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "bob@example.test", "bob", "password1", "Bob"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Run("wrong password is rejected", func(t *testing.T) {
		if _, _, err := svc.Login(ctx, "bob@example.test", "nope"); err == nil {
			t.Error("expected login failure on wrong password")
		}
	})

	t.Run("unknown email is rejected", func(t *testing.T) {
		if _, _, err := svc.Login(ctx, "ghost@example.test", "password1"); err == nil {
			t.Error("expected login failure on unknown email")
		}
	})

	t.Run("correct login issues a token stored only as a hash", func(t *testing.T) {
		user, token, err := svc.Login(ctx, "bob@example.test", "password1")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if token == "" {
			t.Fatal("empty token")
		}
		// The raw token must NOT be a session key; its hash must be.
		if _, raw := repo.sessions[token]; raw {
			t.Error("raw token was stored as a session key")
		}
		if _, ok := repo.sessions[NewTokenGenerator().HashToken(token)]; !ok {
			t.Error("session not stored under the token hash")
		}

		// VerifyToken accepts the issued token and returns the user.
		got, err := svc.VerifyToken(ctx, token)
		if err != nil || got.ID != user.ID {
			t.Errorf("VerifyToken(valid) = (%v, %v), want user %s", got, err, user.ID)
		}
	})

	t.Run("empty and bogus tokens are rejected", func(t *testing.T) {
		if _, err := svc.VerifyToken(ctx, ""); err == nil {
			t.Error("expected error for empty token")
		}
		if _, err := svc.VerifyToken(ctx, "not-a-real-token"); err == nil {
			t.Error("expected error for bogus token")
		}
	})

	t.Run("expired session is rejected", func(t *testing.T) {
		_, token, err := svc.Login(ctx, "bob@example.test", "password1")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		// Force the stored session to be expired.
		repo.sessions[NewTokenGenerator().HashToken(token)].ExpiresAt = time.Now().Add(-time.Hour)
		if _, err := svc.VerifyToken(ctx, token); err == nil {
			t.Error("expected expired-token rejection")
		}
	})
}

// ---------- service: password reset ----------

func TestSendPasswordReset_DoesNotLeakAccountExistence(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()

	// Unknown email returns nil (no enumeration) and creates no token.
	if err := svc.SendPasswordReset(ctx, "nobody@example.test"); err != nil {
		t.Errorf("SendPasswordReset(unknown) = %v, want nil", err)
	}
	if len(repo.resetTokens) != 0 {
		t.Errorf("a reset token was created for an unknown email: %d", len(repo.resetTokens))
	}
}

func TestResetPassword(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()
	u, _ := svc.Register(ctx, "carol@example.test", "carol", "password1", "Carol")

	seed := func(exp time.Time, used bool) string {
		tok := "reset-token"
		// The service stores/looks up reset tokens by hash (raw token only
		// ever leaves in the email), so seed under the hashed key.
		repo.resetTokens[NewTokenGenerator().HashToken(tok)] = resetRec{userID: u.ID, expiresAt: exp, used: used}
		return tok
	}

	t.Run("short password rejected", func(t *testing.T) {
		if err := svc.ResetPassword(ctx, seed(time.Now().Add(time.Hour), false), "short"); err == nil {
			t.Error("expected short-password rejection")
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		if err := svc.ResetPassword(ctx, seed(time.Now().Add(-time.Hour), false), "newpassword1"); err == nil {
			t.Error("expected expired-token rejection")
		}
	})

	t.Run("used token rejected", func(t *testing.T) {
		if err := svc.ResetPassword(ctx, seed(time.Now().Add(time.Hour), true), "newpassword1"); err == nil {
			t.Error("expected used-token rejection")
		}
	})

	t.Run("valid reset updates the password and invalidates sessions", func(t *testing.T) {
		// Give the user an active session first.
		_, token, _ := svc.Login(ctx, "carol@example.test", "password1")
		if _, err := svc.VerifyToken(ctx, token); err != nil {
			t.Fatalf("precondition: session should be valid: %v", err)
		}

		if err := svc.ResetPassword(ctx, seed(time.Now().Add(time.Hour), false), "newpassword1"); err != nil {
			t.Fatalf("ResetPassword: %v", err)
		}
		// New password works, old one doesn't.
		if !NewTokenGenerator().CheckPassword("newpassword1", repo.pw[u.ID]) {
			t.Error("new password not stored")
		}
		// Sessions invalidated → the old token no longer verifies.
		if _, err := svc.VerifyToken(ctx, token); err == nil {
			t.Error("old session still valid after password reset")
		}
	})
}

// ---------- service: change password (self-service) ----------

func TestChangePassword(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()
	u, _ := svc.Register(ctx, "dave@example.test", "dave", "password1", "Dave")

	t.Run("short new password rejected", func(t *testing.T) {
		if err := svc.ChangePassword(ctx, u.ID, "password1", "short"); err == nil {
			t.Error("expected short-password rejection")
		}
	})

	t.Run("wrong old password rejected", func(t *testing.T) {
		if err := svc.ChangePassword(ctx, u.ID, "wrongpass", "newpassword1"); err == nil {
			t.Error("expected rejection on wrong old password")
		}
		// Password must be unchanged.
		if !NewTokenGenerator().CheckPassword("password1", repo.pw[u.ID]) {
			t.Error("password changed despite wrong old password")
		}
	})

	t.Run("correct old password updates the password", func(t *testing.T) {
		// Give the user an active session; it must survive the change.
		_, token, _ := svc.Login(ctx, "dave@example.test", "password1")
		if _, err := svc.VerifyToken(ctx, token); err != nil {
			t.Fatalf("precondition: session should be valid: %v", err)
		}

		if err := svc.ChangePassword(ctx, u.ID, "password1", "newpassword1"); err != nil {
			t.Fatalf("ChangePassword: %v", err)
		}

		// New password verifies via the login/check path; old one does not.
		if _, _, err := svc.Login(ctx, "dave@example.test", "newpassword1"); err != nil {
			t.Errorf("new password does not log in: %v", err)
		}
		if _, _, err := svc.Login(ctx, "dave@example.test", "password1"); err == nil {
			t.Error("old password still logs in after change")
		}

		// Session preserved — the user stays logged in.
		if _, err := svc.VerifyToken(ctx, token); err != nil {
			t.Error("session was invalidated by self-service password change")
		}
	})
}

// ---------- service: admin set password ----------

func TestAdminSetPassword(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()
	u, _ := svc.Register(ctx, "erin@example.test", "erin", "password1", "Erin")

	t.Run("short password rejected", func(t *testing.T) {
		if err := svc.AdminSetPassword(ctx, u.ID, "short"); err == nil {
			t.Error("expected short-password rejection")
		}
	})

	t.Run("sets the hash and invalidates sessions", func(t *testing.T) {
		// Active session that must be invalidated by the admin reset.
		_, token, _ := svc.Login(ctx, "erin@example.test", "password1")
		if _, err := svc.VerifyToken(ctx, token); err != nil {
			t.Fatalf("precondition: session should be valid: %v", err)
		}

		if err := svc.AdminSetPassword(ctx, u.ID, "adminset123"); err != nil {
			t.Fatalf("AdminSetPassword: %v", err)
		}

		// New password verifies.
		if !NewTokenGenerator().CheckPassword("adminset123", repo.pw[u.ID]) {
			t.Error("new password not stored")
		}
		if _, _, err := svc.Login(ctx, "erin@example.test", "adminset123"); err != nil {
			t.Errorf("new password does not log in: %v", err)
		}

		// Existing sessions invalidated.
		if _, err := svc.VerifyToken(ctx, token); err == nil {
			t.Error("old session still valid after admin set-password")
		}
	})
}

// ---------- service: email verification ----------

func TestVerifyEmail(t *testing.T) {
	repo, svc := newTestService()
	ctx := context.Background()
	u, _ := svc.Register(ctx, "dave@example.test", "dave", "password1", "Dave")

	// Verification tokens are stored/looked up by hash (the raw token only
	// leaves in the email), so seed and assert under the hashed key.
	hash := func(tok string) string { return NewTokenGenerator().HashToken(tok) }

	t.Run("expired token rejected", func(t *testing.T) {
		repo.verifTokens[hash("v-expired")] = verifRec{u.ID, time.Now().Add(-time.Hour)}
		if err := svc.VerifyEmail(ctx, "v-expired"); err == nil {
			t.Error("expected expired verification token to be rejected")
		}
	})

	t.Run("valid token marks email verified and is consumed", func(t *testing.T) {
		repo.verifTokens[hash("v-good")] = verifRec{u.ID, time.Now().Add(time.Hour)}
		if err := svc.VerifyEmail(ctx, "v-good"); err != nil {
			t.Fatalf("VerifyEmail: %v", err)
		}
		if !repo.users[u.ID].EmailVerified {
			t.Error("email not marked verified")
		}
		if _, ok := repo.verifTokens[hash("v-good")]; ok {
			t.Error("verification token not consumed (should be one-time use)")
		}
	})
}
