package gateway

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// appendSessionBatch leaves the original transcript unchanged until every
// record has been written, synced, and closed. The caller holds the store lock.
func appendSessionBatch(path string, appendRecords func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-save-*")
	if err != nil {
		return fmt.Errorf("create transcript checkpoint: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	old, err := os.Open(path)
	if err == nil {
		_, copyErr := io.Copy(tmp, old)
		closeErr := old.Close()
		if copyErr != nil {
			return fmt.Errorf("copy transcript: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close original transcript: %w", closeErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read transcript: %w", err)
	}
	if err := appendRecords(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync transcript checkpoint: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close transcript checkpoint: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("commit transcript checkpoint: %w", err)
	}
	return nil
}
