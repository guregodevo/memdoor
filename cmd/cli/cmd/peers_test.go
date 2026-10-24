package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/gateway"
)

// A peer is a gateway this person is signed in to: without a session for
// it, add refuses and says how; with one, the peer is kept with its agents.
func TestPeerAddNeedsASessionAndKeepsThePeer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.651.0"})
		case "/api/auth/whoami":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "human:1"})
		case "/api/agents":
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{{"name": "coder"}, {"name": "chief"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if err := peerAddCmd.RunE(peerAddCmd, []string{"box", srv.URL}); err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Fatalf("no session: %v", err)
	}
	if err := saveCredentialsFor(normalizeGateway(srv.URL), &credentials{Token: "tok", Email: "me@example.test", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := peerAddCmd.Flags().Set("dir", "/srv/app"); err != nil {
		t.Fatal(err)
	}
	if err := peerAddCmd.RunE(peerAddCmd, []string{"Box", srv.URL}); err != nil {
		t.Fatalf("add: %v", err)
	}
	peers, err := gateway.LoadPeers()
	if err != nil || peers["box"].Gateway != srv.URL || peers["box"].Dir != "/srv/app" {
		t.Fatalf("kept: %+v %v", peers, err)
	}
	c, err := peerClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	agents, version, err := peerProbe(c)
	if err != nil || version != "v1.651.0" || strings.Join(agents, ",") != "chief,coder" {
		t.Fatalf("probe: %v %s %v", agents, version, err)
	}
	if err := peerRemoveCmd.RunE(peerRemoveCmd, []string{"box"}); err != nil {
		t.Fatal(err)
	}
	if peers, _ := gateway.LoadPeers(); len(peers) != 0 {
		t.Fatalf("removed: %+v", peers)
	}
	if err := peerAddCmd.RunE(peerAddCmd, []string{"bad:name", srv.URL}); err == nil {
		t.Fatal("a name with a colon is not a peer's")
	}
}

// A question asked on the peer has nobody there to answer: the first option
// is taken and the answer says so.
func TestAPeerTurnsQuestionTakesTheDefault(t *testing.T) {
	s := &peerSink{}
	if got := s.Ask("a.go or b.go?", []string{"a.go", "b.go"}, ""); got != "a.go" || s.asked != 1 {
		t.Fatalf("default taken: %q %d", got, s.asked)
	}
	var streamed []string
	s = &peerSink{onText: func(t string) { streamed = append(streamed, t) }}
	s.Text("po")
	s.Text("ng")
	if s.text.String() != "pong" || strings.Join(streamed, "") != "pong" {
		t.Fatalf("text kept and streamed: %q %v", s.text.String(), streamed)
	}
	_ = context.Background()
}
