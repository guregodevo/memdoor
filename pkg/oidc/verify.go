// Package oidc checks an OpenID Connect ID token: its signature against the
// issuer's published keys (JWKS), then the issuer, the audience, the expiry
// and the nonce of the attempt that asked for it. Sign in with ChatGPT and
// Sign in with Google both end in this check.
package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Claims is what an ID token says, as far as a sign-in needs.
type Claims struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      any    `json:"aud"`
	Expires       int64  `json:"exp"`
	Nonce         string `json:"nonce"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// VerifyIDToken checks token's signature against the keys at jwksURL, then
// that it was issued by issuer for clientID, has not expired and carries
// nonce. An issuer written without its scheme (Google's older tokens say
// "accounts.google.com") matches the https form.
func VerifyIDToken(ctx context.Context, hc *http.Client, issuer, jwksURL, token, clientID, nonce string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(hb, &header) != nil {
		return nil, errors.New("unreadable header")
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("unreadable claims")
	}
	var claims Claims
	if err := json.Unmarshal(pb, &claims); err != nil {
		return nil, errors.New("unreadable claims")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("unreadable signature")
	}
	keys, err := FetchJWKS(ctx, hc, jwksURL)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	verified := false
	for _, k := range keys {
		if header.Kid != "" && k.Kid != "" && k.Kid != header.Kid {
			continue
		}
		switch {
		case header.Alg == "RS256" && k.Kty == "RSA":
			if pub, err := k.rsa(); err == nil && rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) == nil {
				verified = true
			}
		case header.Alg == "ES256" && k.Kty == "EC":
			if pub, err := k.ecdsa(); err == nil && len(sig) == 64 {
				r := new(big.Int).SetBytes(sig[:32])
				s := new(big.Int).SetBytes(sig[32:])
				if ecdsa.Verify(pub, sum[:], r, s) {
					verified = true
				}
			}
		}
		if verified {
			break
		}
	}
	if !verified {
		return nil, fmt.Errorf("signature (%s) matched no key of %s", header.Alg, jwksURL)
	}
	if claims.Issuer != issuer && "https://"+claims.Issuer != issuer {
		return nil, fmt.Errorf("issuer %q, want %q", claims.Issuer, issuer)
	}
	if !audienceHas(claims.Audience, clientID) {
		return nil, fmt.Errorf("audience %v is not this client", claims.Audience)
	}
	if claims.Expires > 0 && time.Now().Unix() >= claims.Expires {
		return nil, errors.New("expired")
	}
	if claims.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}
	return &claims, nil
}

func audienceHas(aud any, clientID string) bool {
	switch a := aud.(type) {
	case string:
		return a == clientID
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok && s == clientID {
				return true
			}
		}
	}
	return false
}

// JWK is one published signing key.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k JWK) rsa() (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
}

func (k JWK) ecdsa() (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("curve %s", k.Crv)
	}
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	y, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
}

var jwksCache = struct {
	sync.Mutex
	at   map[string]time.Time
	keys map[string][]JWK
}{at: map[string]time.Time{}, keys: map[string][]JWK{}}

// FetchJWKS reads the keys at u, once an hour per URL.
func FetchJWKS(ctx context.Context, hc *http.Client, u string) ([]JWK, error) {
	jwksCache.Lock()
	if keys, ok := jwksCache.keys[u]; ok && time.Since(jwksCache.at[u]) < time.Hour {
		jwksCache.Unlock()
		return keys, nil
	}
	jwksCache.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the signing keys could not be read: %w", err)
	}
	defer resp.Body.Close()
	var doc struct {
		Keys []JWK `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("the signing keys could not be read: %w", err)
	}
	jwksCache.Lock()
	jwksCache.at[u], jwksCache.keys[u] = time.Now(), doc.Keys
	jwksCache.Unlock()
	return doc.Keys, nil
}
