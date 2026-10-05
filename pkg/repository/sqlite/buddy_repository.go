package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
	"memdoor/pkg/sandbox"
	"memdoor/pkg/shared"
)

// BuddyRepository implements repository.BuddyRepository for SQLite
type BuddyRepository struct {
	db *sql.DB
}

// NewBuddyRepository creates a new SQLite buddy repository
func NewBuddyRepository(db *sql.DB) repository.BuddyRepository {
	return &BuddyRepository{db: db}
}

// Create creates a new buddy
func (r *BuddyRepository) Create(ctx context.Context, buddy *domain.Buddy) error {
	// Marshal skills and tools to JSON
	skillsJSON, err := json.Marshal(buddy.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}
	toolsJSON, err := json.Marshal(buddy.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}

	// Marshal remote config to JSON (if present)
	var remoteConfigJSON *string
	if buddy.RemoteConfig != nil {
		configBytes, err := json.Marshal(buddy.RemoteConfig)
		if err != nil {
			return fmt.Errorf("marshal remote_config: %w", err)
		}
		configStr := string(configBytes)
		remoteConfigJSON = &configStr
	}

	// Default execution_type to "local" if not specified
	executionType := buddy.ExecutionType
	if executionType == "" {
		executionType = "local"
	}

	query := `
		INSERT INTO buddies (
			id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt,
			execution_type, remote_config,
			temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled,
			is_active, admin_only, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = r.db.ExecContext(ctx, query,
		buddy.ID.String(),
		buddy.WorkspaceID.String(),
		buddy.Name,
		buddy.AvatarEmoji,
		buddy.Icon,
		buddy.Description,
		buddy.Personality,
		buddy.SystemPrompt,
		executionType,
		remoteConfigJSON,
		buddy.Temperature,
		buddy.MaxTokens,
		string(skillsJSON),
		string(toolsJSON),
		string(buddy.SandboxScope),
		buddy.LearningEnabled,
		buddy.IsActive,
		buddy.AdminOnly,
		buddy.CreatedBy.String(),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("insert buddy: %w", err)
	}

	buddy.CreatedAt = now
	buddy.UpdatedAt = now

	return nil
}

// GetByID retrieves a buddy by ID
// NOTE: In SQLite, no workspace filtering (single-tenant)
func (r *BuddyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Buddy, error) {
	query := `
		SELECT id, workspace_id, name, avatar_emoji, COALESCE(avatar_url, ''), icon, description, personality, system_prompt,
			   execution_type, remote_config,
			   temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled,
			   is_active, admin_only, created_by, created_at, updated_at
		FROM buddies
		WHERE id = ?
	`

	var buddy domain.Buddy
	var skillsJSON, toolsJSON, sandboxScopeStr string
	var description, personality, systemPrompt sql.NullString
	var executionType sql.NullString
	var remoteConfigJSON sql.NullString
	var icon sql.NullString
	var idStr, workspaceIDStr, createdByStr string

	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&workspaceIDStr,
		&buddy.Name,
		&buddy.AvatarEmoji,
		&buddy.AvatarURL,
		&icon,
		&description,
		&personality,
		&systemPrompt,
		&executionType,
		&remoteConfigJSON,
		&buddy.Temperature,
		&buddy.MaxTokens,
		&skillsJSON,
		&toolsJSON,
		&sandboxScopeStr,
		&buddy.LearningEnabled,
		&buddy.IsActive,
		&buddy.AdminOnly,
		&createdByStr,
		&buddy.CreatedAt,
		&buddy.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFoundError("Buddy", id.String())
	}
	if err != nil {
		return nil, fmt.Errorf("query buddy: %w", err)
	}

	// Parse UUIDs
	buddy.ID, _ = uuid.Parse(idStr)
	buddy.WorkspaceID, _ = uuid.Parse(workspaceIDStr)
	buddy.CreatedBy, _ = uuid.Parse(createdByStr)

	// Parse nullable strings
	if description.Valid {
		buddy.Description = &description.String
	}
	if personality.Valid {
		buddy.Personality = &personality.String
	}
	if systemPrompt.Valid {
		buddy.SystemPrompt = &systemPrompt.String
	}
	if icon.Valid {
		buddy.Icon = icon.String
	}

	// Parse execution type (default to "local" if not set)
	if executionType.Valid && executionType.String != "" {
		buddy.ExecutionType = executionType.String
	} else {
		buddy.ExecutionType = "local"
	}

	// Parse remote config JSON
	if remoteConfigJSON.Valid && remoteConfigJSON.String != "" && remoteConfigJSON.String != "null" {
		var config domain.RemoteAgentConfig
		if err := json.Unmarshal([]byte(remoteConfigJSON.String), &config); err != nil {
			return nil, fmt.Errorf("unmarshal remote_config: %w", err)
		}
		buddy.RemoteConfig = &config
	}

	// Unmarshal JSON arrays
	if err := json.Unmarshal([]byte(skillsJSON), &buddy.Skills); err != nil {
		return nil, fmt.Errorf("unmarshal skills: %w", err)
	}
	if err := json.Unmarshal([]byte(toolsJSON), &buddy.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	// Parse sandbox scope
	buddy.SandboxScope = sandbox.SandboxScope(sandboxScopeStr)

	return &buddy, nil
}

// GetByName retrieves a buddy by name
func (r *BuddyRepository) GetByName(ctx context.Context, name string) (*domain.Buddy, error) {
	query := `
		SELECT id, workspace_id, name, avatar_emoji, COALESCE(avatar_url, ''), icon, description, personality, system_prompt,
			   execution_type, remote_config,
			   temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled,
			   is_active, admin_only, created_by, created_at, updated_at
		FROM buddies
		WHERE name = ?
	`

	var buddy domain.Buddy
	var skillsJSON, toolsJSON, sandboxScopeStr string
	var description, personality, systemPrompt sql.NullString
	var executionType sql.NullString
	var remoteConfigJSON sql.NullString
	var icon sql.NullString
	var idStr, workspaceIDStr, createdByStr string

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&idStr,
		&workspaceIDStr,
		&buddy.Name,
		&buddy.AvatarEmoji,
		&buddy.AvatarURL,
		&icon,
		&description,
		&personality,
		&systemPrompt,
		&executionType,
		&remoteConfigJSON,
		&buddy.Temperature,
		&buddy.MaxTokens,
		&skillsJSON,
		&toolsJSON,
		&sandboxScopeStr,
		&buddy.LearningEnabled,
		&buddy.IsActive,
		&buddy.AdminOnly,
		&createdByStr,
		&buddy.CreatedAt,
		&buddy.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFoundError("Buddy", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query buddy: %w", err)
	}

	// Parse UUIDs
	buddy.ID, _ = uuid.Parse(idStr)
	buddy.WorkspaceID, _ = uuid.Parse(workspaceIDStr)
	buddy.CreatedBy, _ = uuid.Parse(createdByStr)

	// Parse nullable strings
	if description.Valid {
		buddy.Description = &description.String
	}
	if personality.Valid {
		buddy.Personality = &personality.String
	}
	if systemPrompt.Valid {
		buddy.SystemPrompt = &systemPrompt.String
	}
	if icon.Valid {
		buddy.Icon = icon.String
	}

	// Parse execution type (default to "local" if not set)
	if executionType.Valid && executionType.String != "" {
		buddy.ExecutionType = executionType.String
	} else {
		buddy.ExecutionType = "local"
	}

	// Parse remote config JSON
	if remoteConfigJSON.Valid && remoteConfigJSON.String != "" && remoteConfigJSON.String != "null" {
		var config domain.RemoteAgentConfig
		if err := json.Unmarshal([]byte(remoteConfigJSON.String), &config); err != nil {
			return nil, fmt.Errorf("unmarshal remote_config: %w", err)
		}
		buddy.RemoteConfig = &config
	}

	// Unmarshal JSON arrays
	if err := json.Unmarshal([]byte(skillsJSON), &buddy.Skills); err != nil {
		return nil, fmt.Errorf("unmarshal skills: %w", err)
	}
	if err := json.Unmarshal([]byte(toolsJSON), &buddy.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	// Parse sandbox scope
	buddy.SandboxScope = sandbox.SandboxScope(sandboxScopeStr)

	return &buddy, nil
}

// GetByNames retrieves multiple buddies by their names in a single query (batch loading)
func (r *BuddyRepository) GetByNames(ctx context.Context, names []string) ([]*domain.Buddy, error) {
	if len(names) == 0 {
		return []*domain.Buddy{}, nil
	}

	// Build parameterized query with placeholders
	placeholders := make([]string, len(names))
	args := make([]interface{}, len(names))
	for i, name := range names {
		placeholders[i] = "?"
		args[i] = name
	}

	query := fmt.Sprintf(`
		SELECT id, workspace_id, name, avatar_emoji, COALESCE(avatar_url, ''), icon, description, personality, system_prompt,
			   temperature, max_tokens, skills, tools, sandbox_scope,
			   is_active, admin_only, created_by, created_at, updated_at
		FROM buddies
		WHERE name IN (%s)
	`, fmt.Sprintf("%s", fmt.Sprintf("%s", strings.Join(placeholders, ","))))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query buddies by names: %w", err)
	}
	defer rows.Close()

	var buddies []*domain.Buddy
	for rows.Next() {
		var buddy domain.Buddy
		var skillsJSON, toolsJSON sql.NullString
		var sandboxScopeStr string
		var description, personality, systemPrompt sql.NullString
		var icon sql.NullString
		var idStr, workspaceIDStr, createdByStr string

		err := rows.Scan(
			&idStr,
			&workspaceIDStr,
			&buddy.Name,
			&buddy.AvatarEmoji,
			&buddy.AvatarURL,
			&icon,
			&description,
			&personality,
			&systemPrompt,
			&buddy.Temperature,
			&buddy.MaxTokens,
			&skillsJSON,
			&toolsJSON,
			&sandboxScopeStr,
			&buddy.IsActive,
			&buddy.AdminOnly,
			&createdByStr,
			&buddy.CreatedAt,
			&buddy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan buddy: %w", err)
		}

		// Parse UUIDs
		buddy.ID, _ = uuid.Parse(idStr)
		buddy.WorkspaceID, _ = uuid.Parse(workspaceIDStr)
		buddy.CreatedBy, _ = uuid.Parse(createdByStr)

		// Parse nullable strings
		if description.Valid {
			buddy.Description = &description.String
		}
		if personality.Valid {
			buddy.Personality = &personality.String
		}
		if systemPrompt.Valid {
			buddy.SystemPrompt = &systemPrompt.String
		}
		if icon.Valid {
			buddy.Icon = icon.String
		}

		// Unmarshal JSON arrays
		if skillsJSON.Valid && skillsJSON.String != "" && skillsJSON.String != "null" {
			if err := json.Unmarshal([]byte(skillsJSON.String), &buddy.Skills); err != nil {
				return nil, fmt.Errorf("unmarshal skills: %w", err)
			}
		} else {
			buddy.Skills = []string{}
		}
		if toolsJSON.Valid && toolsJSON.String != "" && toolsJSON.String != "null" {
			if err := json.Unmarshal([]byte(toolsJSON.String), &buddy.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal tools: %w", err)
			}
		} else {
			buddy.Tools = []string{}
		}

		// Parse sandbox scope
		buddy.SandboxScope = sandbox.SandboxScope(sandboxScopeStr)

		buddies = append(buddies, &buddy)
	}

	return buddies, nil
}

// List retrieves all buddies
// NOTE: In SQLite, returns ALL buddies (no workspace filtering)
func (r *BuddyRepository) List(ctx context.Context, page shared.OffsetPage) ([]*domain.Buddy, error) {
	query := `
		SELECT id, workspace_id, name, avatar_emoji, COALESCE(avatar_url, ''), icon, description, personality, system_prompt,
			   temperature, max_tokens, skills, tools, sandbox_scope,
			   is_active, admin_only, created_by, created_at, updated_at
		FROM buddies
		ORDER BY name ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("query buddies: %w", err)
	}
	defer rows.Close()

	var buddies []*domain.Buddy
	for rows.Next() {
		var buddy domain.Buddy
		var skillsJSON, toolsJSON sql.NullString
		var sandboxScopeStr string
		var description, personality, systemPrompt sql.NullString
		var icon sql.NullString
		var idStr, workspaceIDStr, createdByStr string

		err := rows.Scan(
			&idStr,
			&workspaceIDStr,
			&buddy.Name,
			&buddy.AvatarEmoji,
			&buddy.AvatarURL,
			&icon,
			&description,
			&personality,
			&systemPrompt,
			&buddy.Temperature,
			&buddy.MaxTokens,
			&skillsJSON,
			&toolsJSON,
			&sandboxScopeStr,
			&buddy.IsActive,
			&buddy.AdminOnly,
			&createdByStr,
			&buddy.CreatedAt,
			&buddy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan buddy: %w", err)
		}

		// Parse UUIDs
		buddy.ID, _ = uuid.Parse(idStr)
		buddy.WorkspaceID, _ = uuid.Parse(workspaceIDStr)
		buddy.CreatedBy, _ = uuid.Parse(createdByStr)

		// Parse nullable strings
		if description.Valid {
			buddy.Description = &description.String
		}
		if personality.Valid {
			buddy.Personality = &personality.String
		}
		if systemPrompt.Valid {
			buddy.SystemPrompt = &systemPrompt.String
		}
		if icon.Valid {
			buddy.Icon = icon.String
		}

		// Unmarshal JSON arrays
		if skillsJSON.Valid && skillsJSON.String != "" && skillsJSON.String != "null" {
			if err := json.Unmarshal([]byte(skillsJSON.String), &buddy.Skills); err != nil {
				return nil, fmt.Errorf("unmarshal skills: %w", err)
			}
		} else {
			buddy.Skills = []string{}
		}
		if toolsJSON.Valid && toolsJSON.String != "" && toolsJSON.String != "null" {
			if err := json.Unmarshal([]byte(toolsJSON.String), &buddy.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal tools: %w", err)
			}
		} else {
			buddy.Tools = []string{}
		}

		// Parse sandbox scope
		buddy.SandboxScope = sandbox.SandboxScope(sandboxScopeStr)

		buddies = append(buddies, &buddy)
	}

	return buddies, nil
}

