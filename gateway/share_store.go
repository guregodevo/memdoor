package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SHARE (roadmap MUST item 5, 2026-09-29): /share in the TUI turns a
// conversation into a read-only link, https://memdoor.ai/s/<id>#k=<key>. The
// TUI seals the transcript with a fresh key before it leaves the machine
// (cmd/cli/cmd/tui_share.go); this store keeps the ciphertext and hands it
// back, and the key after # never reaches it. Fetching is public — the blob
// is useless without the key — creating and deleting need the account, and
// only the account that created a share can delete it (/unshare).

const (
	// shareMaxBytes is the largest sealed transcript: a long conversation's
	// text, not its tool output (the TUI trims the oldest turns to fit).
	shareMaxBytes = 1 << 20
	// shareMaxPerOwner bounds what one account keeps on the server.
	shareMaxPerOwner = 200
)

var (
	errShareTooMany  = errors.New("this account has too many shares: /unshare some first")
	errShareNotYours = errors.New("this share belongs to another account")
)

type shareStore struct {
	dir  string
	auth wsTokenValidator
	mu   sync.Mutex
}

// newShareStore keeps shares as files under dir: <id>.bin (the ciphertext)
// and <id>.owner (who may delete it).
func newShareStore(dir string, auth wsTokenValidator) (*shareStore, error) {
	if auth == nil {
		panic("share store: a token validator is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &shareStore{dir: dir, auth: auth}, nil
}

func (s *shareStore) path(id, ext string) string { return filepath.Join(s.dir, id+ext) }

// owner is who the request's bearer token names, or "".
func (s *shareStore) owner(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	id, err := s.auth.ValidateToken(strings.TrimSpace(token))
	if err != nil {
		return ""
	}
	return id
}

// Handle serves POST /api/share and GET, DELETE /api/share/<id>.
func (s *shareStore) Handle(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/share"), "/")
	switch {
	case id == "" && r.Method == http.MethodPost:
		s.create(w, r)
	case id != "" && r.Method == http.MethodGet:
		s.get(w, id)
	case id != "" && r.Method == http.MethodDelete:
		s.remove(w, r, id)
	default:
		http.Error(w, "POST /api/share, or GET/DELETE /api/share/<id>", http.StatusMethodNotAllowed)
	}
}

func (s *shareStore) create(w http.ResponseWriter, r *http.Request) {
	owner := s.owner(r)
	if owner == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	blob, err := io.ReadAll(http.MaxBytesReader(w, r.Body, shareMaxBytes+1))
	if err != nil || len(blob) > shareMaxBytes {
		http.Error(w, "a share is at most 1 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	if len(blob) == 0 {
		http.Error(w, "nothing to share", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.countLocked(owner) >= shareMaxPerOwner {
		http.Error(w, errShareTooMany.Error(), http.StatusTooManyRequests)
		return
	}
	id, err := newShareID()
	if err != nil {
		http.Error(w, "could not name the share", http.StatusInternalServerError)
		return
	}
	if err := writeFileAtomic(s.path(id, ".bin"), blob); err != nil {
		http.Error(w, "could not store the share", http.StatusInternalServerError)
		return
	}
	if err := writeFileAtomic(s.path(id, ".owner"), []byte(owner)); err != nil {
		_ = os.Remove(s.path(id, ".bin"))
		http.Error(w, "could not store the share", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (s *shareStore) get(w http.ResponseWriter, id string) {
	if !validRelayKey(id) {
		http.Error(w, "no such share", http.StatusNotFound)
		return
	}
	blob, err := os.ReadFile(s.path(id, ".bin"))
	if err != nil {
		http.Error(w, "no such share: it was deleted, or never existed", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(blob)
}

func (s *shareStore) remove(w http.ResponseWriter, r *http.Request, id string) {
	owner := s.owner(r)
	if owner == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !validRelayKey(id) {
		http.Error(w, "no such share", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	who, err := os.ReadFile(s.path(id, ".owner"))
	if err != nil {
		// Already gone: deleting twice is not an error.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if string(who) != owner {
		http.Error(w, errShareNotYours.Error(), http.StatusForbidden)
		return
	}
	_ = os.Remove(s.path(id, ".bin"))
	_ = os.Remove(s.path(id, ".owner"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *shareStore) countLocked(owner string) int {
	entries, _ := os.ReadDir(s.dir)
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".owner") {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(s.dir, e.Name())); err == nil && string(b) == owner {
			n++
		}
	}
	return n
}

// newShareID is 12 random bytes, the same shape as a relay pairing id.
func newShareID() (string, error) {
	b := make([]byte, remoteRelayKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
