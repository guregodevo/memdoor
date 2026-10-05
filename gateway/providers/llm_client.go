package providers

// truncate caps a string at maxLen runes for safe inclusion in log lines.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
