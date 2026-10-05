package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
)

// WorkspaceRepository reads the workspaces table. Create/Update/Delete remain
// unsupported here — a SQLite install is single-tenant and its one workspace is
// made at setup — but the reads are real, because the plan lives on this row.
type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspaceRepository(db *sql.DB) repository.WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// Create materialises a workspace locally.
//
// It exists so a gateway can ADOPT the workspace the broker says it belongs
// to. The registry is central — the VPS knows every workspace and its owner —
// while the gateway held its own unrelated row, so the two could name
// different things and nothing could reconcile them. That is exactly what
// happened on 2026-08-26: the broker said one name, the gateway had another,
// and every request was refused with a 401 naming both.
//
// Idempotent on the slug: adopting twice is the normal case, since the gateway
// checks on every boot.
func (r *WorkspaceRepository) Create(ctx context.Context, workspace *domain.Workspace) error {
	if workspace == nil || workspace.Slug == "" {
		return fmt.Errorf("a workspace needs a slug")
	}
	if workspace.ID == uuid.Nil {
		workspace.ID = uuid.New()
	}
	if workspace.Name == "" {
		workspace.Name = workspace.Slug
	}
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, name, slug, owner_id, language, plan, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'en'), 'free', 'active', ?, ?)
		ON CONFLICT(slug) DO NOTHING`,
		workspace.ID.String(), workspace.Name, workspace.Slug,
		nullableID(workspace.OwnerID), workspace.Language, now, now)
	if err != nil {
		return fmt.Errorf("create workspace %q: %w", workspace.Slug, err)
	}
	return nil
}

// nullableID renders a zero UUID as NULL rather than a string of zeroes.
func nullableID(id uuid.UUID) interface{} {
	if id == uuid.Nil {
		return nil
	}
	return id.String()
}

func (r *WorkspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	return r.one(ctx, `WHERE id = ?`, id.String())
}

func (r *WorkspaceRepository) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return r.one(ctx, `WHERE slug = ?`, slug)
}

// one reads a single workspace.
//
// These two were stubs returning "workspaces not supported in SQLite
// (single-tenant)" while the table itself existed and held rows — other code
// read it with hand-written SELECTs. They are implemented because the `plan`
// column now decides whether a workspace's pool gets an on-demand floor, and a
// plan nobody can read is a plan that cannot be enforced.
func (r *WorkspaceRepository) one(ctx context.Context, where string, arg any) (*domain.Workspace, error) {
	var (
		w                      domain.Workspace
		id, owner              string
		language, plan, status sql.NullString
		createdAt, updatedAt   sql.NullTime
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, slug, COALESCE(owner_id, ''), language, plan, status, created_at, updated_at
		 FROM workspaces `+where, arg,
	).Scan(&id, &w.Name, &w.Slug, &owner, &language, &plan, &status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("workspace not found")
	}
	if err != nil {
		return nil, err
	}
	if w.ID, err = uuid.Parse(id); err != nil {
		return nil, fmt.Errorf("workspace %q has an unparseable id: %w", w.Slug, err)
	}
	if owner != "" {
		w.OwnerID, _ = uuid.Parse(owner)
	}
	w.Language, w.Plan, w.Status = language.String, plan.String, status.String
	w.CreatedAt, w.UpdatedAt = createdAt.Time, updatedAt.Time
	return &w, nil
}

func (r *WorkspaceRepository) List(ctx context.Context) ([]*domain.Workspace, error) {
	return nil, fmt.Errorf("workspaces not supported in SQLite (single-tenant)")
}

func (r *WorkspaceRepository) Update(ctx context.Context, workspace *domain.Workspace) error {
	return fmt.Errorf("workspaces not supported in SQLite (single-tenant)")
}

func (r *WorkspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf("workspaces not supported in SQLite (single-tenant)")
}

