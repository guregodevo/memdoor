package shared

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
)

// The X-Internal-Service header names a call from inside the gateway's own
// process (an agent's tool calling the gateway's API), which the auth
// middleware treats as "system:internal". Its value is this process's secret,
// made at first use and never written anywhere: until 2026-10-30 any value
// from loopback passed, so any program on the machine — a sign-up's coder's
// bash, a hosted /remote program — was the gateway itself to every route.
var internalService struct {
	once  sync.Once
	token string
}

// InternalServiceToken is the value an in-process call puts in X-Internal-Service.
func InternalServiceToken() string {
	internalService.once.Do(func() {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("internal service token: " + err.Error())
		}
		internalService.token = hex.EncodeToString(b)
	})
	return internalService.token
}

// IsInternalServiceToken reports whether v is this process's token.
func IsInternalServiceToken(v string) bool {
	return v != "" && subtle.ConstantTimeCompare([]byte(v), []byte(InternalServiceToken())) == 1
}
