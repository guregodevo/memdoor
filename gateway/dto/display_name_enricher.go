package dto

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
)

// DisplayNameEnricher is a PRESENTATION LAYER component for enriching domain models
// with UI-specific display names. This is NOT part of the domain layer.
//
// Purpose: Convert ActorID (domain concept) to human-readable names for UI display
// - "agent:writer" -> "writer"
// - "human:uuid-123" -> "alice" (username from users table)
//
// This component provides batch loading and optional caching for performance.
type DisplayNameEnricher struct {
	db        *sql.DB
	buddyRepo repository.BuddyRepository
	cache     map[string]string // actorID -> displayName
	cacheMu   sync.RWMutex
	useCache  bool
}

// NewDisplayNameEnricher creates a new display name enricher for the presentation layer
func NewDisplayNameEnricher(db *sql.DB, buddyRepo repository.BuddyRepository, useCache bool) *DisplayNameEnricher {
	return &DisplayNameEnricher{
		db:        db,
		buddyRepo: buddyRepo,
		cache:     make(map[string]string),
		useCache:  useCache,
	}
}

// LoadBatch loads display names for multiple actors in a single operation
// Returns a map of actorID -> displayName
// For agents: extracts name from "agent:name" format or looks up from buddyRepo
// For humans: batch queries the users table
// For unknown formats: returns the actorID as-is
func (l *DisplayNameEnricher) LoadBatch(ctx context.Context, actorIDs []shared.ActorID) map[string]string {
	if len(actorIDs) == 0 {
		return make(map[string]string)
	}

	result := make(map[string]string, len(actorIDs))
	var humanIDs []string

	// Check cache first if enabled
	if l.useCache {
		l.cacheMu.RLock()
		for _, actorID := range actorIDs {
			id := actorID.String()
			if name, ok := l.cache[id]; ok {
				result[id] = name
			}
		}
		l.cacheMu.RUnlock()
	}

	// Separate actors by type (agents vs humans)
	for _, actorID := range actorIDs {
		id := actorID.String()

		// Skip if already in result (from cache)
		if _, ok := result[id]; ok {
			continue
		}

		if strings.HasPrefix(id, "agent:") {
			// Extract agent name from "agent:name" format. Direct mapping
			// is sufficient — the buddyRepo-backed enrichment path that
			// once collected agentNames into a batch lookup was retired
			// when agent display names became 1:1 with the slug after the
			// colon.
			parts := strings.Split(id, ":")
			if len(parts) == 2 {
				result[id] = parts[1]
			} else {
				result[id] = id
			}
		} else if strings.HasPrefix(id, "human:") {
			// Collect human IDs for batch query
			humanIDs = append(humanIDs, id)
		} else {
			// Unknown format - use as-is
			result[id] = id
		}
	}

	// Batch load human names from database
	if len(humanIDs) > 0 && l.db != nil {
		humanNames := l.batchLoadHumanNames(ctx, humanIDs)
		for id, name := range humanNames {
			result[id] = name
		}

		// Fill in fallbacks for humans not found in database
		for _, id := range humanIDs {
			if _, ok := result[id]; !ok {
				// Fallback: extract UUID part
				parts := strings.Split(id, ":")
				if len(parts) == 2 {
					result[id] = parts[1]
				} else {
					result[id] = id
				}
			}
		}
	}

	// Update cache if enabled
	if l.useCache {
		l.cacheMu.Lock()
		for id, name := range result {
			l.cache[id] = name
		}
		l.cacheMu.Unlock()
	}

	return result
}

// LoadSingle loads the display name for a single actor
// Convenience method that wraps LoadBatch
func (l *DisplayNameEnricher) LoadSingle(ctx context.Context, actorID shared.ActorID) string {
	result := l.LoadBatch(ctx, []shared.ActorID{actorID})
	return result[actorID.String()]
}

// batchLoadHumanNames loads usernames for human users from the database
// Returns map of actorID -> username (includes "human:" prefix in keys)
func (l *DisplayNameEnricher) batchLoadHumanNames(ctx context.Context, humanIDs []string) map[string]string {
	result := make(map[string]string)

	if len(humanIDs) == 0 {
		return result
	}

	// Build parameterized query with placeholders
	placeholders := make([]string, len(humanIDs))
	args := make([]interface{}, len(humanIDs))
	for i, id := range humanIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT id, username FROM users WHERE id IN (%s)", strings.Join(placeholders, ","))

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("[DisplayNameLoader] Failed to batch load human names: %v", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var userID, username string
		if err := rows.Scan(&userID, &username); err != nil {
			log.Printf("[DisplayNameLoader] Failed to scan user row: %v", err)
			continue
		}
		result[userID] = username
	}

	if err := rows.Err(); err != nil {
		log.Printf("[DisplayNameLoader] Error iterating user rows: %v", err)
	}

	return result
}

// ClearCache clears the internal cache
// Useful for testing or when data changes
func (l *DisplayNameEnricher) ClearCache() {
	if l.useCache {
		l.cacheMu.Lock()
		l.cache = make(map[string]string)
		l.cacheMu.Unlock()
	}
}

// InvalidateCache removes specific entries from the cache
// Useful when a user's name changes
func (l *DisplayNameEnricher) InvalidateCache(actorIDs []shared.ActorID) {
	if l.useCache {
		l.cacheMu.Lock()
		for _, actorID := range actorIDs {
			delete(l.cache, actorID.String())
		}
		l.cacheMu.Unlock()
	}
}
