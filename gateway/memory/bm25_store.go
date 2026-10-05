package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/logs"
	_ "memdoor/pkg/sqlitedriver"
)

// BM25Store implements MemoryStore using SQLite FTS5 for keyword search
// Pattern: OpenClaw's BM25-only fallback mode
type BM25Store struct {
	db  *sql.DB
	log *logs.EventLogger
	// fts is false when SQLite was built without the FTS5 module. Storing still
	// works; search falls back from BM25 ranking to LIKE.
	fts bool
}

// NewBM25Store creates a new BM25-based memory store
// Pattern: OpenClaw ~/.openclaw/memory/{agentId}.sqlite
func NewBM25Store(dbPath string, verbose bool) (*BM25Store, error) {
	log := logs.New("Agent")

	log.Debug("Initializing BM25 store",
		slog.String("path", dbPath))

	// Note: Parent directory should already exist from caller (e.g., memory_tools.go)

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &BM25Store{
		db:  db,
		log: log,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Debug("BM25 store initialized successfully")

	return store, nil
}

// initSchema creates the database tables
func (s *BM25Store) initSchema() error {
	schema := `
	-- Main memories table
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		tags TEXT, -- JSON array
		metadata TEXT, -- JSON object
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	-- FTS5 virtual table for full-text search with BM25 ranking
	-- Pattern: OpenClaw's SQLite FTS5 implementation
	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		id UNINDEXED,
		content,
		tags,
		content='memories',
		content_rowid='rowid'
	);

	-- Triggers to keep FTS5 in sync with main table
	CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, id, content, tags)
		VALUES (new.rowid, new.id, new.content, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
		DELETE FROM memories_fts WHERE rowid = old.rowid;
	END;

	-- Note: update trigger created separately in migration step below
	`

	if _, err := s.db.Exec(schema); err != nil {
		// FTS5 IS OPTIONAL. Losing it must not lose the tool. (Every build now
		// runs the pure-Go driver, pkg/sqlitedriver, which has FTS5 compiled in;
		// the fallback stays for a driver that does not.)
		//
		// mattn/go-sqlite3 compiles FTS5 in only when asked — the Makefile passes
		// CGO_CFLAGS=-DSQLITE_ENABLE_FTS5 — so any binary built with a plain
		// `go build` has no fts5 module, and the CREATE VIRTUAL TABLE above fails
		// with "no such module: fts5". That took the whole memory tool down:
		// measured 2026-08-30 on a live coder turn, `memory` returned "failed to
		// store memory: ... no such module: fts5" and the session's finding was
		// simply lost.
		//
		// A build flag is far too easy to drop for the feature to depend on it, so
		// fall back to the plain table and LIKE search. Ranking gets worse;
		// remembering still works, which is the part that matters.
		if !isMissingFTS5(err) {
			return fmt.Errorf("failed to create schema: %w", err)
		}
		s.fts = false
		if _, err := s.db.Exec(baseSchema); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
		s.log.Warn("SQLite has no FTS5 — memories are stored, search falls back to LIKE",
			slog.String("fix", "build with CGO_CFLAGS=-DSQLITE_ENABLE_FTS5 (see gateway/Makefile)"))
		return nil
	}
	s.fts = true

	// Migration: fix FTS5 update trigger for content-synced tables
	// Must run separately — existing DBs may have the old broken trigger
	if !s.fts {
		return nil
	}
	s.db.Exec("DROP TRIGGER IF EXISTS memories_au")
	s.db.Exec(`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
		DELETE FROM memories_fts WHERE rowid = old.rowid;
		INSERT INTO memories_fts(rowid, id, content, tags)
		VALUES (new.rowid, new.id, new.content, new.tags);
	END`)

	return nil
}

// isMissingFTS5 reports whether an error is SQLite refusing the fts5 module,
// as opposed to a real schema problem worth failing on.
func isMissingFTS5(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such module: fts5")
}

// baseSchema is the schema WITHOUT the full-text parts — everything needed to
// store and retrieve a memory, just not to rank one.
const baseSchema = `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		tags TEXT,
		metadata TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`

// Store saves a new memory
func (s *BM25Store) Store(content string, tags []string, metadata map[string]interface{}) (string, error) {
	// Generate ID
	id := fmt.Sprintf("mem_%d", time.Now().UnixNano())

	// Serialize tags and metadata to JSON
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now().Unix()

	// Insert into database
	_, err = s.db.Exec(`
		INSERT INTO memories (id, content, tags, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, content, string(tagsJSON), string(metadataJSON), now, now)

	if err != nil {
		return "", fmt.Errorf("failed to insert memory: %w", err)
	}

	s.log.Debug("Stored memory",
		slog.String("id", id),
		slog.Any("tags", tags))

	return id, nil
}

// Search finds memories using BM25 keyword search
// Pattern: OpenClaw's FTS5 BM25 search
// Converts natural language queries to FTS5 OR queries for better recall
func (s *BM25Store) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Convert natural language to FTS5 OR query for better recall
	// "Tell me about strategies" → "Tell OR me OR about OR strategies"
	// Filter out very short words (stop words) to reduce noise
	ftsQuery := toFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	// Use FTS5 MATCH for BM25 ranking
	// FTS5's bm25() function returns relevance scores (negative, lower = better)
	// We negate it to make higher scores = better match
	var rows *sql.Rows
	var err error
	if s.fts {
		rows, err = s.db.Query(`
		SELECT m.id, m.content, m.tags, m.metadata, m.created_at, m.updated_at,
		       -bm25(memories_fts) as score
		FROM memories_fts
		JOIN memories m ON memories_fts.id = m.id
		WHERE memories_fts MATCH ?
		ORDER BY bm25(memories_fts)
		LIMIT ?
	`, ftsQuery, limit)
	} else {
		// No FTS5 in this build: substring match, newest first, score 0. Worse
		// ranking, same memories — the alternative was the tool failing outright.
		rows, err = s.db.Query(`
		SELECT id, content, tags, metadata, created_at, updated_at, 0 as score
		FROM memories
		WHERE content LIKE ? OR tags LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var tagsJSON, metadataJSON string
		var createdAt, updatedAt int64

		err := rows.Scan(
			&result.ID,
			&result.Content,
			&tagsJSON,
			&metadataJSON,
			&createdAt,
			&updatedAt,
			&result.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Deserialize tags and metadata
		if err := json.Unmarshal([]byte(tagsJSON), &result.Tags); err != nil {
			result.Tags = []string{}
		}
		if err := json.Unmarshal([]byte(metadataJSON), &result.Metadata); err != nil {
			result.Metadata = map[string]interface{}{}
		}

		result.CreatedAt = time.Unix(createdAt, 0)
		result.UpdatedAt = time.Unix(updatedAt, 0)

		results = append(results, result)
	}

	s.log.Debug("Search completed",
		slog.String("query", query),
		slog.Int("results", len(results)))

	return results, nil
}

// toFTS5Query converts natural language to an FTS5 OR query
// Filters stop words and special characters for better recall
func toFTS5Query(query string) string {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "about": true, "like": true, "through": true,
		"and": true, "or": true, "but": true, "not": true, "no": true,
		"if": true, "then": true, "so": true, "than": true,
		"i": true, "me": true, "my": true, "you": true, "your": true,
		"we": true, "our": true, "it": true, "its": true, "he": true,
		"she": true, "they": true, "them": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
		"what": true, "which": true, "who": true, "whom": true, "how": true,
		"tell": true, "give": true, "show": true, "get": true,
	}

	words := strings.Fields(query)
	var keywords []string
	for _, w := range words {
		// Strip non-alphanumeric chars and lowercase
		clean := strings.ToLower(strings.Trim(w, ".,?!;:\"'()[]{}"))
		if len(clean) < 2 || stopWords[clean] {
			continue
		}
		// Quote each term to handle special FTS5 characters
		keywords = append(keywords, `"`+clean+`"`)
	}

	if len(keywords) == 0 && len(words) > 0 {
		// All words were stop words; use the longest original word
		longest := words[0]
		for _, w := range words[1:] {
			if len(w) > len(longest) {
				longest = w
			}
		}
		return `"` + strings.ToLower(strings.Trim(longest, ".,?!;:\"'()[]{}")) + `"`
	}

	return strings.Join(keywords, " OR ")
}

// Retrieve gets a specific memory by ID
func (s *BM25Store) Retrieve(id string) (*Memory, error) {
	var memory Memory
	var tagsJSON, metadataJSON string
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, content, tags, metadata, created_at, updated_at
		FROM memories
		WHERE id = ?
	`, id).Scan(
		&memory.ID,
		&memory.Content,
		&tagsJSON,
		&metadataJSON,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memory: %w", err)
	}

	// Deserialize tags and metadata
	if err := json.Unmarshal([]byte(tagsJSON), &memory.Tags); err != nil {
		memory.Tags = []string{}
	}
	if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
		memory.Metadata = map[string]interface{}{}
	}

	memory.CreatedAt = time.Unix(createdAt, 0)
	memory.UpdatedAt = time.Unix(updatedAt, 0)

	return &memory, nil
}

// List returns all memories, optionally filtered by tags
func (s *BM25Store) List(tags []string) ([]Memory, error) {
	var query string
	var args []interface{}

	if len(tags) > 0 {
		// Filter by tags (search for any matching tag in JSON array)
		// Simple approach: use LIKE with JSON array
		conditions := make([]string, len(tags))
		for i, tag := range tags {
			conditions[i] = "tags LIKE ?"
			args = append(args, fmt.Sprintf("%%\"%s\"%%", tag))
		}
		query = fmt.Sprintf(`
			SELECT id, content, tags, metadata, created_at, updated_at
			FROM memories
			WHERE %s
			ORDER BY created_at DESC
		`, strings.Join(conditions, " OR "))
	} else {
		query = `
			SELECT id, content, tags, metadata, created_at, updated_at
			FROM memories
			ORDER BY created_at DESC
		`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var memory Memory
		var tagsJSON, metadataJSON string
		var createdAt, updatedAt int64

		err := rows.Scan(
			&memory.ID,
			&memory.Content,
			&tagsJSON,
			&metadataJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Deserialize tags and metadata
		if err := json.Unmarshal([]byte(tagsJSON), &memory.Tags); err != nil {
			memory.Tags = []string{}
		}
		if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
			memory.Metadata = map[string]interface{}{}
		}

		memory.CreatedAt = time.Unix(createdAt, 0)
		memory.UpdatedAt = time.Unix(updatedAt, 0)

		memories = append(memories, memory)
	}

	s.log.Debug("List completed",
		slog.Any("tags", tags),
		slog.Int("results", len(memories)))

	return memories, nil
}

// Update modifies an existing memory's content, tags, and/or metadata
// nil values are left unchanged; the FTS5 trigger auto-syncs on UPDATE
func (s *BM25Store) Update(id string, content *string, tags []string, metadata map[string]interface{}) error {
	// Build dynamic SET clause based on provided fields
	var setClauses []string
	var args []interface{}

	if content != nil {
		setClauses = append(setClauses, "content = ?")
		args = append(args, *content)
	}
	if tags != nil {
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		setClauses = append(setClauses, "tags = ?")
		args = append(args, string(tagsJSON))
	}
	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, string(metadataJSON))
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Always update updated_at
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().Unix())

	// Add WHERE clause
	args = append(args, id)

	query := fmt.Sprintf("UPDATE memories SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}

	s.log.Debug("Updated memory",
		slog.String("id", id))

	return nil
}

// Delete removes a memory by ID
func (s *BM25Store) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}

	s.log.Debug("Deleted memory",
		slog.String("id", id))

	return nil
}

// Close closes the database connection
func (s *BM25Store) Close() error {
	s.log.Debug("Closing BM25 store")
	return s.db.Close()
}
