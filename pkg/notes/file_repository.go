package notes

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type fileRepository struct {
	root string
	mu   sync.Mutex
}

// NewFileRepository keeps each conversation's notes as one Markdown file
// under root (~/.memdoor/notes), outside any project folder.
func NewFileRepository(root string) (Repository, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("notes: a folder to keep them in is required")
	}
	return &fileRepository{root: root}, nil
}

var unsafeInName = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func (r *fileRepository) path(key ConversationKey) string {
	return filepath.Join(r.root, unsafeInName.ReplaceAllString(key.String(), "_")+".md")
}

func (r *fileRepository) Load(key ConversationKey) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := os.ReadFile(r.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

func (r *fileRepository) Append(key ConversationKey, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(r.root, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(r.path(key), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func (r *fileRepository) Delete(key ConversationKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.Remove(r.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
