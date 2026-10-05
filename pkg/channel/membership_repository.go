package channel

import (
	"context"

	"memdoor/pkg/shared"
)

// MembershipRepository defines the interface for persisting channel memberships
// Memberships are separate from channels to handle frequent join/leave operations
type MembershipRepository interface {
	// Save persists a membership (upsert - idempotent)
	// INVARIANT: Unique (ChannelID, ActorID) enforced by composite key
	Save(ctx context.Context, membership *ChannelMembership) error

	// FindByID retrieves a membership by composite ID
	FindByID(ctx context.Context, id MembershipID) (*ChannelMembership, error)

	// IsMember checks if an actor is a member of a channel
	// Used for authorization (can this actor post in this channel?)
	IsMember(ctx context.Context, channelID ChannelID, actorID shared.ActorID) (bool, error)

	// IsAdmin checks if an actor is an admin of a channel
	IsAdmin(ctx context.Context, channelID ChannelID, actorID shared.ActorID) (bool, error)

	// ListByChannel retrieves members in a channel with offset pagination
	ListByChannel(ctx context.Context, channelID ChannelID, page shared.OffsetPage) ([]*ChannelMembership, error)

	// ListByActor retrieves all channels an actor is a member of
	ListByActor(ctx context.Context, actorID shared.ActorID) ([]*ChannelMembership, error)

	// CountAdmins counts how many admins are in a channel
	// Used to enforce "at least one admin" invariant
	CountAdmins(ctx context.Context, channelID ChannelID) (int, error)

	// Delete removes a membership (leave channel)
	// Idempotent - safe to call multiple times
	// INVARIANT: Cannot delete if last admin (enforced by service)
	Delete(ctx context.Context, id MembershipID) error
}
