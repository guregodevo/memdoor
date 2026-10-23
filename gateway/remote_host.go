package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"memdoor/pkg/shared"
)

// THE GATEWAY HOLDS THE PHONE'S LINK (Greg, 2026-10-23, reviewing the
// commands: "the daemon owns the work"). Until now the phone's terminal, the
// second TUI program on a conversation, ran inside the window or inside
// the old `memdoor remote`, so a link lived exactly as long as that process: closing
// the window on a box closed the phone's door. Now a window hands its link
// to the gateway (PUT /api/remote/<channel>) and the gateway's own process
// hosts that terminal, in the binary's cmd package (cmd/cli/cmd/remote_host.go)
// behind the RemoteHost interface declared here, as the domain's callbacks
// are. The link outlives the window, a gate is answered from the phone with
// nothing else open, and a restart revives every link from disk.
//
// What is kept, on this machine and nowhere else: the link (id and key: the
// key seals every frame, and the gateway hosting the terminal is the one
// end that must have it), the session token of the person who turned the
// link on (the host follows and posts as them), where the link lives (this
// gateway, or memdoor.ai with the account token for a gateway kept to
// localhost). ~/.memdoor/remote-hosted.json, mode 0600, beside the sessions
// the gateway already keeps.

// RemoteHostSpec is one hosted link: what the host needs to be the terminal
// end of the relay for one conversation.
type RemoteHostSpec struct {
	Workspace   string `json:"workspace"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Dir         string `json:"dir"`
	ID          string `json:"id"`
	Key         string `json:"key"`
	Token       string `json:"token"`
	Relay       string `json:"relay"`
	RelayToken  string `json:"relay_token,omitempty"`
	// Actor is the gateway user who turned the link on: the only one who
	// may read, replace or revoke it. Set by the handler, never by the body.
	Actor string `json:"actor"`
}

// RemoteHost is the process-side of hosting: start being the terminal end
// of a link, stop being it. The binary provides it (SetRemoteHost) before
// Start; a gateway with none answers 404, and the window hosts as before.
type RemoteHost interface {
	Start(spec RemoteHostSpec) error
	Stop(channelID string)
}

var remoteHostMu sync.Mutex
var remoteHost RemoteHost

// SetRemoteHost names the host the gateway hands links to.
func SetRemoteHost(h RemoteHost) {
	remoteHostMu.Lock()
	defer remoteHostMu.Unlock()
	remoteHost = h
}

func currentRemoteHost() RemoteHost {
	remoteHostMu.Lock()
	defer remoteHostMu.Unlock()
	return remoteHost
}

// remoteHosted is the hosted links, on disk and in memory.
type remoteHosted struct {
	mu    sync.Mutex
	path  string
	specs map[string]RemoteHostSpec // by channel id
}

const remoteHostedFile = "remote-hosted.json"

func newRemoteHosted(path string) *remoteHosted {
	h := &remoteHosted{path: path, specs: map[string]RemoteHostSpec{}}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &h.specs)
	}
	return h
}

func (h *remoteHosted) save() error {
	b, err := json.MarshalIndent(h.specs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(h.path, b, 0o600)
}

// revive hands every link kept on disk to the host: a restarted gateway
// hosts what it hosted. Nothing to do without a host.
func (h *remoteHosted) revive() {
	host := currentRemoteHost()
	if host == nil {
		return
	}
	h.mu.Lock()
	specs := make([]RemoteHostSpec, 0, len(h.specs))
	for _, s := range h.specs {
		specs = append(specs, s)
	}
	h.mu.Unlock()
	for _, s := range specs {
		_ = host.Start(s)
	}
}

// handle is /api/remote/<channel>, for the authenticated user:
//
//	GET     the link this gateway hosts for the conversation, if yours: {hosted, id, key, relay}
//	PUT     host this link (body: the spec; a repeat with the same link is a no-op)
//	DELETE  stop hosting it: the pages on the link are told, the link is forgotten
//
// 404 without a host (an older binary, or a gateway that is not this
// process), so a window falls back to hosting the link itself.
func (h *remoteHosted) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actor, ok := requireAuth(w, r)
	if !ok {
		return
	}
	host := currentRemoteHost()
	if host == nil {
		http.Error(w, `{"error":"this gateway does not host links"}`, http.StatusNotFound)
		return
	}
	channel := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/remote"), "/")
	if channel == "" || strings.Contains(channel, "/") {
		http.Error(w, `{"error":"/api/remote/<channel>"}`, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.mu.Lock()
		spec, found := h.specs[channel]
		h.mu.Unlock()
		if !found || spec.Actor != string(actor) {
			_ = json.NewEncoder(w).Encode(map[string]any{"hosted": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"hosted": true, "id": spec.ID, "key": spec.Key, "relay": spec.Relay})
	case http.MethodPut:
		var spec RemoteHostSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, `{"error":"a link: workspace, channel_id, id, key, token, relay"}`, http.StatusBadRequest)
			return
		}
		spec.ChannelID, spec.Actor = channel, string(actor)
		if err := spec.valid(); err != nil {
			http.Error(w, `{"error":"`+jsonSafe(err.Error())+`"}`, http.StatusBadRequest)
			return
		}
		h.mu.Lock()
		if had, found := h.specs[channel]; found && had.Actor != spec.Actor {
			h.mu.Unlock()
			http.Error(w, `{"error":"another user holds this conversation's link"}`, http.StatusForbidden)
			return
		}
		h.specs[channel] = spec
		err := h.save()
		h.mu.Unlock()
		if err != nil {
			http.Error(w, `{"error":"could not keep the link: `+jsonSafe(err.Error())+`"}`, http.StatusInternalServerError)
			return
		}
		if err := host.Start(spec); err != nil {
			http.Error(w, `{"error":"`+jsonSafe(err.Error())+`"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"hosted": true, "id": spec.ID, "relay": spec.Relay})
	case http.MethodDelete:
		h.mu.Lock()
		spec, found := h.specs[channel]
		if found && spec.Actor == string(actor) {
			delete(h.specs, channel)
			_ = h.save()
		}
		h.mu.Unlock()
		if !found || spec.Actor != string(actor) {
			_ = json.NewEncoder(w).Encode(map[string]any{"hosted": false})
			return
		}
		host.Stop(channel)
		_ = json.NewEncoder(w).Encode(map[string]any{"hosted": false, "was": true})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s RemoteHostSpec) valid() error {
	switch {
	case strings.TrimSpace(s.Workspace) == "":
		return errors.New("workspace is required")
	case strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Key) == "":
		return errors.New("the link's id and key are required")
	case strings.TrimSpace(s.Token) == "":
		return errors.New("the session token is required: the host follows and posts as you")
	case !strings.HasPrefix(s.Relay, "http://") && !strings.HasPrefix(s.Relay, "https://"):
		return errors.New("relay must be where the link lives, an http(s) URL")
	}
	return nil
}

// remoteHostedPath is where the hosted links are kept.
func remoteHostedPath() string {
	return shared.MemdoorHome(remoteHostedFile)
}
