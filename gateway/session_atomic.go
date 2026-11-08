package gateway

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// appendSessionBatch appends a batch of records and syncs it. A batch that
// fails is cut off again, so the transcript holds all of it or none of it.
// Appending costs the batch, not the transcript: a copy-and-rename rewrote
// the whole file on every tool round. The caller holds the store lock.
func appendSessionBatch(path string, appendRecords func(io.Writer) error) (err error) {
	_, statErr := os.Stat(path)
	created := os.IsNotExist(statErr)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat transcript: %w", err)
	}
	size := info.Size()
	defer func() {
		if err != nil {
			err = errors.Join(err, rollBackSessionBatch(file, path, size, created))
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			err = fmt.Errorf("close transcript: %w", closeErr)
		}
	}()

	if err := endTornRecord(file, size); err != nil {
		return err
	}
	if err := appendRecords(file); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync transcript: %w", err)
	}
	if created {
		return syncDir(filepath.Dir(path))
	}
	return nil
}

// endTornRecord ends a last line a crash left without its newline, so the
// batch's first record is not glued to it. The torn line stays unreadable
// and the reader skips it, as it skips any malformed line.
func endTornRecord(file *os.File, size int64) error {
	if size == 0 {
		return nil
	}
	last := make([]byte, 1)
	if _, err := file.ReadAt(last, size-1); err != nil {
		return fmt.Errorf("read transcript end: %w", err)
	}
	if last[0] == '\n' {
		return nil
	}
	if _, err := file.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("end torn transcript record: %w", err)
	}
	return nil
}

func rollBackSessionBatch(file *os.File, path string, size int64, created bool) error {
	truncErr := file.Truncate(size)
	closeErr := file.Close()
	if created {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove failed transcript: %w", err)
		}
		return nil
	}
	if truncErr != nil {
		return fmt.Errorf("cut failed batch from transcript: %w", truncErr)
	}
	return closeErr
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open transcript dir: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync transcript dir: %w", err)
	}
	return nil
}
