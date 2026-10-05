package authorization

import (
	"context"
	"fmt"

	"memdoor/pkg/auth"
	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// Permission represents a specific action that can be performed
type Permission string

const (
	// Channel permissions
	PermissionReadChannel   Permission = "channel:read"
	PermissionWriteChannel  Permission = "channel:write"
	PermissionDeleteChannel Permission = "channel:delete"
	PermissionManageChannel Permission = "channel:manage" // Add/remove members, change settings

	// Message permissions
	PermissionReadMessages   Permission = "message:read"
	PermissionWriteMessages  Permission = "message:write"
	PermissionDeleteMessages Permission = "message:delete"
	PermissionEditMessages   Permission = "message:edit"

	// Member permissions
	PermissionAddMembers    Permission = "members:add"
	PermissionRemoveMembers Permission = "members:remove"
	PermissionViewMembers   Permission = "members:view"

	// Reaction permissions
	PermissionAddReaction    Permission = "reaction:add"
	PermissionRemoveReaction Permission = "reaction:remove"
)

// Role represents a set of permissions
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleGuest  Role = "guest" // Read-only access
)

// rolePermissions maps roles to their permissions
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		// Admin has all permissions
		PermissionReadChannel,
		PermissionWriteChannel,
		PermissionDeleteChannel,
		PermissionManageChannel,
		PermissionReadMessages,
		PermissionWriteMessages,
		PermissionDeleteMessages,
		PermissionEditMessages,
		PermissionAddMembers,
		PermissionRemoveMembers,
		PermissionViewMembers,
		PermissionAddReaction,
		PermissionRemoveReaction,
	},
	RoleMember: {
		// Members can read/write but not delete channels
		PermissionReadChannel,
		PermissionWriteChannel,
		PermissionReadMessages,
		PermissionWriteMessages,
		PermissionEditMessages, // Can edit own messages
		PermissionViewMembers,
		PermissionAddMembers, // Members can invite others to collaborate
		PermissionAddReaction,
		PermissionRemoveReaction,
	},
	RoleGuest: {
		// Guests have read-only access
		PermissionReadChannel,
		PermissionReadMessages,
		PermissionViewMembers,
	},
}

// HasPermission checks if a role has a specific permission
func (r Role) HasPermission(perm Permission) bool {
	perms, ok := rolePermissions[r]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// Service provides authorization checking
type Service struct {
	membershipRepo channel.MembershipRepository
	channelRepo    repository.ChannelRepository
	buddyRepo      repository.BuddyRepository
	authRepo       auth.Repository
}

// NewService creates a new authorization service
func NewService(membershipRepo channel.MembershipRepository, channelRepo repository.ChannelRepository, buddyRepo repository.BuddyRepository, authRepo auth.Repository) *Service {
	return &Service{
		membershipRepo: membershipRepo,
		channelRepo:    channelRepo,
		buddyRepo:      buddyRepo,
		authRepo:       authRepo,
	}
}

// CanAccessChannel checks if an actor can access a channel with a specific permission
func (s *Service) CanAccessChannel(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID, perm Permission) (bool, error) {
	// Check if actor is a member
	isMember, err := s.membershipRepo.IsMember(ctx, channelID, actorID)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}

	if !isMember {
		return false, nil
	}

	// Get actor's role
	membershipID := channel.NewMembershipID(channelID, actorID)
	membership, err := s.membershipRepo.FindByID(ctx, membershipID)
	if err != nil {
		return false, fmt.Errorf("failed to get membership: %w", err)
	}

	// Check if role has permission
	role := Role(membership.Role.String())
	return role.HasPermission(perm), nil
}

// CanReadChannel checks if actor can read channel
// For public channels, anyone can read even without membership (browse/discover)
// For private channels, must be a member
func (s *Service) CanReadChannel(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID) (bool, error) {
	// First check if they're a member
	allowed, err := s.CanAccessChannel(ctx, actorID, channelID, PermissionReadChannel)
	if err != nil {
		return false, err
	}

	if allowed {
		// Member with read permission
		return true, nil
	}

	// Not a member - check if channel is public
	// Public channels allow anyone to read (browse/discover) without joining
	channelUUID, err := uuid.Parse(string(channelID))
	if err != nil {
		return false, fmt.Errorf("invalid channel ID: %w", err)
	}

	channel, err := s.channelRepo.GetByID(ctx, channelUUID)
	if err != nil {
		return false, fmt.Errorf("failed to get channel: %w", err)
	}

	// Allow reading public channels even without membership
	return !channel.IsPrivate, nil
}

