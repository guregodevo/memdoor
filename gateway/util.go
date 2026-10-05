package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateID generates a random ID with a prefix
func generateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
