package channel

import "fmt"

// MembershipRole represents a member's role in a channel
type MembershipRole int

const (
	// RoleMember - Regular member (can read/write)
	RoleMember MembershipRole = iota

	// RoleAdmin - Channel admin (can manage members, settings)
	RoleAdmin
)

// String returns the string representation of the role
func (r MembershipRole) String() string {
	switch r {
	case RoleMember:
		return "member"
	case RoleAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// Validate checks if the role is valid
func (r MembershipRole) Validate() error {
	if r != RoleMember && r != RoleAdmin {
		return fmt.Errorf("invalid membership role: %d", r)
	}
	return nil
}

// IsAdmin returns true if the role is admin
func (r MembershipRole) IsAdmin() bool {
	return r == RoleAdmin
}
