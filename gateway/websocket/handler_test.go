package websocket

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubValidator struct{ userID string }

func (s stubValidator) ValidateToken(token string) (string, error) {
	if token == "good" {
		return s.userID, nil
	}
	return "", errors.New("bad token")
}

// The former "legacy" branch accepted ?user=<id> with no token when no
// validator was wired. There is no such branch any more: a connection
// without a token is rejected whatever the query string says.
func TestHandleWebSocketWithAuth_RejectsQueryUserWithoutToken(t *testing.T) {
	h := HandleWebSocketWithAuth(NewHub(), stubValidator{userID: "u1"}, nil)

	for _, target := range []string{"/chat/ws", "/chat/ws?user=admin", "/chat/ws?user=admin&channel=general"} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", target, rec.Code)
		}
	}
}

func TestHandleWebSocketWithAuth_RejectsInvalidToken(t *testing.T) {
	h := HandleWebSocketWithAuth(NewHub(), stubValidator{userID: "u1"}, nil)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/chat/ws?token=forged&user=admin", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("forged token: got %d, want 401", rec.Code)
	}
}

func TestHandleWebSocketWithAuth_RequiresValidator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil validator must panic at wiring time, not silently accept anonymous clients")
		}
	}()
	HandleWebSocketWithAuth(NewHub(), nil, nil)
}

// The upgrader used to accept ANY Origin (CheckOrigin: return true), so any site
// could open an authenticated socket (CSWSH). A cross-origin browser request
// (Origin host != request Host) must now be rejected before the upgrade — it
// fails the handshake with 403 rather than reaching the token check.
func TestHandleWebSocketWithAuth_RejectsCrossOrigin(t *testing.T) {
	h := HandleWebSocketWithAuth(NewHub(), stubValidator{userID: "u1"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/chat/ws?token=good", nil)
	req.Host = "memdoor.ai"
	req.Header.Set("Origin", "http://evil.example.com")
	// Make it look like a real WS upgrade so CheckOrigin is consulted.
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin upgrade: got %d, want 403", rec.Code)
	}

	// A same-origin request passes the origin gate (then proceeds to upgrade).
	req2 := httptest.NewRequest(http.MethodGet, "/chat/ws?token=good", nil)
	req2.Host = "memdoor.ai"
	req2.Header.Set("Origin", "https://memdoor.ai")
	req2.Header.Set("Connection", "Upgrade")
	req2.Header.Set("Upgrade", "websocket")
	req2.Header.Set("Sec-WebSocket-Version", "13")
	req2.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec2 := httptest.NewRecorder()
	h(rec2, req2)
	if rec2.Code == http.StatusForbidden {
		t.Error("same-origin upgrade must not be blocked by the origin gate")
	}
}