// UserRepository - Stub implementation (implement similar to BuddyRepository)
type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) repository.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	// TODO: Implement similar to BuddyRepository.Create
	return fmt.Errorf("not implemented yet")
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, username, password_hash, name, avatar_url, email_verified, created_at, updated_at, last_login_at, workspace_id
		FROM users
		WHERE id = ?
	`

	var user domain.User
	var passwordHash sql.NullString
	var lastLoginAt sql.NullTime
	var emailVerified int
	var workspaceID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.Name,
		&user.AvatarURL,
		&emailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&workspaceID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Convert nullable fields
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if workspaceID.Valid {
		user.WorkspaceID = workspaceID.String
	}
	user.EmailVerified = emailVerified != 0

	return &user, nil
}

// GetByIDs retrieves multiple users by their IDs in a single query (batch loading)
// Pattern: Prevents N+1 query problem when loading many users
func (r *UserRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.User, error) {
	if len(ids) == 0 {
		return []*domain.User{}, nil
	}

	// Build parameterized query with placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, email, username, password_hash, name, avatar_url, email_verified, created_at, updated_at, last_login_at, workspace_id
		FROM users
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query users by IDs: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		var passwordHash sql.NullString
		var lastLoginAt sql.NullTime
		var emailVerified int
		var workspaceID sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Username,
			&passwordHash,
			&user.Name,
			&user.AvatarURL,
			&emailVerified,
			&user.CreatedAt,
			&user.UpdatedAt,
			&lastLoginAt,
			&workspaceID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		// Convert nullable fields
		if passwordHash.Valid {
			user.PasswordHash = &passwordHash.String
		}
		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}
		if workspaceID.Valid {
			user.WorkspaceID = workspaceID.String
		}
		user.EmailVerified = emailVerified != 0

		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return users, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, username, password_hash, name, avatar_url, email_verified, created_at, updated_at, last_login_at, workspace_id
		FROM users
		WHERE email = ?
	`

	var user domain.User
	var passwordHash sql.NullString
	var lastLoginAt sql.NullTime
	var emailVerified int
	var workspaceID sql.NullString

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.Name,
		&user.AvatarURL,
		&emailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&workspaceID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with email: %s", email)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	// Convert nullable fields
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if workspaceID.Valid {
		user.WorkspaceID = workspaceID.String
	}
	user.EmailVerified = emailVerified != 0

	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, email, username, password_hash, name, avatar_url, email_verified, created_at, updated_at, last_login_at, workspace_id
		FROM users
		WHERE username = ?
	`

	var user domain.User
	var passwordHash sql.NullString
	var lastLoginAt sql.NullTime
	var emailVerified int
	var workspaceID sql.NullString

	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.Name,
		&user.AvatarURL,
		&emailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&workspaceID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with username: %s", username)
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	// Convert nullable fields
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if workspaceID.Valid {
		user.WorkspaceID = workspaceID.String
	}
	user.EmailVerified = emailVerified != 0

	return &user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

// ChannelRepository - Stub implementation
type ChannelRepository struct {
	db *sql.DB
}

func NewChannelRepository(db *sql.DB) repository.ChannelRepository {
	return &ChannelRepository{db: db}
}

func (r *ChannelRepository) Create(ctx context.Context, channel *domain.Channel) error {
	query := `
		INSERT INTO channels (
			id, workspace_id, name, description, is_private, created_at, updated_at, archived_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		channel.ID.String(),
		channel.WorkspaceID.String(),
		channel.Name,
		channel.Description,
		channel.IsPrivate,
		channel.CreatedAt,
		channel.UpdatedAt,
		channel.ArchivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}

	return nil
}

func (r *ChannelRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error) {
	query := `
		SELECT id, workspace_id, name, description, is_private, created_at, updated_at, archived_at
		FROM channels
		WHERE id = ?
	`

	var channel domain.Channel
	var description sql.NullString
	var archivedAt sql.NullTime
	var idStr, workspaceIDStr string

	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&workspaceIDStr,
		&channel.Name,
		&description,
		&channel.IsPrivate,
		&channel.CreatedAt,
		&channel.UpdatedAt,
		&archivedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("channel not found: %s", id.String())
	}
	if err != nil {
		return nil, fmt.Errorf("query channel: %w", err)
	}

	// Parse UUIDs
	channel.ID, _ = uuid.Parse(idStr)
	channel.WorkspaceID, _ = uuid.Parse(workspaceIDStr)

	// Handle nullable fields
	if description.Valid {
		channel.Description = &description.String
	}
	if archivedAt.Valid {
		channel.ArchivedAt = &archivedAt.Time
	}

	return &channel, nil
}

// GetWorkspaceSlug resolves a workspace UUID to its slug. Falls back to
// returning the UUID string if no row matches (matches the
// gateway/agent_handlers.go non-A2A path's behavior, so both paths fail
// the same way instead of one silently breaking).
func (r *ChannelRepository) GetWorkspaceSlug(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	var slug string
	err := r.db.QueryRowContext(ctx,
		`SELECT slug FROM workspaces WHERE id = ?`,
		workspaceID.String(),
	).Scan(&slug)
	if err == sql.ErrNoRows {
		return workspaceID.String(), nil
	}
	if err != nil {
		return "", fmt.Errorf("query workspace slug: %w", err)
	}
	if slug == "" {
		return workspaceID.String(), nil
	}
	return slug, nil
}

