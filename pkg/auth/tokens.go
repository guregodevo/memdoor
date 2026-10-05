package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

// tokenGenerator implements TokenGenerator interface
type tokenGenerator struct{}

// NewTokenGenerator creates a new token generator
func NewTokenGenerator() TokenGenerator {
	return &tokenGenerator{}
}

// GenerateToken creates a secure random token
func (t *tokenGenerator) GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashToken hashes a token using SHA-256 for secure storage
func (t *tokenGenerator) HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// HashPassword hashes a password using bcrypt
func (t *tokenGenerator) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword checks if a password matches a hash
func (t *tokenGenerator) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
