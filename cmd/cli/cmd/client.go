package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memdoor/pkg/auth"
	"memdoor/pkg/shared"
)

// Client is a reusable HTTP client for the Memdoor gateway API
type Client struct {
	BaseURL       string
	HTTPClient    *http.Client
	token         string
	workspaceSlug string
}

// NewClient creates a client using the global gateway address and
// loads saved credentials. The global -w/--workspace flag (validated
// in root.go's PersistentPreRunE) is copied into the client so every
// outbound request carries an X-Forwarded-Workspace header — that's
// how the gateway scopes channel/message/agent queries to the
// caller's workspace.
func NewClient() *Client {
	c := &Client{
		BaseURL:       gatewayAddr,
		HTTPClient:    &http.Client{Timeout: 180 * time.Second},
		workspaceSlug: workspaceSlug,
	}
	if creds, err := loadCredentials(); err == nil && creds.Token != "" {
		c.token = creds.Token
	}
	return c
}

// Do executes a raw HTTP request with auth headers
func (c *Client) Do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.workspaceSlug != "" {
		req.Header.Set("X-Forwarded-Workspace", c.workspaceSlug)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway unreachable at %s: %w\n  Start gateway with: memdoor gateway --verbose", c.BaseURL, err)
	}
	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(path string) (*http.Response, error) {
	return c.Do("GET", path, nil)
}

// Post performs a POST request with JSON body
func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
	return c.Do("POST", path, body)
}

