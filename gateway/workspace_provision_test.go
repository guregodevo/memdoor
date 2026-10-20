package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// makerFake records the workspaces the sign-up handlers ask for.
type makerFake struct {
	made    []string // "name|hint"
	seeded  []string // "slug|name|owner"
	dropped []string
}

func (m *makerFake) NewWorkspace(_ context.Context, name, hint string) (string, error) {
	m.made = append(m.made, name+"|"+hint)
	return "ws-" + hint, nil
}

func (m *makerFake) DropWorkspace(_ context.Context, slug string) {
	m.dropped = append(m.dropped, slug)
}

func (m *makerFake) SeedWorkspace(_ context.Context, slug, name, owner string) {
	m.seeded = append(m.seeded, slug+"|"+name+"|"+owner)
}

// A sign-up with no invite gets a workspace of its own, named after the
// person, seeded with the person as its owner.
func TestASignUpGetsItsOwnWorkspace(t *testing.T) {
	fake := &registerFake{}
	maker := &makerFake{}
	a := NewAuthGatewayAdapter(fake)
	a.SetWorkspaces(maker)
	rec := postRegister(a, "198.51.100.30:1",
		`{"email":"jane@example.com","username":"jane","password":"longenough1","name":"Jane Doe"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-up: %d %s", rec.Code, rec.Body.String())
	}
	if len(maker.made) != 1 || maker.made[0] != "Jane Doe's workspace|jane" {
		t.Fatalf("the workspace made: %v", maker.made)
	}
	if len(fake.workspaces) != 1 || fake.workspaces[0] != "ws-jane" {
		t.Fatalf("the account lives in it: %v", fake.workspaces)
	}
	if len(maker.seeded) != 1 || maker.seeded[0] != "ws-jane|Jane Doe's workspace|human:jane@example.com" {
		t.Fatalf("seeded with the owner: %v", maker.seeded)
	}
	var resp AuthResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("signed in")
	}

	// Email and password alone: the workspace is named after the address.
	rec = postRegister(a, "198.51.100.33:1",
		`{"email":"Sam.Lee@example.com","password":"longenough1"}`)
	if rec.Code != http.StatusOK || maker.made[1] != "sam-lee's workspace|sam-lee" {
		t.Fatalf("from the address alone: %d %v", rec.Code, maker.made)
	}

	// A name for the workspace is taken when given (the enterprise form).
	rec = postRegister(a, "198.51.100.31:1",
		`{"email":"ops@acme.example","username":"ops","password":"longenough1","name":"Ops","workspace_name":"Acme Corp"}`)
	if rec.Code != http.StatusOK || maker.made[2] != "Acme Corp|Acme Corp" || fake.workspaces[2] != "ws-Acme Corp" {
		t.Fatalf("a named workspace: %d %v %v", rec.Code, maker.made, fake.workspaces)
	}
}

// A first sign-in through a provider gets the same: its own workspace,
// seeded; a known account gets none.
func TestAProviderSignUpGetsItsOwnWorkspace(t *testing.T) {
	p := &fakeProvider{id: socialIdentity{Email: "dev@example.com", Name: "Dev One", Verified: true}}
	a, fake := socialAdapter(t, p)
	maker := &makerFake{}
	a.SetWorkspaces(maker)
	state, _ := a.social.begin("")
	back := get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c&state="+state, "198.51.100.40:1")
	if back.Code != http.StatusFound || !strings.Contains(back.Header().Get("Location"), "#token=") {
		t.Fatalf("callback: %d %s", back.Code, back.Header().Get("Location"))
	}
	if len(maker.made) != 1 || maker.made[0] != "Dev One's workspace|dev" || len(maker.seeded) != 1 || !strings.HasPrefix(maker.seeded[0], "ws-dev|Dev One's workspace|human:dev@example.com") {
		t.Fatalf("made %v seeded %v", maker.made, maker.seeded)
	}
	fake.known = map[string]bool{"dev@example.com": true}
	state, _ = a.social.begin("")
	_ = get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c&state="+state, "198.51.100.40:1")
	if len(maker.made) != 1 || len(maker.seeded) != 1 {
		t.Fatalf("a known account gets no new workspace: made %v seeded %v", maker.made, maker.seeded)
	}
}

// A sign-up the service refuses (the address already registered) leaves no
// workspace behind; the one that then succeeds keeps the plain slug.
func TestAFailedSignUpLeavesNoWorkspace(t *testing.T) {
	fake := &registerFake{refuse: "email already registered"}
	maker := &makerFake{}
	a := NewAuthGatewayAdapter(fake)
	a.SetWorkspaces(maker)
	rec := postRegister(a, "198.51.100.32:1",
		`{"email":"jane@example.com","username":"jane","password":"longenough1","name":"Jane Doe"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("refused sign-up: %d %s", rec.Code, rec.Body.String())
	}
	if len(maker.made) != 1 || len(maker.dropped) != 1 || maker.dropped[0] != "ws-jane" || len(maker.seeded) != 0 {
		t.Fatalf("made %v dropped %v seeded %v", maker.made, maker.dropped, maker.seeded)
	}
}

func TestPersonalWorkspaceName(t *testing.T) {
	cases := [][3]string{
		{"Jane Doe", "jane", "Jane Doe's workspace"},
		{"", "jane", "jane's workspace"},
		{"Chris", "c", "Chris' workspace"},
		{"", "", "My workspace"},
	}
	for _, c := range cases {
		if got := personalWorkspaceName(c[0], c[1]); got != c[2] {
			t.Errorf("%q/%q: got %q want %q", c[0], c[1], got, c[2])
		}
	}
}

// The slug is the id, made unique with a number when taken.
func TestNewWorkspaceRowMakesTheSlugUnique(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT UNIQUE NOT NULL, owner_id TEXT, language TEXT, plan TEXT, status TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := newWorkspaceRow(ctx, db, "Jane Doe's workspace", "jane")
	second, err2 := newWorkspaceRow(ctx, db, "Another Jane", "jane")
	third, err3 := newWorkspaceRow(ctx, db, "Acme Corp", "Acme Corp")
	if err != nil || err2 != nil || err3 != nil {
		t.Fatal(err, err2, err3)
	}
	if first != "jane" || second != "jane-2" || third != "acme-corp" {
		t.Fatalf("slugs: %q %q %q", first, second, third)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM workspaces WHERE id = 'jane-2'`).Scan(&name); err != nil || name != "Another Jane" {
		t.Fatalf("the row keeps the name: %q %v", name, err)
	}
}
