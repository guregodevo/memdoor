package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A host that records what it was told.
type recordingHost struct {
	mu      sync.Mutex
	started []RemoteHostSpec
	stopped []string
}

func (h *recordingHost) Start(spec RemoteHostSpec) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.started = append(h.started, spec)
	return nil
}

func (h *recordingHost) Stop(channelID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = append(h.stopped, channelID)
}

func hostedCall(t *testing.T, h *remoteHosted, method, channel, actor, body string) (int, map[string]any) {
	t.Helper()
	req := authenticatedRequest(method, "/api/remote/"+channel, strings.NewReader(body), actor)
	rec := httptest.NewRecorder()
	h.handle(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

const hostedSpec = `{"workspace":"w","channel_name":"c","dir":"/srv/proj","id":"abc","key":"k1","token":"tok","relay":"http://box:18789"}`

// THE GATEWAY HOLDS THE LINK: a window hands it over, the host is started,
// the link is on disk (0600) for the next start, only its owner sees it,
// and revoking it stops the host and forgets it.
func TestTheGatewayHostsALinkForItsOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote-hosted.json")
	host := &recordingHost{}
	SetRemoteHost(host)
	t.Cleanup(func() { SetRemoteHost(nil) })
	h := newRemoteHosted(path)

	code, out := hostedCall(t, h, http.MethodGet, "ch1", "user:a", "")
	if code != 200 || out["hosted"] != false {
		t.Fatalf("nothing hosted yet: %d %v", code, out)
	}
	code, out = hostedCall(t, h, http.MethodPut, "ch1", "user:a", hostedSpec)
	if code != 200 || out["hosted"] != true || out["id"] != "abc" {
		t.Fatalf("put: %d %v", code, out)
	}
	if len(host.started) != 1 || host.started[0].ChannelID != "ch1" || host.started[0].Actor != "user:a" || host.started[0].Dir != "/srv/proj" {
		t.Fatalf("the host was started with the spec, channel and actor from the request: %+v", host.started)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("the link is kept on disk for the owner alone: %v %v", err, fi)
	}
	if _, out = hostedCall(t, h, http.MethodGet, "ch1", "user:a", ""); out["hosted"] != true || out["key"] != "k1" || out["relay"] != "http://box:18789" {
		t.Fatalf("the owner reads the link back: %v", out)
	}
	if _, out = hostedCall(t, h, http.MethodGet, "ch1", "user:b", ""); out["hosted"] != false {
		t.Fatalf("another user sees no link: %v", out)
	}
	if code, _ = hostedCall(t, h, http.MethodPut, "ch1", "user:b", hostedSpec); code != http.StatusForbidden {
		t.Fatalf("another user cannot take the link over: %d", code)
	}
	if _, out = hostedCall(t, h, http.MethodDelete, "ch1", "user:b", ""); out["was"] == true {
		t.Fatalf("another user cannot revoke it: %v", out)
	}
	if len(host.stopped) != 0 {
		t.Fatalf("nothing stopped yet: %v", host.stopped)
	}

	// A restart: the file alone, revived into a fresh host.
	again := &recordingHost{}
	SetRemoteHost(again)
	newRemoteHosted(path).revive()
	if len(again.started) != 1 || again.started[0].Key != "k1" {
		t.Fatalf("a restarted gateway hosts what it hosted: %+v", again.started)
	}

	SetRemoteHost(host)
	if _, out = hostedCall(t, h, http.MethodDelete, "ch1", "user:a", ""); out["was"] != true {
		t.Fatalf("the owner revokes: %v", out)
	}
	if len(host.stopped) != 1 || host.stopped[0] != "ch1" {
		t.Fatalf("the host was stopped: %v", host.stopped)
	}
	if _, out = hostedCall(t, h, http.MethodGet, "ch1", "user:a", ""); out["hosted"] != false {
		t.Fatalf("forgotten after revoke: %v", out)
	}
	if specs := newRemoteHosted(path).specs; len(specs) != 0 {
		t.Fatalf("forgotten on disk too: %v", specs)
	}
}

// Without a host (a gateway that is not this binary's own process) the
// endpoint says so with 404, and a window hosts the link itself as before.
func TestNoHostMeansTheWindowHosts(t *testing.T) {
	SetRemoteHost(nil)
	h := newRemoteHosted(filepath.Join(t.TempDir(), "remote-hosted.json"))
	if code, _ := hostedCall(t, h, http.MethodPut, "ch1", "user:a", hostedSpec); code != http.StatusNotFound {
		t.Fatalf("no host: 404, got %d", code)
	}
	SetRemoteHost(&recordingHost{})
	t.Cleanup(func() { SetRemoteHost(nil) })
	if code, _ := hostedCall(t, h, http.MethodPut, "ch1", "user:a", `{"workspace":"w","id":"abc","key":"k1","token":"tok","relay":"box:18789"}`); code != http.StatusBadRequest {
		t.Fatalf("a relay that is not a URL is refused: %d", code)
	}
	if code, _ := hostedCall(t, h, http.MethodPut, "ch1", "user:a", `{"workspace":"w","id":"abc","key":"k1","relay":"http://box:18789"}`); code != http.StatusBadRequest {
		t.Fatalf("no token, no host: %d", code)
	}
}
