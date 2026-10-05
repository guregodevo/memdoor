package mcp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memdoor/pkg/secrets"
)

// Token is a server's sign-in: what to send, and what refreshes it.
type Token struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	TokenEndpoint string    `json:"token_endpoint"`
	ClientID      string    `json:"client_id"`
	ClientSecret  string    `json:"client_secret,omitempty"`
	Resource      string    `json:"resource,omitempty"`
	Scope         string    `json:"scope,omitempty"`
}

// TokenStore keeps sign-ins, keyed by the server's URL; never in the
// config file, so a committed .mcp.json carries no credential.
type TokenStore interface {
	Get(serverURL string) (*Token, error)
	Put(serverURL string, t *Token) error
	Delete(serverURL string) error
}

// NewFileTokenStore keeps tokens in memdoorDir/mcp-tokens.json, each one
// encrypted with the master key (pkg/secrets).
func NewFileTokenStore(memdoorDir string) TokenStore {
	return &fileTokens{path: filepath.Join(memdoorDir, "mcp-tokens.json")}
}

type fileTokens struct {
	path string
	mu   sync.Mutex
}

func (f *fileTokens) read() (map[string]string, error) {
	m := map[string]string{}
	b, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	return m, json.Unmarshal(b, &m)
}

func (f *fileTokens) write(m map[string]string) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *fileTokens) Get(serverURL string) (*Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return nil, err
	}
	enc, ok := m[serverURL]
	if !ok {
		return nil, nil
	}
	plain, err := secrets.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal([]byte(plain), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (f *fileTokens) Put(serverURL string, t *Token) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	enc, err := secrets.Encrypt(string(b))
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return err
	}
	m[serverURL] = enc
	return f.write(m)
}

func (f *fileTokens) Delete(serverURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return err
	}
	if _, ok := m[serverURL]; !ok {
		return nil
	}
	delete(m, serverURL)
	return f.write(m)
}
