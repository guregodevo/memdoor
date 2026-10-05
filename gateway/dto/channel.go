package dto

import (
	"time"

	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
)

// ChannelDTO represents a channel for API responses
type ChannelDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "public", "private", "dm"
	Description string   `json:"description,omitempty"`
	Members     []string `json:"members"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ChannelMemberDTO represents a channel member for API responses
type ChannelMemberDTO struct {
	ActorID     string `json:"actor_id"`
	Name        string `json:"name"` // UI-specific: display name
	AvatarEmoji string `json:"avatar_emoji,omitempty"`
	Icon        string `json:"icon,omitempty"`   // Agent icon filename (e.g., "coder", "writer")
	Status      string `json:"status,omitempty"` // "online", "offline", "busy"
	Role        string `json:"role"`
	JoinedAt    string `json:"joined_at"`
}

// ToChannelDTO converts a domain Channel to a DTO
func ToChannelDTO(ch *domain.Channel) ChannelDTO {
	channelType := "public"
	if ch.IsPrivate {
		if isDM(ch.Name) {
			channelType = "dm"
		} else {
			channelType = "private"
		}
	}

	description := ""
	if ch.Description != nil {
		description = *ch.Description
	}

	return ChannelDTO{
		ID:          ch.ID.String(),
		Name:        ch.Name,
		Type:        channelType,
		Description: description,
		Members:     []string{}, // Can be populated if needed
		CreatedAt:   ch.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   ch.UpdatedAt.Format(time.RFC3339),
	}
}

// ToChannelDTOBatch converts multiple domain Channels to DTOs
func ToChannelDTOBatch(channels []*domain.Channel) []ChannelDTO {
	dtos := make([]ChannelDTO, len(channels))
	for i, ch := range channels {
		dtos[i] = ToChannelDTO(ch)
	}
	return dtos
}

// ToChannelMemberDTO converts a domain ChannelMembership to a DTO with enriched display data
func ToChannelMemberDTO(
	membership *channel.ChannelMembership,
	displayName string,
	avatarEmoji string,
	icon string,
	status string,
) ChannelMemberDTO {
	return ChannelMemberDTO{
		ActorID:     membership.ID.ActorID.String(),
		Name:        displayName,
		AvatarEmoji: avatarEmoji,
		Icon:        icon,
		Status:      status,
		Role:        membership.Role.String(),
		JoinedAt:    membership.JoinedAt.Format(time.RFC3339),
	}
}

// ToChannelMemberDTOBatch converts multiple memberships to DTOs
// displayNames, avatarEmojis, icons, and statuses are maps keyed by actorID
func ToChannelMemberDTOBatch(
	memberships []*channel.ChannelMembership,
	displayNames map[string]string,
	avatarEmojis map[string]string,
	icons map[string]string,
	statuses map[string]string,
) []ChannelMemberDTO {
	dtos := make([]ChannelMemberDTO, len(memberships))
	for i, membership := range memberships {
		actorID := membership.ID.ActorID.String()

		displayName := displayNames[actorID]
		if displayName == "" {
			displayName = actorID
		}

		dtos[i] = ToChannelMemberDTO(
			membership,
			displayName,
			avatarEmojis[actorID],
			icons[actorID],
			statuses[actorID],
		)
	}
	return dtos
}

// isDM checks if a channel name indicates a direct message
func isDM(name string) bool {
	return len(name) > 3 && name[:3] == "dm-"
}
