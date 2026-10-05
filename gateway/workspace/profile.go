package workspace

// Profile support following OpenClaw pattern
// Pattern: OpenClaw OPENCLAW_PROFILE environment variable
//
// Profiles allow multiple workspace environments:
// - default: ~/.greg/workspace
// - staging: ~/.greg/workspace-staging
// - production: ~/.greg/workspace-production
//
// Usage:
//   GREG_PROFILE=staging memdoor run
//   GREG_PROFILE=production memdoor doctor

const (
	// ProfileEnvVar is the environment variable for profile selection
	ProfileEnvVar = "GREG_PROFILE"

	// DefaultProfileName is the default profile name
	DefaultProfileName = "default"
)

// ProfileInfo returns information about a profile
type ProfileInfo struct {
	Name      string
	Workspace string
	Config    string
	Active    bool
	Exists    bool
}
