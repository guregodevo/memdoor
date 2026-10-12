package gateway

import (
	"strings"
	"testing"
)

// A username nobody typed must never be the reason setup fails (onboarding
// walk, 2026-09-27: `--admin-email s@example.com` died on "username must be
// 3-30 characters").
func TestUsernameFromEmailAlwaysValidates(t *testing.T) {
	for _, tc := range []struct{ email, want string }{
		{"s@example.com", "s-example"},
		{"ab@x.io", "ab-x"},
		{"stranger@example.com", "stranger"},
		{"a@b", "a-b"},
		{"s@", "s00"},
		{strings.Repeat("n", 40) + "@example.com", strings.Repeat("n", 30)},
		// first.last@ died on "username must be alphanumeric with hyphens or
		// underscores" (2026-10-12): the dot, a plus, anything else the
		// validator refuses becomes a dash.
		{"gregory.desvaux@gmail.com", "gregory-desvaux"},
		{"first.last+tag@example.com", "first-last-tag"},
		{"a.b@example.com", "a-b"},
		{"..@example.com", "example"},
		{"é@example.com", "example"},
		{strings.Repeat("n", 29) + ".x@example.com", strings.Repeat("n", 29)},
	} {
		got := usernameFromEmail(tc.email)
		if got != tc.want {
			t.Errorf("usernameFromEmail(%q) = %q, want %q", tc.email, got, tc.want)
		}
		if len(got) < 3 || len(got) > 30 {
			t.Errorf("usernameFromEmail(%q) = %q, which validation refuses", tc.email, got)
		}
		for _, ch := range got {
			ok := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_'
			if !ok {
				t.Errorf("usernameFromEmail(%q) = %q, has %q which validation refuses", tc.email, got, ch)
			}
		}
	}
}
