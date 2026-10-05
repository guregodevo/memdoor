package secrets

import "regexp"

// Redacted replaces a secret in shared text.
const Redacted = "[redacted]"

// textSecrets are the shapes of a secret in free text — a conversation about
// to be shared (/share), where a key pasted to the agent or printed by a
// command must not travel. Each match is replaced whole except where a group
// keeps the name that announced it ("password: …" stays "password: [redacted]").
var textSecrets = []*regexp.Regexp{
	// Private key blocks.
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
	// Vendor tokens by their prefix (the same vendors AuditValue knows).
	regexp.MustCompile(`\b(?:sk-or-v1-|sk-ant-|sk-proj-|sk-)[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`\b(?:xox[bpas]|xapp)-[A-Za-z0-9\-]{10,}`),
	regexp.MustCompile(`\b(?:ghp|gho|ghs|ghu|ghr)_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{30,}`),
	regexp.MustCompile(`\b(?:sk|rk|pk)_(?:live|test)_[0-9A-Za-z]{16,}`),
	regexp.MustCompile(`\bre_[A-Za-z0-9]{8,}_[A-Za-z0-9]{8,}`),
	// JSON web tokens.
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}`),
}

// namedSecret is a value announced by its name: `password=…`, `API_KEY: …`,
// `Authorization: Bearer …`. The name stays; the value goes.
var namedSecret = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api[_-]?key|access[_-]?key|auth[_-]?token|client[_-]?secret|private[_-]?key|authorization)["']?\s*[:=]\s*["']?(?:bearer\s+)?)([^\s"'` + "`" + `,;]{8,})`)

// RedactText returns s with every secret-shaped value replaced by Redacted.
func RedactText(s string) string {
	for _, re := range textSecrets {
		s = re.ReplaceAllString(s, Redacted)
	}
	return namedSecret.ReplaceAllString(s, "${1}"+Redacted)
}
