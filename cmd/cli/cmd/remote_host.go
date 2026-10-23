package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway"
)

// THE GATEWAY'S OWN PROCESS HOSTS THE PHONE'S TERMINAL (2026-10-23,
// gateway/remote_host.go). `memdoor gateway` registers this host before the
// server starts; a window that turns /remote on hands the gateway its link
// (hostRemoteOnGateway), and from then on the terminal end of the relay, the
// second TUI program on the conversation, runs here, in the daemon, exactly
// as the window used to run it: the same wiring (tui_wiring.go), the same
// poster, the same relay client (tui_remote.go). The window may close; the
// link stays open; a restart revives it.

// gatewayRemoteHost hosts links for the gateway in this process.
type gatewayRemoteHost struct {
	base string // this gateway, as this process dials it
}

func newGatewayRemoteHost(host string, port int) *gatewayRemoteHost {
	base := fmt.Sprintf("http://%s:%d", gateway.DialHost(host), port)
	// The relay session is built as a window builds one, and a window reads
	// the gateway's address from the global: in the gateway's own process
	// that address is itself.
	gatewayAddr = base
	remoteHostedHere = true
	return &gatewayRemoteHost{base: base}
}

// remoteHostedHere is set in the gateway's process: /remote typed into the
// phone's own terminal there has nothing to turn on.
var remoteHostedHere bool

func (h *gatewayRemoteHost) Start(spec gateway.RemoteHostSpec) error {
	c := &Client{BaseURL: h.base, HTTPClient: &http.Client{Timeout: 180 * time.Second}, token: spec.Token, workspaceSlug: spec.Workspace}
	dir := spec.Dir
	curDir := func() string { return dir }
	w := newTUIWiring(c, spec.Workspace, wsURLOf(h.base), curDir, tuiPoster(c, spec.Workspace, curDir))
	s := w.remoteFor(ui.FirstAgent(), spec.ChannelID, spec.ChannelName)
	// Settled by the window that turned the link on: where it lives and
	// what the terminal end dials it with.
	s.Relay, s.RelayToken = spec.Relay, spec.RelayToken
	startRemote(s, remoteLink{ChannelID: spec.ChannelID, ID: spec.ID, Key: spec.Key, Created: time.Now()})
	return nil
}

func (h *gatewayRemoteHost) Stop(channelID string) { stopRemote(channelID) }

// hostedLinkReply is GET /api/remote/<channel>.
type hostedLinkReply struct {
	Hosted bool   `json:"hosted"`
	ID     string `json:"id"`
	Key    string `json:"key"`
	Relay  string `json:"relay"`
}

// remoteAPI is the window's client for /api/remote on the conversation's
// gateway, with the session the window holds.
func (s remoteSession) remoteAPI(method string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(s.Gateway, "/")+"/api/remote/"+s.ChannelID, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	if s.Workspace != "" {
		req.Header.Set("X-Forwarded-Workspace", s.Workspace)
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return (&http.Client{Timeout: 10 * time.Second}).Do(req)
}

// hostedLink is the link the gateway already hosts for this conversation,
// from any window on any machine: the same link everywhere. It is kept in
// this machine's links file too, so `memdoor remote` prints it.
func (s remoteSession) hostedLink() (remoteLink, bool) {
	resp, err := s.remoteAPI(http.MethodGet, nil)
	if err != nil {
		return remoteLink{}, false
	}
	defer resp.Body.Close()
	var reply hostedLinkReply
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&reply) != nil || !reply.Hosted {
		return remoteLink{}, false
	}
	l := remoteLink{ChannelID: s.ChannelID, ID: reply.ID, Key: reply.Key, Created: time.Now()}
	links := loadRemoteLinks()
	if had, ok := links[s.ChannelID]; !ok || had.ID != l.ID || had.Key != l.Key {
		links[s.ChannelID] = l
		_ = saveRemoteLinks(links)
	}
	return l, true
}

// hostRemoteOnGateway hands the link to the gateway. True when the gateway
// hosts it from now on; false with no error when it cannot (an older gateway
// with no such door, or none reached), and the window hosts the link itself
// as before; an error when the gateway refused, in its words (another user
// holds this conversation's link): the person is told, nothing is hosted.
func hostRemoteOnGateway(s remoteSession, l remoteLink) (bool, error) {
	if remoteHostedHere || strings.TrimSpace(s.Gateway) == "" || s.Token == "" {
		return false, nil
	}
	dir := ""
	if s.Dir != nil {
		dir = s.Dir()
	}
	spec := gateway.RemoteHostSpec{
		Workspace: s.Workspace, ChannelID: s.ChannelID, ChannelName: s.ChannelName, Dir: dir,
		ID: l.ID, Key: l.Key, Token: s.Token, Relay: s.relayBase(), RelayToken: s.relayToken(),
	}
	resp, err := s.remoteAPI(http.MethodPut, spec)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	var reply struct {
		Error string `json:"error"`
	}
	why := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &reply) == nil && reply.Error != "" {
		why = reply.Error
	}
	return false, fmt.Errorf("the gateway did not take the link: %s", why)
}

// unhostRemoteOnGateway revokes the link on the gateway (/remote off).
func unhostRemoteOnGateway(s remoteSession) {
	if remoteHostedHere || strings.TrimSpace(s.Gateway) == "" || s.Token == "" {
		return
	}
	if resp, err := s.remoteAPI(http.MethodDelete, nil); err == nil {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
	}
}
