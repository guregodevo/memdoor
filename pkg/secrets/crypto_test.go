package secrets

import (
	"encoding/base64"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTempKey points the master key at a throwaway file so tests never touch
// the real ~/.memdoor/master.key.
func useTempKey(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, shared.MemdoorDirName, "master.key")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	useTempKey(t)

	for _, plaintext := range []string{"", "sk-supersecret", "unicode ✓ 北京", strings.Repeat("x", 5000)} {
		enc, err := Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plaintext, err)
		}
		if enc == plaintext && plaintext != "" {
			t.Errorf("ciphertext equals plaintext for %q", plaintext)
		}
		got, err := Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plaintext {
			t.Errorf("round-trip mismatch: got %q want %q", got, plaintext)
		}
	}
}

// AES-GCM uses a random nonce, so the same plaintext must encrypt to different
// ciphertexts — and both must still decrypt back.
func TestEncrypt_NonceIsRandom(t *testing.T) {
	useTempKey(t)
	a, err := Encrypt("same input")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encrypt("same input")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two encryptions of the same plaintext produced identical ciphertext (nonce reuse?)")
	}
}

// GCM is authenticated: any tampering with the ciphertext must fail to decrypt,
// not silently return corrupted plaintext.
func TestDecrypt_TamperDetected(t *testing.T) {
	useTempKey(t)
	enc, err := Encrypt("integrity-protected")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xFF // flip a bit in the auth tag / ciphertext
	if _, err := Decrypt(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Error("tampered ciphertext decrypted without error — GCM authentication not enforced")
	}
}

func TestDecrypt_RejectsGarbage(t *testing.T) {
	useTempKey(t)
	if _, err := Decrypt("not valid base64 !!!"); err == nil {
		t.Error("expected error for non-base64 input")
	}
	if _, err := Decrypt(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("expected error for ciphertext shorter than the nonce")
	}
}

// A different master key must not be able to read another key's ciphertext.
func TestDecrypt_WrongKeyFails(t *testing.T) {
	useTempKey(t)
	enc, err := Encrypt("locked")
	if err != nil {
		t.Fatal(err)
	}
	// Swap in a fresh key file → new random key.
	t.Setenv("HOME", t.TempDir())
	if _, err := Decrypt(enc); err == nil {
		t.Error("ciphertext decrypted under a different master key")
	}
}

func TestGetMasterKey_CreatesPersistent0600Key(t *testing.T) {
	p := useTempKey(t)

	k1, err := getMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("key length = %d, want 32 (AES-256)", len(k1))
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("key file not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file perms = %o, want 600", perm)
	}
	// Second call must reuse the same key (stable across restarts).
	k2, err := getMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(k1) != string(k2) {
		t.Error("getMasterKey regenerated the key instead of reusing the stored one")
	}
}
