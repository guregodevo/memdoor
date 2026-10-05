package message

import "fmt"

// MessageID uniquely identifies a message
// Auto-increment ID for append-only log
type MessageID int64

// IsZero returns true if the ID is empty
func (id MessageID) IsZero() bool {
	return id == 0
}

// Validate checks if the ID is valid
func (id MessageID) Validate() error {
	if id <= 0 {
		return fmt.Errorf("message ID must be positive: %d", id)
	}
	return nil
}

// String returns the string representation
func (id MessageID) String() string {
	return fmt.Sprintf("%d", id)
}
