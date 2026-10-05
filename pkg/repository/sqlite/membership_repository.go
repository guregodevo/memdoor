package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/channel"
	"memdoor/pkg/shared"
)

// MembershipRepository implements channel.MembershipRepository for SQLite
type MembershipRepository struct {
	db *sql.DB
}

// NewMembershipRepository creates a new SQLite membership repository
func NewMembershipRepository(db *sql.DB) channel.MembershipRepository {
	return &MembershipRepository{db: db}
}

// Save creates or updates a membership (idempotent)
func (r *MembershipRepository) Save(ctx context.Context, membership *channel.ChannelMembership) error {
	query := `
		INSERT INTO channel_memberships (channel_id, actor_id, role, joined_at, invited_by)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(channel_id, actor_id) DO UPDATE SET
			role = excluded.role,
			invited_by = excluded.invited_by
	`

	var invitedBy *string
	if membership.InvitedBy != nil {
		str := membership.InvitedBy.String()
		invitedBy = &str
	}

	_, err := r.db.ExecContext(ctx, query,
		membership.ID.ChannelID.String(),
		membership.ID.ActorID.String(),
		membership.Role.String(),
		membership.JoinedAt.Format(time.RFC3339),
		invitedBy,
	)
	if err != nil {
		return fmt.Errorf("save membership: %w", err)
	}

	return nil
}

// FindByID retrieves a specific membership
func (r *MembershipRepository) FindByID(ctx context.Context, id channel.MembershipID) (*channel.ChannelMembership, error) {
	query := `
		SELECT channel_id, actor_id, role, joined_at, invited_by
		FROM channel_memberships
		WHERE channel_id = ? AND actor_id = ?
	`

	var channelIDStr, actorIDStr string
	var roleStr string
	var joinedAtStr string
	var invitedBy *string

	err := r.db.QueryRowContext(ctx, query,
		id.ChannelID.String(),
		id.ActorID.String(),
	).Scan(&channelIDStr, &actorIDStr, &roleStr, &joinedAtStr, &invitedBy)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("membership not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find membership: %w", err)
	}

	// Parse role
	var role channel.MembershipRole
	switch roleStr {
	case "admin":
		role = channel.RoleAdmin
	case "member":
		role = channel.RoleMember
	default:
		return nil, fmt.Errorf("unknown role: %s", roleStr)
	}

	// Parse joined_at
	joinedAt, err := time.Parse(time.RFC3339, joinedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse joined_at: %w", err)
	}

	// Parse invited_by
	var invitedByActorID *shared.ActorID
	if invitedBy != nil && *invitedBy != "" {
		actorID := shared.ActorID(*invitedBy)
		invitedByActorID = &actorID
	}

	return &channel.ChannelMembership{
		ID:        id,
		Role:      role,
		JoinedAt:  joinedAt,
		InvitedBy: invitedByActorID,
	}, nil
}

// IsMember checks if an actor is a member of a channel
func (r *MembershipRepository) IsMember(ctx context.Context, channelID channel.ChannelID, actorID shared.ActorID) (bool, error) {
	query := `SELECT COUNT(*) FROM channel_memberships WHERE channel_id = ? AND actor_id = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, channelID.String(), actorID.String()).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}

	return count > 0, nil
}

// IsAdmin checks if an actor is an admin of a channel
func (r *MembershipRepository) IsAdmin(ctx context.Context, channelID channel.ChannelID, actorID shared.ActorID) (bool, error) {
	query := `SELECT role FROM channel_memberships WHERE channel_id = ? AND actor_id = ?`

	var roleStr string
	err := r.db.QueryRowContext(ctx, query, channelID.String(), actorID.String()).Scan(&roleStr)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check admin status: %w", err)
	}

	return roleStr == "admin", nil
}

// ListByChannel retrieves members in a channel with offset pagination
func (r *MembershipRepository) ListByChannel(ctx context.Context, channelID channel.ChannelID, page shared.OffsetPage) ([]*channel.ChannelMembership, error) {
	query := `
		SELECT channel_id, actor_id, role, joined_at, invited_by
		FROM channel_memberships
		WHERE channel_id = ?
		ORDER BY joined_at ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, channelID.String(), page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()

	var memberships []*channel.ChannelMembership
	for rows.Next() {
		var channelIDStr, actorIDStr, roleStr, joinedAtStr string
		var invitedBy *string

		if err := rows.Scan(&channelIDStr, &actorIDStr, &roleStr, &joinedAtStr, &invitedBy); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}

		// Parse components
		actorID := shared.ActorID(actorIDStr)

		var role channel.MembershipRole
		switch roleStr {
		case "admin":
			role = channel.RoleAdmin
		case "member":
			role = channel.RoleMember
		default:
			return nil, fmt.Errorf("unknown role: %s", roleStr)
		}

		joinedAt, err := time.Parse(time.RFC3339, joinedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse joined_at: %w", err)
		}

		var invitedByActorID *shared.ActorID
		if invitedBy != nil && *invitedBy != "" {
			actorID := shared.ActorID(*invitedBy)
			invitedByActorID = &actorID
		}

		memberships = append(memberships, &channel.ChannelMembership{
			ID:        channel.NewMembershipID(channelID, actorID),
			Role:      role,
			JoinedAt:  joinedAt,
			InvitedBy: invitedByActorID,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return memberships, nil
}

// ListByActor retrieves all channels an actor is a member of
func (r *MembershipRepository) ListByActor(ctx context.Context, actorID shared.ActorID) ([]*channel.ChannelMembership, error) {
	query := `
		SELECT channel_id, actor_id, role, joined_at, invited_by
		FROM channel_memberships
		WHERE actor_id = ?
		ORDER BY joined_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, actorID.String())
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()

	var memberships []*channel.ChannelMembership
	for rows.Next() {
		var channelIDStr, actorIDStr, roleStr, joinedAtStr string
		var invitedBy *string

		if err := rows.Scan(&channelIDStr, &actorIDStr, &roleStr, &joinedAtStr, &invitedBy); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}

		// Parse components
		channelID := channel.ChannelID(channelIDStr)

		var role channel.MembershipRole
		switch roleStr {
		case "admin":
			role = channel.RoleAdmin
		case "member":
			role = channel.RoleMember
		default:
			return nil, fmt.Errorf("unknown role: %s", roleStr)
		}

		joinedAt, err := time.Parse(time.RFC3339, joinedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse joined_at: %w", err)
		}

		var invitedByActorID *shared.ActorID
		if invitedBy != nil && *invitedBy != "" {
			actorID := shared.ActorID(*invitedBy)
			invitedByActorID = &actorID
		}

		memberships = append(memberships, &channel.ChannelMembership{
			ID:        channel.NewMembershipID(channelID, actorID),
			Role:      role,
			JoinedAt:  joinedAt,
			InvitedBy: invitedByActorID,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return memberships, nil
}

// CountAdmins counts the number of admins in a channel
func (r *MembershipRepository) CountAdmins(ctx context.Context, channelID channel.ChannelID) (int, error) {
	query := `SELECT COUNT(*) FROM channel_memberships WHERE channel_id = ? AND role = 'admin'`

	var count int
	err := r.db.QueryRowContext(ctx, query, channelID.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}

	return count, nil
}

// Delete removes a membership
func (r *MembershipRepository) Delete(ctx context.Context, id channel.MembershipID) error {
	query := `DELETE FROM channel_memberships WHERE channel_id = ? AND actor_id = ?`

	result, err := r.db.ExecContext(ctx, query, id.ChannelID.String(), id.ActorID.String())
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Idempotent - not an error if already deleted
		return nil
	}

	return nil
}
