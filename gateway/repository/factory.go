package repository

import (
	"fmt"

	"memdoor/gateway/config"
	"memdoor/pkg/repository"
	"memdoor/pkg/repository/sqlite"
)

// NewRepositoryFactory creates a repository factory based on config.
// SQLite is the only supported backend — the file-based repo was
// removed 2026-04-11 because it created a parallel source of truth
// for buddies/agents (reading from `cfg.Agents.List` in config.json
// instead of the buddies SQL table) and led to ghost agents that
// couldn't be deleted via the normal CLI flow. One source of truth:
// the SQLite database.
func NewRepositoryFactory(cfg *config.DatabaseConfig) (repository.RepositoryFactory, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database config is nil")
	}

	dbType := cfg.GetDatabaseType()
	if dbType != "sqlite" {
		return nil, fmt.Errorf("unsupported database type: %s (only 'sqlite' is supported)", dbType)
	}
	path := cfg.GetSQLitePath()
	return sqlite.NewSQLiteFactory(path)
}
