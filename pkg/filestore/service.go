package filestore

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"memdoor/pkg/shared"
)

// Service coordinates file operations
type Service struct {
	repo   Repository
	store  FileStore
	config Config
}

// NewService creates a new file service
func NewService(repo Repository, store FileStore, config Config) *Service {
	return &Service{
		repo:   repo,
		store:  store,
		config: config,
	}
}

// Upload validates, stores, and persists a file
func (s *Service) Upload(ctx context.Context, channelID string, uploaderID shared.ActorID, filename string, mimeType string, reader io.Reader) (*FileAttachment, error) {
	id := uuid.New().String()

	// Limit reader to max file size
	maxBytes := int64(s.config.MaxFileSizeMB) * 1024 * 1024
	limitedReader := io.LimitReader(reader, maxBytes+1) // +1 to detect overflow

	storagePath, err := s.store.Save(ctx, id, limitedReader)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	// Check actual size by reading file info from storage path
	// The LimitReader caps at maxBytes+1, so if we wrote exactly maxBytes+1 bytes, file is too large
	info, err := s.getFileSize(ctx, storagePath)
	if err != nil {
		s.store.Delete(ctx, storagePath)
		return nil, fmt.Errorf("check file size: %w", err)
	}

	if info > maxBytes {
		s.store.Delete(ctx, storagePath)
		return nil, fmt.Errorf("file exceeds maximum size of %d MB", s.config.MaxFileSizeMB)
	}

	file := &FileAttachment{
		ID:          id,
		ChannelID:   channelID,
		UploaderID:  uploaderID,
		Filename:    filename,
		MimeType:    mimeType,
		SizeBytes:   info,
		StoragePath: storagePath,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Save(ctx, file); err != nil {
		s.store.Delete(ctx, storagePath)
		return nil, fmt.Errorf("save file metadata: %w", err)
	}

	return file, nil
}

// getFileSize opens the file and checks its size
func (s *Service) getFileSize(_ context.Context, storagePath string) (int64, error) {
	f, err := s.store.Open(context.Background(), storagePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Count bytes
	n, err := io.Copy(io.Discard, f)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// GetFile returns file metadata and a content reader
func (s *Service) GetFile(ctx context.Context, fileID string) (*FileAttachment, io.ReadCloser, error) {
	file, err := s.repo.FindByID(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}

	reader, err := s.store.Open(ctx, file.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open file content: %w", err)
	}

	return file, reader, nil
}

// AttachToMessage links file IDs to a message
func (s *Service) AttachToMessage(ctx context.Context, fileIDs []string, messageID int64) error {
	for _, fileID := range fileIDs {
		if err := s.repo.AttachToMessage(ctx, fileID, messageID); err != nil {
			return fmt.Errorf("attach file %s to message %d: %w", fileID, messageID, err)
		}
	}
	return nil
}

// GetAttachmentsBatch loads attachments for multiple messages
func (s *Service) GetAttachmentsBatch(ctx context.Context, messageIDs []int64) (map[int64][]*FileAttachment, error) {
	return s.repo.FindByMessageIDs(ctx, messageIDs)
}

// GetRepository returns the underlying repository
func (s *Service) GetRepository() Repository {
	return s.repo
}

// CleanupExpired deletes files older than retention period and enforces storage cap
func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	deleted := 0

	// Phase 1: Delete files older than retention period
	if s.config.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)
		expired, err := s.repo.ListExpired(ctx, cutoff)
		if err != nil {
			return 0, fmt.Errorf("list expired files: %w", err)
		}

		for _, file := range expired {
			if err := s.store.Delete(ctx, file.StoragePath); err != nil {
				slog.Warn("Failed to delete expired file from store",
					slog.String("file_id", file.ID),
					slog.String("error", err.Error()))
			}
			if err := s.repo.Delete(ctx, file.ID); err != nil {
				slog.Warn("Failed to delete expired file metadata",
					slog.String("file_id", file.ID),
					slog.String("error", err.Error()))
				continue
			}
			deleted++
		}
	}

	// Phase 2: Enforce storage cap
	if s.config.MaxStorageMB > 0 {
		maxBytes := int64(s.config.MaxStorageMB) * 1024 * 1024
		totalBytes, err := s.repo.TotalStorageBytes(ctx)
		if err != nil {
			return deleted, fmt.Errorf("get total storage: %w", err)
		}

		if totalBytes > maxBytes {
			oldest, err := s.repo.FindOldestFiles(ctx, 100)
			if err != nil {
				return deleted, fmt.Errorf("find oldest files: %w", err)
			}

			for _, file := range oldest {
				if totalBytes <= maxBytes {
					break
				}
				if err := s.store.Delete(ctx, file.StoragePath); err != nil {
					slog.Warn("Failed to delete file for storage cap",
						slog.String("file_id", file.ID),
						slog.String("error", err.Error()))
				}
				if err := s.repo.Delete(ctx, file.ID); err != nil {
					continue
				}
				totalBytes -= file.SizeBytes
				deleted++
			}
		}
	}

	if deleted > 0 {
		slog.Info("File cleanup completed", slog.Int("deleted", deleted))
	}

	return deleted, nil
}
