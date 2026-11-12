package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/logs"
	"memdoor/pkg/auth"
	"memdoor/pkg/authorization"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"

	_ "memdoor/pkg/sqlitedriver"
)

type dbFactory struct {
	repository.RepositoryFactory
	db *sql.DB
}

func (f dbFactory) DB() interface{} { return f.db }

// sessionFake records the user a session was opened for.
type sessionFake struct {
	auth.Service
	opened string
}

// adminsFake answers GetUserByID with each user's role and workspace.
type adminsFake struct {
	auth.Repository
	users map[string]auth.User
}

func (f adminsFake) GetUserByID(_ context.Context, id string) (*auth.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, context.Canceled
	}
	return &u, nil
}

func (f *sessionFake) OpenSessionFor(_ context.Context, userID string) (*auth.User, string, error) {
	f.opened = userID
	u := &auth.User{}
	u.ID, u.Email = userID, userID+"@test"
	return u, "tok-" + userID, nil
}

func hostLoginServer(t *testing.T) (*Server, *sessionFake) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, workspace_id TEXT, created_at TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users VALUES ('human:first', 'first@test', 'ws', '2026-01-01'), ('human:peer', 'peer@test', 'ws', '2026-02-01'), ('human:other', 'other@test', 'ws2', '2026-03-01')`); err != nil {
		t.Fatal(err)
	}
	admin := func(id, ws string) auth.User {
		u := auth.User{}
		u.ID, u.WorkspaceID, u.Role = id, ws, domain.UserRoleAdmin
		return u
	}
	authz := authorization.NewService(&mockMembershipRepo{}, &mockChannelRepo{}, &mockBuddyRepo{}, adminsFake{users: map[string]auth.User{
		"human:first": admin("human:first", "ws"), "human:peer": admin("human:peer", "ws"), "human:other": admin("human:other", "ws2"),
	}})
	fake := &sessionFake{}
	ensureHostKey()
	return &Server{log: logs.New("test"), repoFactory: dbFactory{db: db}, authAdapter: NewAuthGatewayAdapter(fake), authzService: authz}, fake
}

func hostLogin(s *Server, from, key, email string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"key": key, "email": email})
	r := httptest.NewRequest(http.MethodPost, "/api/auth/host-login", strings.NewReader(string(body)))
	r.RemoteAddr = from
	w := httptest.NewRecorder()
	s.handleHostLogin(w, r)
	return w
}

// The key the gateway wrote signs in its first account, from its own machine
// only; a wrong key or a request over the network is refused.
func TestHostKeySignsInTheFirstAccountFromTheMachineOnly(t *testing.T) {
	s, fake := hostLoginServer(t)
	key := readHostKey()
	if len(key) != 64 {
		t.Fatalf("the key was not written: %q", key)
	}
	testenv.OwnerOnly(t, filepath.Join(os.Getenv("HOME"), ".memdoor", hostKeyFile)) // the owner's only

	if w := hostLogin(s, "10.0.0.7:4000", key, ""); w.Code != http.StatusForbidden {
		t.Fatalf("over the network: want 403, got %d %s", w.Code, w.Body)
	}
	if w := hostLogin(s, "127.0.0.1:4000", strings.Repeat("0", 64), ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: want 401, got %d %s", w.Code, w.Body)
	}
	if fake.opened != "" {
		t.Fatalf("a session was opened on a refused request: %q", fake.opened)
	}

	w := hostLogin(s, "127.0.0.1:4000", key, "")
	if w.Code != http.StatusOK || fake.opened != "human:first" {
		t.Fatalf("the right key from the machine: want the first account, got %d %s (opened %q)", w.Code, w.Body, fake.opened)
	}
	var resp AuthResponse
	if json.Unmarshal(w.Body.Bytes(), &resp) != nil || resp.Token != "tok-human:first" {
		t.Fatalf("the session is answered: %s", w.Body)
	}
	// The account the CLI was signed in as is kept when it is the gateway's
	// operator too (an admin of the first account's workspace); an admin of
	// another workspace, a sign-up of its own, is not: the first account is.
	if w := hostLogin(s, "127.0.0.1:4000", key, "peer@test"); w.Code != http.StatusOK || fake.opened != "human:peer" {
		t.Fatalf("naming a fellow operator: want that account, got %d (opened %q)", w.Code, fake.opened)
	}
	if w := hostLogin(s, "127.0.0.1:4000", key, "other@test"); w.Code != http.StatusOK || fake.opened != "human:first" {
		t.Fatalf("naming a stranger: want the first account, got %d (opened %q)", w.Code, fake.opened)
	}
}

// A second boot keeps the key: the CLI's copy stays good.
func TestHostKeyIsKeptAcrossBoots(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	ensureHostKey()
	first := readHostKey()
	ensureHostKey()
	if first == "" || readHostKey() != first {
		t.Fatalf("the key changed: %q -> %q", first, readHostKey())
	}
}
