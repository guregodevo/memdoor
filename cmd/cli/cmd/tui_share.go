package cmd

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/pkg/secrets"
	"memdoor/pkg/shared"
)

// /share (roadmap MUST item 5, 2026-09-29): a read-only link to this
// conversation as it is now, https://memdoor.ai/s/<id>#k=<key>. The
// transcript — what was asked and what was answered, not tool output — has
// its secrets removed (secrets.RedactText), is sealed here with a fresh
// AES-256-GCM key, and only the ciphertext is uploaded
// (gateway/share_store.go). The key after # never reaches memdoor.ai; the
// page at /s/<id> opens it in the browser. /unshare deletes every share of
// this conversation. Four of the five agents compared on 2026-09-29 have a
// share; only omp encrypted it.

const (
	// shareAAD binds a sealed transcript to its purpose.
	shareAAD = "memdoor/share/v1"
	// shareMaxBytes is the server's cap (gateway shareMaxBytes).
	shareMaxBytes = 1 << 20
	// shareTranscriptLimit is how many messages a share reads.
	shareTranscriptLimit = 1000
)

// shareDoc is what the page shows.
type shareDoc struct {
	V        int         `json:"v"`
	Title    string      `json:"title"`
	Shared   time.Time   `json:"shared"`
	Messages []shareLine `json:"messages"`
	// Dropped is how many of the oldest messages did not fit.
	Dropped int `json:"dropped,omitempty"`
}

type shareLine struct {
	Role string `json:"role"` // "user" | "assistant"
	Text string `json:"text"`
}

// shareSession is what /share needs from the TUI.
type shareSession struct {
	Account    string
	ChannelID  string
	Transcript func() []ui.Message
}

// shareRecord remembers a share so /unshare can delete it.
type shareRecord struct {
	ID      string    `json:"id"`
	Key     string    `json:"key"`
	Created time.Time `json:"created"`
}

func sharesPath() string {
	return shared.MemdoorHome("shares.json")
}

func loadShares() map[string][]shareRecord {
	out := map[string][]shareRecord{}
	if b, err := os.ReadFile(sharesPath()); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func saveShares(all map[string][]shareRecord) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sharesPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(sharesPath(), b, 0o600) // it holds keys: owner only
}

func (r shareRecord) url() string { return remoteURL() + "/s/" + r.ID + "#k=" + r.Key }

// newShareDoc is the conversation, secrets removed.
func newShareDoc(msgs []ui.Message, now time.Time) shareDoc {
	doc := shareDoc{V: 1, Shared: now}
	for _, m := range msgs {
		text := strings.TrimSpace(secrets.RedactText(m.Content))
		if text == "" {
			continue
		}
		role := "assistant"
		if m.Role == "user" {
			role = "user"
			if doc.Title == "" {
				doc.Title = firstLine(text)
			}
		}
		doc.Messages = append(doc.Messages, shareLine{Role: role, Text: text})
	}
	return doc
}

func shareAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealShare seals doc under key, dropping the oldest messages until it fits.
func sealShare(doc shareDoc, key []byte) ([]byte, shareDoc, error) {
	aead, err := shareAEAD(key)
	if err != nil {
		return nil, doc, err
	}
	for {
		plain, err := json.Marshal(doc)
		if err != nil {
			return nil, doc, err
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return nil, doc, err
		}
		sealed := aead.Seal(nonce, nonce, plain, []byte(shareAAD))
		if len(sealed) <= shareMaxBytes {
			return sealed, doc, nil
		}
		if len(doc.Messages) <= 1 {
			return nil, doc, errors.New("the conversation is too long to share, even its last message")
		}
		drop := max(1, len(doc.Messages)/10)
		doc.Messages = doc.Messages[drop:]
		doc.Dropped += drop
	}
}

func shareCall(method, url, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// tuiShare answers /share and /unshare ("off").
func tuiShare(s shareSession) func(arg string) (string, error) {
	return func(arg string) (string, error) {
		if s.Account == "" {
			return "", errors.New("sharing needs a memdoor.ai sign-in: run `memdoor login you@example.com` first, then try /share again")
		}
		if strings.TrimSpace(arg) == "off" {
			return unshare(s)
		}
		msgs := s.Transcript()
		doc := newShareDoc(msgs, time.Now().UTC())
		if len(doc.Messages) == 0 {
			return "", errors.New("nothing to share yet: this conversation has no messages")
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return "", err
		}
		sealed, doc, err := sealShare(doc, key)
		if err != nil {
			return "", err
		}
		body, err := shareCall(http.MethodPost, remoteURL()+"/api/share", s.Account, sealed)
		if err != nil {
			return "", fmt.Errorf("could not upload the share: %w", err)
		}
		var created struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &created) != nil || created.ID == "" {
			return "", fmt.Errorf("the server did not name the share: %s", body)
		}
		rec := shareRecord{ID: created.ID, Key: base64.RawURLEncoding.EncodeToString(key), Created: time.Now()}
		all := loadShares()
		all[s.ChannelID] = append(all[s.ChannelID], rec)
		if err := saveShares(all); err != nil {
			return "", err
		}
		note := ""
		if doc.Dropped > 0 {
			note = fmt.Sprintf(" The %d oldest did not fit and were left out.", doc.Dropped)
		}
		return fmt.Sprintf("A read-only link to this conversation as it is now — %d messages, secrets removed.%s\n\n%s\n\n"+
			"Anyone with the link can read it; the key after # never reaches memdoor.ai. /unshare deletes it.",
			len(doc.Messages), note, rec.url()), nil
	}
}

func unshare(s shareSession) (string, error) {
	all := loadShares()
	recs := all[s.ChannelID]
	if len(recs) == 0 {
		return "This conversation has no shared links.", nil
	}
	var kept []shareRecord
	var failed []string
	for _, r := range recs {
		if _, err := shareCall(http.MethodDelete, remoteURL()+"/api/share/"+r.ID, s.Account, nil); err != nil {
			kept = append(kept, r)
			failed = append(failed, err.Error())
		}
	}
	if len(kept) == 0 {
		delete(all, s.ChannelID)
	} else {
		all[s.ChannelID] = kept
	}
	if err := saveShares(all); err != nil {
		return "", err
	}
	if len(failed) > 0 {
		return "", fmt.Errorf("%d of %d shares could not be deleted: %s", len(failed), len(recs), failed[0])
	}
	return fmt.Sprintf("Deleted %d shared link(s) to this conversation: they open nothing now.", len(recs)), nil
}