func (r *ChannelRepository) GetByName(ctx context.Context, name string) (*domain.Channel, error) {
	query := `SELECT id, workspace_id, name, description, is_private, created_at, updated_at, archived_at
		FROM channels WHERE name = ? LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, name)
	var ch domain.Channel
	var desc sql.NullString
	var archivedAt sql.NullTime
	var wsID string
	err := row.Scan(&ch.ID, &wsID, &ch.Name, &desc, &ch.IsPrivate, &ch.CreatedAt, &ch.UpdatedAt, &archivedAt)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		ch.Description = &desc.String
	}
	if archivedAt.Valid {
		ch.ArchivedAt = &archivedAt.Time
	}
	ch.WorkspaceID, _ = uuid.Parse(wsID)
	return &ch, nil
}

func (r *ChannelRepository) List(ctx context.Context, page shared.OffsetPage) ([]*domain.Channel, error) {
	return r.ListActive(ctx, page)
}

func (r *ChannelRepository) ListActive(ctx context.Context, page shared.OffsetPage) ([]*domain.Channel, error) {
	query := `
		SELECT id, workspace_id, name, description, is_private, created_at, updated_at, archived_at
		FROM channels
		WHERE archived_at IS NULL
		ORDER BY created_at DESC, name ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("query active channels: %w", err)
	}
	defer rows.Close()

	var channels []*domain.Channel
	for rows.Next() {
		var channel domain.Channel
		var description sql.NullString
		var archivedAt sql.NullTime
		var idStr, workspaceIDStr string

		err := rows.Scan(
			&idStr,
			&workspaceIDStr,
			&channel.Name,
			&description,
			&channel.IsPrivate,
			&channel.CreatedAt,
			&channel.UpdatedAt,
			&archivedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}

		// Parse UUIDs
		channel.ID, _ = uuid.Parse(idStr)
		channel.WorkspaceID, _ = uuid.Parse(workspaceIDStr)

		// Handle nullable fields
		if description.Valid {
			channel.Description = &description.String
		}
		if archivedAt.Valid {
			channel.ArchivedAt = &archivedAt.Time
		}

		channels = append(channels, &channel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}

	return channels, nil
}

func (r *ChannelRepository) Update(ctx context.Context, channel *domain.Channel) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *ChannelRepository) Archive(ctx context.Context, id uuid.UUID) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *ChannelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM channels WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("channel not found: %s", id.String())
	}

	return nil
}

// AgentEventRepository - Stub implementation
type AgentEventRepository struct {
	db *sql.DB
}

func NewAgentEventRepository(db *sql.DB) repository.AgentEventRepository {
	return &AgentEventRepository{db: db}
}

func (r *AgentEventRepository) Create(ctx context.Context, event *domain.AgentEvent) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *AgentEventRepository) GetByID(ctx context.Context, id int64) (*domain.AgentEvent, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *AgentEventRepository) GetBySessionID(ctx context.Context, sessionID uuid.UUID, limit int) ([]*domain.AgentEvent, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *AgentEventRepository) GetByBuddyID(ctx context.Context, buddyID uuid.UUID, limit int) ([]*domain.AgentEvent, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *AgentEventRepository) List(ctx context.Context, limit int, offset int) ([]*domain.AgentEvent, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *AgentEventRepository) Delete(ctx context.Context, id int64) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

// BuddyMemoryRepository - Stub implementation
type BuddyMemoryRepository struct {
	db *sql.DB
}

func NewBuddyMemoryRepository(db *sql.DB) repository.BuddyMemoryRepository {
	return &BuddyMemoryRepository{db: db}
}

func (r *BuddyMemoryRepository) Create(ctx context.Context, memory *domain.BuddyMemory) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *BuddyMemoryRepository) GetByID(ctx context.Context, id int64) (*domain.BuddyMemory, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *BuddyMemoryRepository) GetByBuddyID(ctx context.Context, buddyID uuid.UUID, limit int) ([]*domain.BuddyMemory, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *BuddyMemoryRepository) Search(ctx context.Context, buddyID uuid.UUID, query string, limit int) ([]*domain.BuddyMemory, error) {
	// TODO: Implement vector search
	return nil, fmt.Errorf("not implemented yet")
}

func (r *BuddyMemoryRepository) Delete(ctx context.Context, id int64) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *BuddyMemoryRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// TODO: Implement
	return 0, fmt.Errorf("not implemented yet")
}

// SessionRepository - Stub implementation
type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) repository.SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *SessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented yet")
}

func (r *SessionRepository) Update(ctx context.Context, session *domain.Session) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// TODO: Implement
	return fmt.Errorf("not implemented yet")
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// TODO: Implement
	return 0, fmt.Errorf("not implemented yet")
}
