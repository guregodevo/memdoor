package channel

import "fmt"

// ChannelID uniquely identifies a channel
type ChannelID string

// IsZero returns true if the ID is empty
func (id ChannelID) IsZero() bool {
	return id == ""
}

// Validate checks if the ID is valid
func (id ChannelID) Validate() error {
	if id.IsZero() {
		return fmt.Errorf("channel ID cannot be empty")
	}
	return nil
}

// String returns the string representation
func (id ChannelID) String() string {
	return string(id)
}