// Put performs a PUT request with JSON body
func (c *Client) Put(path string, body interface{}) (*http.Response, error) {
	return c.Do("PUT", path, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(path string) (*http.Response, error) {
	return c.Do("DELETE", path, nil)
}

// GetJSON performs a GET and decodes the JSON response into v
func (c *Client) GetJSON(path string, v interface{}) error {
	resp, err := c.Get(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// DeleteJSON performs a DELETE and decodes the JSON response into v.
// Used for cascade-deletes (e.g. `memdoor workspace delete`) where
// the server returns a structured summary of what was removed.
func (c *Client) DeleteJSON(path string, v interface{}) error {
	resp, err := c.Do("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// PostJSON performs a POST and decodes the JSON response into v
func (c *Client) PostJSON(path string, body interface{}, v interface{}) error {
	resp, err := c.Post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// PostExpectOK performs a POST and checks for 2xx status
func (c *Client) PostExpectOK(path string, body interface{}) error {
	resp, err := c.Post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// PutExpectOK performs a PUT and checks for 2xx status
func (c *Client) PutExpectOK(path string, body interface{}) error {
	resp, err := c.Put(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// DeleteExpectOK performs a DELETE and checks for 2xx status
func (c *Client) DeleteExpectOK(path string) error {
	resp, err := c.Delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// GetText performs a GET and returns the response body as text
func (c *Client) GetText(path string) (string, error) {
	resp, err := c.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return "", err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// resolveChannelID looks up a channel by name and returns its UUID.
func (c *Client) resolveChannelID(nameOrID string) (string, error) {
	var result struct {
		Channels []map[string]interface{} `json:"channels"`
	}
	if err := c.GetJSON("/api/channels", &result); err != nil {
		return "", err
	}
	for _, ch := range result.Channels {
		if fmt.Sprintf("%v", ch["id"]) == nameOrID {
			return nameOrID, nil // already a UUID
		}
		if fmt.Sprintf("%v", ch["name"]) == nameOrID {
			return fmt.Sprintf("%v", ch["id"]), nil
		}
	}
	return "", fmt.Errorf("channel not found: %s", nameOrID)
}

// createChannel creates a public channel and returns its id. The TUI uses this to
// start each session in a FRESH channel, so a busy channel's accumulated history
// can't poison the turn.
func (c *Client) createChannel(name string) (string, error) {
	var resp map[string]interface{}
	if err := c.PostJSON("/api/channels", map[string]interface{}{"name": name, "type": "public"}, &resp); err != nil {
		return "", err
	}
	// Trust the id the create returned. Re-resolving by name looked more
	// robust and was the opposite: /api/channels caps its listing, so once a
	// workspace passed that many channels the lookup stopped finding the
	// channel just created, createChannel returned "not found", and the TUI
	// fell back to the SHARED channel — silently undoing the per-session
	// isolation this function exists to provide.
	if id, ok := resp["id"].(string); ok && id != "" {
		return id, nil
	}
	return c.resolveChannelID(name)
}

// checkStatus returns an error if the response is not 2xx
func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	// A gateway that says what is wrong says it in {"error": "..."}: show that
	// sentence, not the HTTP envelope around it. A paste the add box refused
	// read `API error (400): {"error":"that is not valid JSON: …"}`.
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil && strings.TrimSpace(body.Error) != "" {
		return errors.New(body.Error)
	}
	return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
}

// credentials is a single gateway's saved session.
type credentials struct {
	Token     string    `json:"token"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// credentialStore is the on-disk shape of ~/.memdoor/credentials.json. Sessions
// are keyed by gateway URL so logging into one gateway (e.g. a hosted
// memdoor.ai) does NOT evict the session for another (e.g. the local gateway).
// The top-level Token/Email/ExpiresAt fields are the legacy single-session
// format: read for back-compat, never written.
type credentialStore struct {
	Gateways map[string]credentials `json:"gateways,omitempty"`

	Token     string     `json:"token,omitempty"`
	Email     string     `json:"email,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

const defaultGatewayAddr = "http://localhost:18789"

func credentialsPath() string {
	return shared.MemdoorHome("credentials.json")
}

// normalizeGateway canonicalizes a gateway URL for use as a session-map key
// (trailing slashes dropped) so "http://x:1/" and "http://x:1" share a session.
func normalizeGateway(g string) string {
	if g == "" {
		g = defaultGatewayAddr
	}
	return strings.TrimRight(strings.TrimSpace(g), "/")
}

func loadCredentialStore() (*credentialStore, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		return nil, err
	}
	var st credentialStore
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Gateways == nil {
		st.Gateways = map[string]credentials{}
	}
	// Back-compat: a legacy single token belongs to the default (local)
	// gateway. Surface it there so existing local sessions keep working; the
	// next saveCredentials rewrites the file in the per-gateway format.
	if st.Token != "" {
		def := normalizeGateway(defaultGatewayAddr)
		if _, ok := st.Gateways[def]; !ok {
			var exp time.Time
			if st.ExpiresAt != nil {
				exp = *st.ExpiresAt
			}
			st.Gateways[def] = credentials{Token: st.Token, Email: st.Email, ExpiresAt: exp}
		}
	}
	return &st, nil
}

// loadCredentials returns the saved session for the gateway the CLI is currently
// targeting (the --gateway flag / its default), or an error if there is none.
func loadCredentials() (*credentials, error) {
	st, err := loadCredentialStore()
	if err != nil {
		return nil, err
	}
	c, ok := st.Gateways[normalizeGateway(gatewayAddr)]
	if !ok || c.Token == "" {
		return nil, fmt.Errorf("not authenticated for gateway %s", normalizeGateway(gatewayAddr))
	}
	return &c, nil
}

// sessionProblem names the reason a request would be refused BEFORE the
// refusal is read as something else.
//
// A session that has run out does not say so: the channel create is refused,
// the channel listing comes back empty, and the TUI reported "channel not
// found: general" — which sent the reader after a channel that was never the
// problem (2026-09-18, the app's terminal after the 30-day token lapsed). The
// credentials file carries the expiry; read it and say the real thing.
func sessionProblem() error {
	creds, err := loadCredentials()
	if err != nil {
		// A FIRST RUN IS NOT AN EXPIRED SESSION (onboarding walk, 2026-09-27).
		// Someone who has just installed this and typed `memdoor tui` was told
		// to run `memdoor auth login-direct --email … --password …`, a command
		// they have no account for and which is not the documented first step.
		return fmt.Errorf("this machine is not set up yet: run 'memdoor setup' once (it makes the workspace and this machine's user), then 'memdoor tui' in a project")
	}
	if !creds.ExpiresAt.IsZero() && time.Now().After(creds.ExpiresAt) {
		return fmt.Errorf("your session on %s expired %s ago (%s): run 'memdoor auth login-direct --email %s --password <pass>'",
			normalizeGateway(gatewayAddr), humanizeDuration(time.Since(creds.ExpiresAt)),
			creds.ExpiresAt.Format("2006-01-02 15:04"), creds.Email)
	}
	return nil
}

// saveCredentials stores the session for the currently-targeted gateway without
// disturbing sessions saved for other gateways.
func saveCredentials(creds *credentials) error {
	dir := filepath.Dir(credentialsPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Stamp expiry client-side so 'auth status' can show "expires in N days"
	// without a round-trip. Mirrors pkg/auth.SessionTTL (30d).
	if creds.ExpiresAt.IsZero() {
		creds.ExpiresAt = time.Now().Add(auth.SessionTTL)
	}
	st, err := loadCredentialStore()
	if err != nil {
		st = &credentialStore{Gateways: map[string]credentials{}}
	}
	// Migrate forward: drop the legacy top-level fields once we write.
	st.Token, st.Email, st.ExpiresAt = "", "", nil
	st.Gateways[normalizeGateway(gatewayAddr)] = *creds
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credentialsPath(), data, 0600)
}

// deleteCredentials clears the session for the currently-targeted gateway only;
// sessions saved for other gateways are preserved.
func deleteCredentials() error {
	st, err := loadCredentialStore()
	if err != nil {
		return os.Remove(credentialsPath())
	}
	delete(st.Gateways, normalizeGateway(gatewayAddr))
	st.Token, st.Email, st.ExpiresAt = "", "", nil
	if len(st.Gateways) == 0 {
		return os.Remove(credentialsPath())
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credentialsPath(), data, 0600)
}
