package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed *.sql
var migrationFiles embed.FS

// Runner handles database migrations
type Runner struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewRunner creates a new migration runner
func NewRunner(db *sql.DB, logger *slog.Logger) *Runner {
	return &Runner{
		db:     db,
		logger: logger,
	}
}

// Run executes all pending migrations
func (r *Runner) Run() error {
	// Create migrations table if it doesn't exist
	if err := r.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	files, err := r.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Execute each migration
	for _, file := range files {
		if err := r.executeMigration(file); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	r.logger.Info("All migrations completed successfully")
	return nil
}

// createMigrationsTable creates the migrations tracking table
func (r *Runner) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT UNIQUE NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	return err
}

// getMigrationFiles returns sorted list of migration files
func (r *Runner) getMigrationFiles() ([]string, error) {
	entries, err := migrationFiles.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	// Sort files to ensure consistent order
	sort.Strings(files)

	return files, nil
}

// executeMigration executes a single migration file
func (r *Runner) executeMigration(filename string) error {
	// Extract version from filename (e.g., "001_initial.sql" -> "001")
	version := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Check if migration already applied
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		r.logger.Debug("Migration already applied, skipping", slog.String("version", version))
		return nil
	}

	// Read migration file
	content, err := migrationFiles.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Execute migration in a transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute SQL
	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	// Record migration
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Migration applied successfully", slog.String("version", version))
	return nil
}
