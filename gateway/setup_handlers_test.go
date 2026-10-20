package gateway

import (
	"strings"
	"testing"

	"memdoor/pkg/auth"
)

// A username nobody typed must never be the reason setup fails (onboarding
// walk, 2026-09-27: `--admin-email s@example.com` died on "username must be
// 3-30 characters"; a Mac's first run, 2026-10-20: first.last@localhost
// died on "username must be alphanumeric with hyphens or underscores").
// Setup derives it the way every sign-up does, through auth.UsernameFromEmail.
func TestSetupUsernameAlwaysValidates(t *testing.T) {
	for _, tc := range []struct{ email, want string }{
		{"first.last@localhost", "first-last"},
		{"Jane.Doe+dev@example.com", "jane-doe-dev"},
		{"stranger@example.com", "stranger"},
		{"s@example.com", "user-s"},
		{"a@b", "user-a"},
		{strings.Repeat("n", 40) + "@example.com", strings.Repeat("n", 30)},
	} {
		got := auth.UsernameFromEmail(tc.email)
		if got != tc.want {
			t.Errorf("UsernameFromEmail(%q) = %q, want %q", tc.email, got, tc.want)
		}
		if len(got) < 3 || len(got) > 30 {
			t.Errorf("UsernameFromEmail(%q) = %q, which validation refuses", tc.email, got)
		}
		for _, ch := range got {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
				t.Errorf("UsernameFromEmail(%q) = %q, which validation refuses", tc.email, got)
			}
		}
	}
}
