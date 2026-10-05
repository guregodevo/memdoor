package gateway

import "testing"

func TestSessionTier(t *testing.T) {
	s := &Session{Metadata: map[string]interface{}{}}
	if sessionTier(s) != 0 || sessionTier(nil) != 0 {
		t.Fatal("no tier is 0")
	}
	s.SetMetadata(sessionTierKey, 2)
	if sessionTier(s) != 2 {
		t.Fatal("int")
	}
	s.SetMetadata(sessionTierKey, float64(3)) // as read back from disk
	if sessionTier(s) != 3 {
		t.Fatal("float64")
	}
	s.SetMetadata(sessionTierKey, -1)
	if sessionTier(s) != 0 {
		t.Fatal("negative clamps to 0")
	}
}
