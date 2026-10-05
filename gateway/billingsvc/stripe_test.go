package billingsvc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func signStripe(payload []byte, secret string, t time.Time) string {
	ts := fmt.Sprintf("%d", t.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyStripeSignature(t *testing.T) {
	payload := []byte(`{"id":"evt_1"}`)
	now := time.Now()
	good := signStripe(payload, "whsec_test", now)

	if err := verifyStripeSignature(payload, good, "whsec_test", now, 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := verifyStripeSignature(payload, good, "whsec_WRONG", now, 5*time.Minute); err == nil {
		t.Fatal("wrong secret must be rejected")
	}
	if err := verifyStripeSignature([]byte(`{"id":"evt_TAMPERED"}`), good, "whsec_test", now, 5*time.Minute); err == nil {
		t.Fatal("tampered payload must be rejected")
	}
	// A captured event replayed outside the tolerance window is rejected.
	old := signStripe(payload, "whsec_test", now.Add(-time.Hour))
	if err := verifyStripeSignature(payload, old, "whsec_test", now, 5*time.Minute); err == nil {
		t.Fatal("stale timestamp must be rejected")
	}
	if err := verifyStripeSignature(payload, "garbage", "whsec_test", now, 5*time.Minute); err == nil {
		t.Fatal("malformed header must be rejected")
	}
}
