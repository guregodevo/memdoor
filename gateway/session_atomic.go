package gateway

import (
	"fmt"
	"io"
	"os"
)

// appendSessionBatch leaves the original transcript unchanged until every
// record has been written, synced, and closed. The caller holds the store lock.
func appendSessionBatch(path string, appendRecords func(io.Writer) error) error {
	return replaceSessionFile(path, func(tmp io.Writer) error {
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
		return appendRecords(tmp)
	})
}
