package channel

import (
	"fmt"

	"memdoor/pkg/shared"
)

// MembershipID is a composite ID (ChannelID, ActorID)
// This prevents duplicates and makes join/leave idempotent
// No random UUIDs needed - the combination IS the identity
type MembershipID struct {
	ChannelID ChannelID
	ActorID   shared.ActorID
}

// NewMembershipID creates a new membership ID
func NewMembershipID(channelID ChannelID, actorID shared.ActorID) MembershipID {
	return MembershipID{
		ChannelID: channelID,
		ActorID:   actorID,
	}
}

// Validate checks if the membership ID is valid
func (id MembershipID) Validate() error {
	if err := id.ChannelID.Validate(); err != nil {
		return fmt.Errorf("invalid channel ID: %w", err)
	}

	if err := id.ActorID.Validate(); err != nil {
		return fmt.Errorf("invalid actor ID: %w", err)
	}

	return nil
}

// String returns a string representation for logging
func (id MembershipID) String() string {
	return fmt.Sprintf("%s:%s", id.ChannelID, id.ActorID)
}

// Equals checks if two membership IDs are equal
func (id MembershipID) Equals(other MembershipID) bool {
	return id.ChannelID == other.ChannelID && id.ActorID == other.ActorID
}
