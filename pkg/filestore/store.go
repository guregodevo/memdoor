package filestore

import (
	"context"
	"io"
)

// FileStore abstracts file storage backends (local filesystem, GCS, S3)
type FileStore interface {
	// Save writes file content and returns the storage path
	Save(ctx context.Context, id string, reader io.Reader) (storagePath string, err error)

	// Open returns a reader for the stored file
	Open(ctx context.Context, storagePath string) (io.ReadCloser, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, storagePath string) error
}