// CanWriteToChannel checks if actor can write to channel
func (s *Service) CanWriteToChannel(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID) (bool, error) {
	return s.CanAccessChannel(ctx, actorID, channelID, PermissionWriteMessages)
}

// CanManageChannel checks if actor can manage channel (add/remove members, delete, etc.)
func (s *Service) CanManageChannel(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID) (bool, error) {
	return s.CanAccessChannel(ctx, actorID, channelID, PermissionManageChannel)
}

// CanDeleteMessage checks if actor can delete a message
// Users can delete their own messages, or admins can delete any message
func (s *Service) CanDeleteMessage(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID, messageAuthorID shared.ActorID) (bool, error) {
	// Check if actor is the message author
	if actorID == messageAuthorID {
		return true, nil
	}

	// Check if actor is admin
	return s.CanAccessChannel(ctx, actorID, channelID, PermissionDeleteMessages)
}

// CanEditMessage checks if actor can edit a message
// Users can only edit their own messages
func (s *Service) CanEditMessage(ctx context.Context, actorID shared.ActorID, messageAuthorID shared.ActorID) (bool, error) {
	// Only message author can edit
	return actorID == messageAuthorID, nil
}

// CanAddMembers checks if actor can add members to channel
func (s *Service) CanAddMembers(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID) (bool, error) {
	return s.CanAccessChannel(ctx, actorID, channelID, PermissionAddMembers)
}

// CanRemoveMembers checks if actor can remove members from channel
func (s *Service) CanRemoveMembers(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID) (bool, error) {
	return s.CanAccessChannel(ctx, actorID, channelID, PermissionRemoveMembers)
}

// RequirePermission returns an error if actor doesn't have permission
func (s *Service) RequirePermission(ctx context.Context, actorID shared.ActorID, channelID channel.ChannelID, perm Permission) error {
	allowed, err := s.CanAccessChannel(ctx, actorID, channelID, perm)
	if err != nil {
		return err
	}

	if !allowed {
		return &PermissionDeniedError{
			ActorID:    actorID,
			ChannelID:  channelID,
			Permission: perm,
		}
	}

	return nil
}

// PermissionDeniedError represents an authorization failure
type PermissionDeniedError struct {
	ActorID    shared.ActorID
	ChannelID  channel.ChannelID
	Permission Permission
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("actor %s does not have permission %s for channel %s",
		e.ActorID, e.Permission, e.ChannelID)
}

// Workspace admins can manage all heartbeat items

// Check if actor is the agent's creator

// IsWorkspaceAdmin checks if actor is a workspace administrator
// Workspace admins have elevated permissions for managing users, agents, and workspace settings
func (s *Service) IsWorkspaceAdmin(ctx context.Context, actorID shared.ActorID) (bool, error) {
	// Internal service calls are always admin
	if actorID == "system:internal" {
		return true, nil
	}

	// Only human users can be admins
	// ActorID format: "human:USER_ID" or "agent:AGENT_NAME"
	if len(actorID) < 7 || actorID[:6] != "human:" {
		return false, nil
	}

	// In the database, user IDs are stored with the "human:" prefix
	// So we use the full ActorID as the userID
	userID := string(actorID)

	// Fetch user from auth repository
	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user has admin role
	return user.Role == domain.UserRoleAdmin, nil
}

// IsWorkspaceAdminFor reports whether the actor is an admin OF a specific
// workspace: role==admin AND the actor's own workspace_id matches the target.
// This is the workspace-scoped check that stops an admin of one workspace
// from acting on another (cross-tenant IDOR). system:internal stays global.
// An empty targetWorkspaceID falls back to the unscoped role check (callers
// that can't determine a target), so always pass a real target at IDOR sites.
func (s *Service) IsWorkspaceAdminFor(ctx context.Context, actorID shared.ActorID, targetWorkspaceID string) (bool, error) {
	// Internal service calls are always admin, globally.
	if actorID == "system:internal" {
		return true, nil
	}

	// Only human users can be admins.
	if len(actorID) < 7 || actorID[:6] != "human:" {
		return false, nil
	}

	// In the database, user IDs are stored with the "human:" prefix.
	userID := string(actorID)

	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	if user.Role != domain.UserRoleAdmin {
		return false, nil
	}

	// Scoped check: an admin may only act on their own workspace. An empty
	// target falls back to the role check above (caller couldn't resolve a
	// target).
	if targetWorkspaceID != "" && user.WorkspaceID != targetWorkspaceID {
		return false, nil
	}

	return true, nil
}
