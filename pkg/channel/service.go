package channel

import (
	"context"
	"fmt"

	"memdoor/pkg/shared"
)

// Authorizer defines authorization operations (duck typing to avoid circular dependency with pkg/authorization)
type Authorizer interface {
	CanAddMembers(ctx context.Context, actorID shared.ActorID, channelID ChannelID) (bool, error)
	CanRemoveMembers(ctx context.Context, actorID shared.ActorID, channelID ChannelID) (bool, error)
	CanManageChannel(ctx context.Context, actorID shared.ActorID, channelID ChannelID) (bool, error)
}

// ChannelService provides business logic for channel management
// This layer is DATABASE-AGNOSTIC - it only uses repository interfaces
type ChannelService struct {
	membershipRepo MembershipRepository
	authorizer     Authorizer // Authorization service (duck typed interface)
}

// NewChannelService creates a new channel service
func NewChannelService(membershipRepo MembershipRepository, authorizer Authorizer) *ChannelService {
	return &ChannelService{
		membershipRepo: membershipRepo,
		authorizer:     authorizer,
	}
}

// GetMembershipRepository returns the membership repository
func (s *ChannelService) GetMembershipRepository() MembershipRepository {
	return s.membershipRepo
}

// AddMemberRequest is the input for adding a member to a channel
type AddMemberRequest struct {
	ChannelID   ChannelID       `json:"channel_id"`
	ActorID     shared.ActorID  `json:"actor_id"`               // Actor being added
	RequesterID *shared.ActorID `json:"requester_id,omitempty"` // Who is adding this member (nil = system/self-join)
	Role        MembershipRole  `json:"role"`                   // "member" or "admin"
	InvitedBy   *shared.ActorID `json:"invited_by,omitempty"`   // Required for private channels
}

// AddMember adds a member to a channel
// Business logic:
// - AUTHORIZATION: Validates requester has permission to add members
// - Validates channel ID and actor ID
// - Creates membership with proper role
// - Handles invitation tracking for private channels
// - Idempotent: Safe to call multiple times (upsert)
//
// Special cases:
// - RequesterID = nil: System operation or self-join (public channels only)
// - RequesterID = ActorID: Self-join (allowed for public channels)
func (s *ChannelService) AddMember(ctx context.Context, req AddMemberRequest) (*ChannelMembership, error) {
	// Validation
	if err := req.ChannelID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}
	if err := req.ActorID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid actor_id: %w", err)
	}
	if err := req.Role.Validate(); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	// AUTHORIZATION CHECK: Verify requester has permission to add members
	// Pattern: Predicate-based authorization using Specification pattern
	// Skip for system operations (RequesterID = nil) or when authorizer not wired
	if req.RequesterID != nil && s.authorizer != nil {
		// Special case: Allow self-join (users joining public channels themselves)
		// Similar to self-removal pattern in RemoveMember
		isSelfJoin := *req.RequesterID == req.ActorID

		if !isSelfJoin {
			// Create authorization predicate for adding other members
			canAddMembersPredicate := func(req AddMemberRequest) bool {
				allowed, err := s.authorizer.CanAddMembers(ctx, *req.RequesterID, req.ChannelID)
				// If error occurs, consider as unauthorized for safety
				return err == nil && allowed
			}

			// Validate authorization
			if !canAddMembersPredicate(req) {
				return nil, fmt.Errorf("permission denied: actor %s cannot add members to channel %s", *req.RequesterID, req.ChannelID)
			}
		}
	}

	// Create membership domain model
	membership, err := NewMembership(req.ChannelID, req.ActorID, req.Role, req.InvitedBy)
	if err != nil {
		return nil, fmt.Errorf("create membership: %w", err)
	}

	// Save to database (idempotent - upsert)
	if err := s.membershipRepo.Save(ctx, membership); err != nil {
		return nil, fmt.Errorf("save membership: %w", err)
	}

	return membership, nil
}

// RemoveMember removes a member from a channel
// Business logic:
// - AUTHORIZATION: Validates requester has permission to remove members
// - Ensures at least one admin remains
// - Idempotent: Safe to call multiple times
//
// Special cases:
// - RequesterID = nil: System operation (skip authorization)
// - RequesterID = ActorID: Self-removal (allowed for non-admins)
func (s *ChannelService) RemoveMember(ctx context.Context, channelID ChannelID, actorID shared.ActorID, requesterID *shared.ActorID) error {
	// Validation
	if err := channelID.Validate(); err != nil {
		return fmt.Errorf("invalid channel_id: %w", err)
	}
	if err := actorID.Validate(); err != nil {
		return fmt.Errorf("invalid actor_id: %w", err)
	}

	// AUTHORIZATION CHECK: Verify requester has permission to remove members
	// Pattern: Predicate-based authorization using Specification pattern
	// Skip for system operations (RequesterID = nil) or when authorizer not wired
	if requesterID != nil && s.authorizer != nil {
		// Special case: Allow self-removal (users can leave channels)
		isSelfRemoval := *requesterID == actorID

		if !isSelfRemoval {
			// Create authorization predicate for removing other members
			canRemoveMembersPredicate := func(requesterID shared.ActorID, channelID ChannelID) bool {
				allowed, err := s.authorizer.CanRemoveMembers(ctx, requesterID, channelID)
				// If error occurs, consider as unauthorized for safety
				return err == nil && allowed
			}

			// Validate authorization
			if !canRemoveMembersPredicate(*requesterID, channelID) {
				return fmt.Errorf("permission denied: actor %s cannot remove members from channel %s", *requesterID, channelID)
			}
		}
	}

	membershipID := NewMembershipID(channelID, actorID)

	// Check if this member is an admin
	isAdmin, err := s.membershipRepo.IsAdmin(ctx, channelID, actorID)
	if err != nil {
		return fmt.Errorf("check admin status: %w", err)
	}

	// If removing an admin, ensure at least one admin remains
	if isAdmin {
		adminCount, err := s.membershipRepo.CountAdmins(ctx, channelID)
		if err != nil {
			return fmt.Errorf("count admins: %w", err)
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot remove last admin from channel")
		}
	}

	// Delete membership (idempotent)
	if err := s.membershipRepo.Delete(ctx, membershipID); err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}

	return nil
}

// ListMembers retrieves all members in a channel
func (s *ChannelService) ListMembers(ctx context.Context, channelID ChannelID, page shared.OffsetPage) ([]*ChannelMembership, error) {
	return s.membershipRepo.ListByChannel(ctx, channelID, page)
}

// IsMember checks if an actor is a member of a channel
// Used for authorization before posting messages
func (s *ChannelService) IsMember(ctx context.Context, channelID ChannelID, actorID shared.ActorID) (bool, error) {
	return s.membershipRepo.IsMember(ctx, channelID, actorID)
}

// IsAdmin checks if an actor is an admin of a channel
func (s *ChannelService) IsAdmin(ctx context.Context, channelID ChannelID, actorID shared.ActorID) (bool, error) {
	return s.membershipRepo.IsAdmin(ctx, channelID, actorID)
}
