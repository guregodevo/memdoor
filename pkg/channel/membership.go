package channel

import (
	"fmt"
	"time"

	"memdoor/pkg/shared"
)

// ChannelMembership is a separate aggregate representing a member in a channel
// Separated from Channel aggregate to handle high-frequency join/leave operations
//
// INVARIANTS:
// 1. Unique (ChannelID, ActorID) - enforced by composite ID
// 2. Private channels require explicit membership (enforced by service)
// 3. At least one admin per channel (enforced by service)
type ChannelMembership struct {
	// Composite ID (natural key, no random UUID needed)
	ID MembershipID

	// Role
	Role MembershipRole

	// Metadata
	JoinedAt  time.Time
	InvitedBy *shared.ActorID // Who invited this member (nil for public channels)
}

// NewMembership creates a new channel membership
// For public channels, invitedBy is nil
// For private channels, invitedBy must be set
func NewMembership(channelID ChannelID, actorID shared.ActorID, role MembershipRole, invitedBy *shared.ActorID) (*ChannelMembership, error) {
	id := NewMembershipID(channelID, actorID)

	if err := id.Validate(); err != nil {
		return nil, fmt.Errorf("invalid membership ID: %w", err)
	}

	if err := role.Validate(); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	return &ChannelMembership{
		ID:        id,
		Role:      role,
		JoinedAt:  time.Now(),
		InvitedBy: invitedBy,
	}, nil
}

// Validate checks if the membership is valid
func (m *ChannelMembership) Validate() error {
	if err := m.ID.Validate(); err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}

	if err := m.Role.Validate(); err != nil {
		return fmt.Errorf("invalid role: %w", err)
	}

	if m.InvitedBy != nil {
		if err := m.InvitedBy.Validate(); err != nil {
			return fmt.Errorf("invalid inviter: %w", err)
		}
	}

	return nil
}

// PromoteToAdmin changes the role to admin
func (m *ChannelMembership) PromoteToAdmin() {
	m.Role = RoleAdmin
}

// DemoteToMember changes the role to member
// Service layer must ensure at least one admin remains
func (m *ChannelMembership) DemoteToMember() error {
	if m.Role == RoleMember {
		return fmt.Errorf("already a member")
	}
	m.Role = RoleMember
	return nil
}

// IsAdmin returns true if the member is an admin
func (m *ChannelMembership) IsAdmin() bool {
	return m.Role.IsAdmin()
}

// WasInvited returns true if the member was explicitly invited
func (m *ChannelMembership) WasInvited() bool {
	return m.InvitedBy != nil
}
