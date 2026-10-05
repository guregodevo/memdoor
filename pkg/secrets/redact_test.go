package secrets

import (
	"strings"
	"testing"
)

// A conversation about to be shared loses its secrets and keeps its words.
func TestRedactText(t *testing.T) {
	for in, gone := range map[string]string{
		"export OPEN_ROUTER_API_KEY=sk-or-v1-0123456789abcdef0123456789abcdef":                             "sk-or-v1-0123456789abcdef",
		"the key is sk-ant-api03-AbCdEfGhIjKlMnOpQrStUv now":                                               "sk-ant-api03-AbCdEfGhIjKlMnOp",
		"token ghp_0123456789abcdefghijABCDEFGHIJ012345 in the log":                                        "ghp_0123456789abcdefghij",
		"AWS AKIAIOSFODNN7EXAMPLE was leaked":                                                              "AKIAIOSFODNN7EXAMPLE",
		"stripe sk_live_51H8abcdefghijklmnop":                                                              "sk_live_51H8abcdefghijklmnop",
		"Authorization: Bearer abcdefghijklmnopqrstuvwxyz0123":                                             "abcdefghijklmnopqrstuvwxyz0123",
		`{"password": "hunter2hunter2"}`:                                                                   "hunter2hunter2",
		"DB_PASSWORD=correcthorsebatterystaple":                                                            "correcthorsebatterystaple",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U": "eyJhbGciOiJIUzI1NiJ9",
		"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\n-----END OPENSSH PRIVATE KEY-----": "b3BlbnNzaC1rZXktdjEAAAAA",
	} {
		out := RedactText(in)
		if strings.Contains(out, gone) || !strings.Contains(out, Redacted) {
			t.Errorf("%q\n  -> %q", in, out)
		}
	}
	// The name that announced a secret stays, so the reader knows what went.
	if got := RedactText("password: hunter2hunter2"); got != "password: "+Redacted {
		t.Errorf("got %q", got)
	}
	// Ordinary prose and code are left alone.
	for _, keep := range []string{
		"the token count went from 57,423 to 29,458",
		"func (s *Store) Secret() string { return s.secret }",
		"see skill: review the diff",
		"pass the key name, not the key",
	} {
		if got := RedactText(keep); got != keep {
			t.Errorf("changed plain text: %q -> %q", keep, got)
		}
	}
}