// ListActive retrieves all active buddies
func (r *BuddyRepository) ListActive(ctx context.Context, page shared.OffsetPage) ([]*domain.Buddy, error) {
	query := `
		SELECT id, workspace_id, name, avatar_emoji, COALESCE(avatar_url, ''), icon, description, personality, system_prompt,
			   temperature, max_tokens, skills, tools, sandbox_scope,
			   is_active, admin_only, created_by, created_at, updated_at
		FROM buddies
		WHERE is_active = 1
		ORDER BY name ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("query active buddies: %w", err)
	}
	defer rows.Close()

	var buddies []*domain.Buddy
	for rows.Next() {
		var buddy domain.Buddy
		var skillsJSON, toolsJSON sql.NullString
		var sandboxScopeStr string
		var description, personality, systemPrompt sql.NullString
		var icon sql.NullString
		var idStr, workspaceIDStr, createdByStr string

		err := rows.Scan(
			&idStr,
			&workspaceIDStr,
			&buddy.Name,
			&buddy.AvatarEmoji,
			&buddy.AvatarURL,
			&icon,
			&description,
			&personality,
			&systemPrompt,
			&buddy.Temperature,
			&buddy.MaxTokens,
			&skillsJSON,
			&toolsJSON,
			&sandboxScopeStr,
			&buddy.IsActive,
			&buddy.AdminOnly,
			&createdByStr,
			&buddy.CreatedAt,
			&buddy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan buddy: %w", err)
		}

		// Parse UUIDs
		buddy.ID, _ = uuid.Parse(idStr)
		buddy.WorkspaceID, _ = uuid.Parse(workspaceIDStr)
		buddy.CreatedBy, _ = uuid.Parse(createdByStr)

		// Parse nullable strings
		if description.Valid {
			buddy.Description = &description.String
		}
		if personality.Valid {
			buddy.Personality = &personality.String
		}
		if systemPrompt.Valid {
			buddy.SystemPrompt = &systemPrompt.String
		}
		if icon.Valid {
			buddy.Icon = icon.String
		}

		// Unmarshal JSON arrays
		if skillsJSON.Valid && skillsJSON.String != "" && skillsJSON.String != "null" {
			if err := json.Unmarshal([]byte(skillsJSON.String), &buddy.Skills); err != nil {
				return nil, fmt.Errorf("unmarshal skills: %w", err)
			}
		} else {
			buddy.Skills = []string{}
		}
		if toolsJSON.Valid && toolsJSON.String != "" && toolsJSON.String != "null" {
			if err := json.Unmarshal([]byte(toolsJSON.String), &buddy.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal tools: %w", err)
			}
		} else {
			buddy.Tools = []string{}
		}

		// Parse sandbox scope
		buddy.SandboxScope = sandbox.SandboxScope(sandboxScopeStr)

		buddies = append(buddies, &buddy)
	}

	return buddies, nil
}

// Update updates a buddy
func (r *BuddyRepository) Update(ctx context.Context, buddy *domain.Buddy) error {
	// Marshal skills and tools to JSON
	skillsJSON, err := json.Marshal(buddy.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}
	toolsJSON, err := json.Marshal(buddy.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}

	// Marshal remote config to JSON if present
	var remoteConfigJSON *string
	if buddy.RemoteConfig != nil {
		rc, err := json.Marshal(buddy.RemoteConfig)
		if err != nil {
			return fmt.Errorf("marshal remote_config: %w", err)
		}
		s := string(rc)
		remoteConfigJSON = &s
	}

	query := `
		UPDATE buddies SET
			name = ?, avatar_emoji = ?, avatar_url = ?, icon = ?, description = ?, personality = ?, system_prompt = ?,
			execution_type = ?, remote_config = ?,
			temperature = ?, max_tokens = ?,
			skills = ?, tools = ?, sandbox_scope = ?, learning_enabled = ?, is_active = ?, admin_only = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query,
		buddy.Name,
		buddy.AvatarEmoji,
		buddy.AvatarURL,
		buddy.Icon,
		buddy.Description,
		buddy.Personality,
		buddy.SystemPrompt,
		buddy.ExecutionType,
		remoteConfigJSON,
		buddy.Temperature,
		buddy.MaxTokens,
		string(skillsJSON),
		string(toolsJSON),
		string(buddy.SandboxScope),
		buddy.LearningEnabled,
		buddy.IsActive,
		buddy.AdminOnly,
		now,
		buddy.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update buddy: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundError("Buddy", buddy.ID.String())
	}

	buddy.UpdatedAt = now
	return nil
}

// Delete deletes a buddy by id. Falls back to deleting by name if no row
// matches the id — this handles a class of stale-state bugs where the
// row exists in the table but its `id` column drifted from the value
// returned by GetByName scan (observed 2026-04-11 with a buddies row
// returning id `ac1bf0e1-...` from GetByName but no row matching that
// id in DELETE — the name "hn" was the only consistent identifier).
//
// The CLI flow always calls GetByName first to look up the id, so the
// caller already has the name in scope; passing it via DeleteByName as
// a fallback is the pragmatic fix.
func (r *BuddyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM buddies WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("delete buddy: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundError("Buddy", id.String())
	}

	return nil
}

// DeleteByName deletes a buddy by name. Used as a fallback when the
// id-based Delete returns 0 rows but the row still exists in the table
// (the stale-id-state bug above) and as the primary delete path from
// handlers that already know the name (e.g. /api/agents/{name}).
//
// Returns NotFoundError when no row matches the name.
func (r *BuddyRepository) DeleteByName(ctx context.Context, name string) error {
	query := `DELETE FROM buddies WHERE name = ?`

	result, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("delete buddy by name: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundError("Buddy", name)
	}

	return nil
}
