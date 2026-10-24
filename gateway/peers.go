package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"memdoor/gateway/infra"
	"memdoor/pkg/shared"
)

// PEERS, EXPERIMENTAL (Greg, 2026-10-24: "we need A2A from one gateway to
// another", "we can do @<host>:agent_name", "we have a quorum of gateways;
// from the quorum we can see which gateway are available"). A peer is a
// gateway this person has signed in to and named: ~/.memdoor/peers.json,
// `memdoor peers add box http://box:18789 --dir /srv/app`. From then on
// @box:coder in any conversation here is a mention of box's coder: the
// turn runs there, in that directory, as `memdoor --gateway box run` would
// run it, and the answer comes back as the thread's reply from box:coder
// (pkg/message/peers.go). `memdoor peers` is the quorum: each peer, whether
// it answers, which agents it has. The call itself is the binary's
// (cmd/cli/cmd/peers.go, SetPeerCaller): the same door and the same sign-in
// a person uses, nothing new on the wire.

// PeerEntry is one named gateway.
type PeerEntry struct {
	Gateway string `json:"gateway"`
	Dir     string `json:"dir,omitempty"` // the project there a turn runs in
}

const peersFile = "peers.json"

// PeersPath is where the peers are kept.
func PeersPath() string { return shared.MemdoorHome(peersFile) }

// LoadPeers reads the peers by name; none is not an error.
func LoadPeers() (map[string]PeerEntry, error) {
	peers := map[string]PeerEntry{}
	b, err := os.ReadFile(PeersPath())
	if errors.Is(err, os.ErrNotExist) {
		return peers, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &peers); err != nil {
		return nil, fmt.Errorf("%s: %w", PeersPath(), err)
	}
	return peers, nil
}

// SavePeers writes the peers, the person's alone.
func SavePeers(peers map[string]PeerEntry) error {
	b, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(PeersPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(PeersPath(), b, 0o600)
}

// PeerCaller runs one turn of an agent on a peer gateway, in dir there,
// hands each piece of its text to onText as it arrives, and returns the
// whole answer. The binary provides it (SetPeerCaller) before Start; a
// gateway with none cannot ask a peer.
type PeerCaller func(ctx context.Context, gateway, agent, dir, text string, onText func(string)) (string, error)

var peerCallerMu sync.Mutex
var peerCaller PeerCaller

// SetPeerCaller names what asks a peer.
func SetPeerCaller(f PeerCaller) {
	peerCallerMu.Lock()
	defer peerCallerMu.Unlock()
	peerCaller = f
}

// peerSet is the message service's view of the peers (message.PeerGateways).
// A peer's turn is streamed to the asking conversation as the events a
// local turn emits (lifecycle start, the text, lifecycle complete), so a
// window or a run on it shows the answer as it comes and ends when it ends.
type peerSet struct {
	events func() *infra.EventEmitter
}

func newPeerSet(events func() *infra.EventEmitter) peerSet { return peerSet{events: events} }

func (p peerSet) emit(runID string, stream infra.AgentEventStream, sessionKey string, data map[string]interface{}) {
	if p.events == nil || sessionKey == "" {
		return
	}
	if ee := p.events(); ee != nil {
		ee.EmitEvent(runID, stream, sessionKey, data)
	}
}

func (p peerSet) IsPeer(peer string) bool {
	peers, err := LoadPeers()
	if err != nil {
		return false
	}
	_, ok := peers[strings.ToLower(strings.TrimSpace(peer))]
	return ok
}

func (p peerSet) Ask(ctx context.Context, peer, agent, text, sessionKey, runID string) (string, error) {
	peers, err := LoadPeers()
	if err != nil {
		return "", err
	}
	entry, ok := peers[strings.ToLower(strings.TrimSpace(peer))]
	if !ok {
		return "", fmt.Errorf("%s is not a peer of this gateway (memdoor peers)", peer)
	}
	peerCallerMu.Lock()
	call := peerCaller
	peerCallerMu.Unlock()
	if call == nil {
		return "", errors.New("this gateway cannot ask a peer")
	}
	p.emit(runID, infra.EventStreamLifecycle, sessionKey, map[string]interface{}{"event": "start", "message": text})
	started := false
	onText := func(s string) {
		if s == "" {
			return
		}
		if !started {
			started = true
			p.emit(runID, infra.EventStreamAssistant, sessionKey, map[string]interface{}{"event": "text_start"})
		}
		p.emit(runID, infra.EventStreamAssistant, sessionKey, map[string]interface{}{"event": "text_delta", "delta": s})
	}
	answer, err := call(ctx, entry.Gateway, agent, entry.Dir, text, onText)
	if err != nil {
		p.emit(runID, infra.EventStreamLifecycle, sessionKey, map[string]interface{}{"event": "error", "error": err.Error()})
		return answer, err
	}
	p.emit(runID, infra.EventStreamAssistant, sessionKey, map[string]interface{}{"event": "text_end", "text": answer})
	p.emit(runID, infra.EventStreamLifecycle, sessionKey, map[string]interface{}{"event": "complete", "success": true, "tools_executed": 0, "model": agent + "@" + peer})
	return answer, nil
}
