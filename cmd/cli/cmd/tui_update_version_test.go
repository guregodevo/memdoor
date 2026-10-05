package cmd

import "testing"

// A build ahead of the release is not told to update to it (2026-09-28).
func TestVersionNewer(t *testing.T) {
	for _, c := range []struct {
		remote, local string
		want          bool
	}{
		{"v1.330.0-wiki", "v1.330.0-wiki-3-gcbc36540", false},
		{"v1.330.0-wiki", "v1.330.0-wiki-dirty", false},
		{"v1.331.0-wiki", "v1.330.0-wiki-3-gcbc36540", true},
		{"v1.330.0-wiki", "v1.331.0-wiki", false},
		{"v2.0.0", "v1.999.9", true},
		{"v1.330.0-wiki", "v1.330.0-wiki", false},
	} {
		if got := versionNewer(c.remote, c.local); got != c.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", c.remote, c.local, got, c.want)
		}
	}
}
