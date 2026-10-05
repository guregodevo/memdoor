package channel

import (
	"testing"

	"memdoor/pkg/shared"
)

func TestMembershipID(t *testing.T) {
	t.Run("composite ID creation", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")

		id := NewMembershipID(channelID, actorID)

		if id.ChannelID != channelID {
			t.Errorf("MembershipID channelID = %v, want %v", id.ChannelID, channelID)
		}

		if id.ActorID != actorID {
			t.Errorf("MembershipID actorID = %v, want %v", id.ActorID, actorID)
		}

		if err := id.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("composite ID equality", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")

		id1 := NewMembershipID(channelID, actorID)
		id2 := NewMembershipID(channelID, actorID)

		if !id1.Equals(id2) {
			t.Errorf("Equals() = false, want true for same IDs")
		}
	})

	t.Run("composite ID inequality", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actor1 := shared.NewHumanActorID("user-456")
		actor2 := shared.NewHumanActorID("user-789")

		id1 := NewMembershipID(channelID, actor1)
		id2 := NewMembershipID(channelID, actor2)

		if id1.Equals(id2) {
			t.Errorf("Equals() = true, want false for different actors")
		}
	})

	t.Run("string representation", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")

		id := NewMembershipID(channelID, actorID)
		str := id.String()

		expected := "channel-123:human:user-456"
		if str != expected {
			t.Errorf("String() = %v, want %v", str, expected)
		}
	})
}

func TestMembershipRole(t *testing.T) {
	t.Run("member role", func(t *testing.T) {
		role := RoleMember

		if role.String() != "member" {
			t.Errorf("String() = %v, want 'member'", role.String())
		}

		if role.IsAdmin() {
			t.Errorf("IsAdmin() = true, want false for member role")
		}

		if err := role.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("admin role", func(t *testing.T) {
		role := RoleAdmin

		if role.String() != "admin" {
			t.Errorf("String() = %v, want 'admin'", role.String())
		}

		if !role.IsAdmin() {
			t.Errorf("IsAdmin() = false, want true for admin role")
		}

		if err := role.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})
}

func TestNewMembership(t *testing.T) {
	channelID := ChannelID("channel-123")
	actorID := shared.NewHumanActorID("user-456")

	t.Run("valid membership", func(t *testing.T) {
		membership, err := NewMembership(channelID, actorID, RoleMember, nil)
		if err != nil {
			t.Errorf("NewMembership() unexpected error: %v", err)
			return
		}

		if membership.ID.ChannelID != channelID {
			t.Errorf("NewMembership() channelID = %v, want %v", membership.ID.ChannelID, channelID)
		}

		if membership.ID.ActorID != actorID {
			t.Errorf("NewMembership() actorID = %v, want %v", membership.ID.ActorID, actorID)
		}

		if membership.Role != RoleMember {
			t.Errorf("NewMembership() role = %v, want %v", membership.Role, RoleMember)
		}

		if membership.InvitedBy != nil {
			t.Errorf("NewMembership() invitedBy = %v, want nil", membership.InvitedBy)
		}

		if membership.WasInvited() {
			t.Errorf("WasInvited() = true, want false for public join")
		}
	})

	t.Run("membership with inviter", func(t *testing.T) {
		inviterID := shared.NewHumanActorID("admin-789")
		membership, err := NewMembership(channelID, actorID, RoleMember, &inviterID)
		if err != nil {
			t.Errorf("NewMembership() unexpected error: %v", err)
			return
		}

		if !membership.WasInvited() {
			t.Errorf("WasInvited() = false, want true for invite")
		}

		if membership.InvitedBy == nil || *membership.InvitedBy != inviterID {
			t.Errorf("NewMembership() invitedBy = %v, want %v", membership.InvitedBy, inviterID)
		}
	})

	t.Run("invalid channel ID", func(t *testing.T) {
		_, err := NewMembership(ChannelID(""), actorID, RoleMember, nil)
		if err == nil {
			t.Errorf("NewMembership() expected error for empty channel ID")
		}
	})

	t.Run("invalid actor ID", func(t *testing.T) {
		_, err := NewMembership(channelID, shared.ActorID("invalid"), RoleMember, nil)
		if err == nil {
			t.Errorf("NewMembership() expected error for invalid actor ID")
		}
	})
}

func TestMembership_RoleChanges(t *testing.T) {
	channelID := ChannelID("channel-123")
	actorID := shared.NewHumanActorID("user-456")

	t.Run("promote to admin", func(t *testing.T) {
		membership, _ := NewMembership(channelID, actorID, RoleMember, nil)

		if membership.IsAdmin() {
			t.Errorf("IsAdmin() = true, want false before promotion")
		}

		membership.PromoteToAdmin()

		if !membership.IsAdmin() {
			t.Errorf("IsAdmin() = false, want true after promotion")
		}

		if membership.Role != RoleAdmin {
			t.Errorf("Role = %v, want %v", membership.Role, RoleAdmin)
		}
	})

	t.Run("demote to member", func(t *testing.T) {
		membership, _ := NewMembership(channelID, actorID, RoleAdmin, nil)

		if !membership.IsAdmin() {
			t.Errorf("IsAdmin() = false, want true before demotion")
		}

		err := membership.DemoteToMember()
		if err != nil {
			t.Errorf("DemoteToMember() unexpected error: %v", err)
		}

		if membership.IsAdmin() {
			t.Errorf("IsAdmin() = true, want false after demotion")
		}

		if membership.Role != RoleMember {
			t.Errorf("Role = %v, want %v", membership.Role, RoleMember)
		}
	})

	t.Run("demote already member", func(t *testing.T) {
		membership, _ := NewMembership(channelID, actorID, RoleMember, nil)

		err := membership.DemoteToMember()
		if err == nil {
			t.Errorf("DemoteToMember() expected error for already member")
		}
	})
}

func TestMembership_Validate(t *testing.T) {
	t.Run("valid membership", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")
		membership, _ := NewMembership(channelID, actorID, RoleMember, nil)

		if err := membership.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("valid membership with inviter", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")
		inviterID := shared.NewHumanActorID("admin-789")
		membership, _ := NewMembership(channelID, actorID, RoleMember, &inviterID)

		if err := membership.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})
}

func TestMembership_IdempotentOperations(t *testing.T) {
	t.Run("same composite ID prevents duplicates", func(t *testing.T) {
		channelID := ChannelID("channel-123")
		actorID := shared.NewHumanActorID("user-456")

		// Two memberships with same composite ID
		mem1, _ := NewMembership(channelID, actorID, RoleMember, nil)
		mem2, _ := NewMembership(channelID, actorID, RoleMember, nil)

		// They have the same ID (natural key)
		if !mem1.ID.Equals(mem2.ID) {
			t.Errorf("Composite IDs should be equal for same channel+actor")
		}

		// In repository, this would be an upsert (update, not duplicate)
		// This is what makes join/leave idempotent
	})
}
