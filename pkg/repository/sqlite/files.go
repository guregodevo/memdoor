package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/filestore"
	"memdoor/pkg/shared"
)

type fileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) filestore.Repository {
	return &fileRepository{db: db}
}

func (r *fileRepository) Save(ctx context.Context, file *filestore.FileAttachment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO files (id, message_id, channel_id, uploader_id, filename, mime_type, size_bytes, storage_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, file.ID, file.MessageID, file.ChannelID, file.UploaderID.String(), file.Filename, file.MimeType, file.SizeBytes, file.StoragePath, file.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	return nil
}

func (r *fileRepository) FindByID(ctx context.Context, id string) (*filestore.FileAttachment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, message_id, channel_id, uploader_id, filename, mime_type, size_bytes, storage_path, created_at
		FROM files WHERE id = ?
	`, id)
	return scanFile(row)
}

func (r *fileRepository) FindByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64][]*filestore.FileAttachment, error) {
	if len(messageIDs) == 0 {
		return make(map[int64][]*filestore.FileAttachment), nil
	}

	// Build placeholders
	placeholders := ""
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_id, channel_id, uploader_id, filename, mime_type, size_bytes, storage_path, created_at
		FROM files WHERE message_id IN (`+placeholders+`)
		ORDER BY created_at ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query files by message IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]*filestore.FileAttachment)
	for rows.Next() {
		file, err := scanFileRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		if file.MessageID != nil {
			result[*file.MessageID] = append(result[*file.MessageID], file)
		}
	}
	return result, rows.Err()
}

func (r *fileRepository) AttachToMessage(ctx context.Context, fileID string, messageID int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE files SET message_id = ? WHERE id = ?
	`, messageID, fileID)
	if err != nil {
		return fmt.Errorf("attach file to message: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("file not found: %s", fileID)
	}
	return nil
}

func (r *fileRepository) ListExpired(ctx context.Context, olderThan time.Time) ([]*filestore.FileAttachment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_id, channel_id, uploader_id, filename, mime_type, size_bytes, storage_path, created_at
		FROM files WHERE created_at < ?
		ORDER BY created_at ASC
	`, olderThan)
	if err != nil {
		return nil, fmt.Errorf("list expired files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (r *fileRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (r *fileRepository) TotalStorageBytes(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT SUM(size_bytes) FROM files`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum storage bytes: %w", err)
	}
	return total.Int64, nil
}

func (r *fileRepository) FindOldestFiles(ctx context.Context, limit int) ([]*filestore.FileAttachment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_id, channel_id, uploader_id, filename, mime_type, size_bytes, storage_path, created_at
		FROM files ORDER BY created_at ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("find oldest files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// scanFile scans a single row into a FileAttachment
func scanFile(row *sql.Row) (*filestore.FileAttachment, error) {
	var f filestore.FileAttachment
	var uploaderID string
	var messageID sql.NullInt64
	err := row.Scan(&f.ID, &messageID, &f.ChannelID, &uploaderID, &f.Filename, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	f.UploaderID = shared.ActorID(uploaderID)
	if messageID.Valid {
		f.MessageID = &messageID.Int64
	}
	return &f, nil
}

// scanFileRow scans a rows iterator into a FileAttachment
func scanFileRow(rows *sql.Rows) (*filestore.FileAttachment, error) {
	var f filestore.FileAttachment
	var uploaderID string
	var messageID sql.NullInt64
	err := rows.Scan(&f.ID, &messageID, &f.ChannelID, &uploaderID, &f.Filename, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	f.UploaderID = shared.ActorID(uploaderID)
	if messageID.Valid {
		f.MessageID = &messageID.Int64
	}
	return &f, nil
}

// scanFiles scans multiple rows
func scanFiles(rows *sql.Rows) ([]*filestore.FileAttachment, error) {
	var files []*filestore.FileAttachment
	for rows.Next() {
		f, err := scanFileRow(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
