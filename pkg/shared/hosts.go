package shared

import "net/url"

// IsLoopbackURL reports whether raw names this machine: 127.0.0.1,
// localhost or ::1. The CLI sign-in's callback and the account-token
// overrides below accept nothing else (open-redirect token theft).
func IsLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// AccountHostTrusted reports whether the memdoor.ai account token may be sent
// to raw: memdoor.ai over https, or this machine (a local gateway in tests, a
// self-hosted relay). An environment variable pointing anywhere else is
// ignored, so a poisoned environment cannot ship the token away (2026-10-04:
// "secure gate and avoid those env vars").
func AccountHostTrusted(raw string) bool {
	if IsLoopbackURL(raw) {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == "memdoor.ai"
}
