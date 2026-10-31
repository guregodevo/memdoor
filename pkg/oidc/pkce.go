package oidc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEChallenge is the S256 code challenge of a verifier (RFC 7636): the
// base64url of its SHA-256, unpadded. Every browser sign-in Memdoor runs
// (ChatGPT, OpenRouter, an MCP server's) sends it.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// RandomString is n random URL-safe characters: a PKCE verifier, a state, a
// nonce.
func RandomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
