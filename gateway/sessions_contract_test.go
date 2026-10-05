package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"

	"memdoor/gateway/broadcast"
)

// The CLI decodes GET /sessions into its own struct. When the two drift the
// failure is SILENT: encoding/json leaves unmatched fields at their zero value,
// so `memdoor sessions list` printed a full table of "<nil>" and empty columns
// for however long the names had disagreed, with nothing failing anywhere.
//
// This pins the field names the CLI reads. Renaming one here without changing
// the CLI (or vice versa) fails the build's tests instead of quietly emptying
// a command.
func TestSessionsPayloadMatchesCLIContract(t *testing.T) {
	s := &Server{
		port:        8080,
		sessions:    NewSessionManager(),
		clients:     make(map[string]*Client),
		upgrader:    websocket.Upgrader{},
		broadcaster: broadcast.NewSubscriptionManager(false),
	}
	if _, err := s.sessions.GetOrCreateSession("workspace:acme:channel:8488c86b-5f13-48de-a4e9-233c11d561e9", "main"); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	rec := httptest.NewRecorder()
	s.handleSessions(rec, httptest.NewRequest("GET", "/sessions", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /sessions = %d, want 200", rec.Code)
	}

	// Decode exactly as cmd/cli/cmd/sessions.go does.
	var cli struct {
		Count    int `json:"count"`
		Sessions []struct {
			SessionKey string `json:"session_key"`
			Kind       string `json:"kind"`
			CreatedAt  int64  `json:"created_at"`
			UpdatedAt  int64  `json:"updated_at"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cli); err != nil {
		t.Fatalf("CLI cannot decode the payload: %v\n%s", err, rec.Body.String())
	}

	if cli.Count != 1 || len(cli.Sessions) != 1 {
		t.Fatalf("count=%d sessions=%d, want 1 and 1", cli.Count, len(cli.Sessions))
	}
	got := cli.Sessions[0]
	// Every column the command prints must actually arrive. Zero values here
	// are exactly the bug: they render as blanks, not as an error.
	if got.SessionKey == "" {
		t.Error("session_key empty — the CLI would print <nil> for the key")
	}
	if got.Kind == "" {
		t.Error("kind empty — the CLI would print a blank column")
	}
	if got.CreatedAt == 0 {
		t.Error("created_at zero — the CLI would print no creation time")
	}
	if got.UpdatedAt == 0 {
		t.Error("updated_at zero — the CLI would print no last-used time")
	}
}
