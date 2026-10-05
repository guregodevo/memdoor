package filestore

import (
	"context"
	"time"
)

// Repository persists file metadata in the database
type Repository interface {
	Save(ctx context.Context, file *FileAttachment) error
	FindByID(ctx context.Context, id string) (*FileAttachment, error)
	FindByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64][]*FileAttachment, error)
	AttachToMessage(ctx context.Context, fileID string, messageID int64) error
	ListExpired(ctx context.Context, olderThan time.Time) ([]*FileAttachment, error)
	Delete(ctx context.Context, id string) error
	TotalStorageBytes(ctx context.Context) (int64, error)
	FindOldestFiles(ctx context.Context, limit int) ([]*FileAttachment, error)
}
