package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// issuer is an OpenID provider for a test: a key, its JWKS, and tokens it
// signs with the claims a test asks for.
type issuer struct {
	srv *httptest.Server
	key *rsa.PrivateKey
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	return &issuer{srv: httptest.NewServer(mux), key: key}
}

func (i *issuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	c, _ := json.Marshal(claims)
	signed := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyIDTokenReadsGooglesClaims(t *testing.T) {
	is := newIssuer(t)
	defer is.srv.Close()
	jwks := is.srv.URL + "/certs"
	good := map[string]any{
		"iss": "accounts.google.com", "aud": "client-1", "sub": "1234", "nonce": "n-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "dev@example.com", "email_verified": true, "name": "Dev One",
	}
	c, err := VerifyIDToken(context.Background(), is.srv.Client(), "https://accounts.google.com", jwks, is.token(t, good), "client-1", "n-1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Email != "dev@example.com" || !c.EmailVerified || c.Name != "Dev One" || c.Subject != "1234" {
		t.Fatalf("claims: %+v", c)
	}

	refused := func(why string, mutate func(m map[string]any), clientID, nonce string) {
		m := map[string]any{}
		for k, v := range good {
			m[k] = v
		}
		mutate(m)
		if _, err := VerifyIDToken(context.Background(), is.srv.Client(), "https://accounts.google.com", jwks, is.token(t, m), clientID, nonce); err == nil {
			t.Errorf("%s: accepted", why)
		}
	}
	refused("another audience", func(map[string]any) {}, "client-2", "n-1")
	refused("another nonce", func(map[string]any) {}, "client-1", "n-2")
	refused("another issuer", func(m map[string]any) { m["iss"] = "https://evil.example" }, "client-1", "n-1")
	refused("expired", func(m map[string]any) { m["exp"] = time.Now().Add(-time.Minute).Unix() }, "client-1", "n-1")

	other := newIssuer(t)
	defer other.srv.Close()
	if _, err := VerifyIDToken(context.Background(), is.srv.Client(), "https://accounts.google.com", jwks, other.token(t, good), "client-1", "n-1"); err == nil {
		t.Fatal("a token signed by another key is refused")
	}
}
