package filestore

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
)

// SQLiteFileStore stores file content as BLOBs in SQLite.
// This enables automatic replication via Raft in cluster mode.
type SQLiteFileStore struct {
	db *sql.DB
}

func NewSQLiteFileStore(db *sql.DB) FileStore {
	return &SQLiteFileStore{db: db}
}

func (s *SQLiteFileStore) Save(ctx context.Context, id string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read file content: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO file_data (id, data) VALUES (?, ?)`,
		id, data,
	)
	if err != nil {
		return "", fmt.Errorf("store file data: %w", err)
	}

	// storagePath is the DB key — the file ID itself
	return "db:" + id, nil
}

func (s *SQLiteFileStore) Open(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	// Extract ID from "db:<id>" path
	id := storagePath
	if len(storagePath) > 3 && storagePath[:3] == "db:" {
		id = storagePath[3:]
	}

	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM file_data WHERE id = ?`, id,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("read file data: %w", err)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *SQLiteFileStore) Delete(ctx context.Context, storagePath string) error {
	id := storagePath
	if len(storagePath) > 3 && storagePath[:3] == "db:" {
		id = storagePath[3:]
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM file_data WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file data: %w", err)
	}
	return nil
}
