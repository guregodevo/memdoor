package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newShareTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := newShareStore(t.TempDir(), relayUsers{"tok-a": "alice", "tok-b": "bob"})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/share", s.Handle)
	mux.HandleFunc("/api/share/", s.Handle)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func shareReq(t *testing.T, method, url, token string, body []byte) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// A share is created by an account, read by anyone with its id (the key is
// in the link, not here), and deleted only by the account that made it.
func TestShareStoreLifecycle(t *testing.T) {
	srv := newShareTestServer(t)
	sealed := []byte("\x00\x01ciphertext the server cannot read")

	if code, _ := shareReq(t, "POST", srv.URL+"/api/share", "", sealed); code != http.StatusUnauthorized {
		t.Fatalf("no account, no share: %d", code)
	}
	code, body := shareReq(t, "POST", srv.URL+"/api/share", "tok-a", sealed)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}
	var created struct{ ID string }
	if json.Unmarshal(body, &created) != nil || !validRelayKey(created.ID) {
		t.Fatalf("create returned %s", body)
	}
	url := srv.URL + "/api/share/" + created.ID

	if code, got := shareReq(t, "GET", url, "", nil); code != http.StatusOK || !bytes.Equal(got, sealed) {
		t.Fatalf("anyone reads the sealed bytes back: %d %q", code, got)
	}
	if code, _ := shareReq(t, "DELETE", url, "tok-b", nil); code != http.StatusForbidden {
		t.Fatalf("another account cannot delete it: %d", code)
	}
	if code, _ := shareReq(t, "DELETE", url, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no account cannot delete it: %d", code)
	}
	if code, _ := shareReq(t, "DELETE", url, "tok-a", nil); code != http.StatusNoContent {
		t.Fatalf("its owner deletes it: %d", code)
	}
	if code, _ := shareReq(t, "GET", url, "", nil); code != http.StatusNotFound {
		t.Fatalf("a deleted share is gone: %d", code)
	}
	if code, _ := shareReq(t, "DELETE", url, "tok-a", nil); code != http.StatusNoContent {
		t.Fatalf("deleting twice is not an error: %d", code)
	}
}

func TestShareStoreLimits(t *testing.T) {
	srv := newShareTestServer(t)
	if code, _ := shareReq(t, "POST", srv.URL+"/api/share", "tok-a", make([]byte, shareMaxBytes+1)); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over 1 MiB is refused: %d", code)
	}
	if code, _ := shareReq(t, "POST", srv.URL+"/api/share", "tok-a", nil); code != http.StatusBadRequest {
		t.Fatalf("an empty share is refused: %d", code)
	}
	for _, bad := range []string{"../../etc/passwd", "x", "AAAAAAAAAAAAAAA!"} {
		if code, _ := shareReq(t, "GET", srv.URL+"/api/share/"+bad, "", nil); code != http.StatusNotFound {
			t.Errorf("%q must be not found: %d", bad, code)
		}
	}
}
