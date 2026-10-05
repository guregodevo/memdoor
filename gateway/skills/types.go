package skills

// Skill represents a loaded skill with metadata
type Skill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	FilePath    string          `json:"file_path"`  // Absolute path to SKILL.md
	BaseDir     string          `json:"base_dir"`   // Parent directory (for references/)
	Source      string          `json:"source"`     // "bundled", "user", "workspace", "plugin"
	Content     string          `json:"content"`    // Full markdown content
	Metadata    *SkillMetadata  `json:"metadata"`   // Optional OpenClaw metadata
	Invocation  *InvocationOpts `json:"invocation"` // Invocation policy
}

// SkillMetadata contains OpenClaw-specific metadata
type SkillMetadata struct {
	Emoji      string             `json:"emoji,omitempty"`
	Homepage   string             `json:"homepage,omitempty"`
	SkillKey   string             `json:"skill_key,omitempty"`   // Override name for config lookup
	PrimaryEnv string             `json:"primary_env,omitempty"` // Primary env var
	Always     bool               `json:"always,omitempty"`      // Load unconditionally
	OS         []string           `json:"os,omitempty"`          // OS restrictions
	Requires   *SkillRequirements `json:"requires,omitempty"`
	Install    []InstallSpec      `json:"install,omitempty"`
}

// SkillRequirements defines what the skill needs to run
type SkillRequirements struct {
	Bins    []string `json:"bins,omitempty"`     // Required binaries (ALL must exist)
	AnyBins []string `json:"any_bins,omitempty"` // Required binaries (ANY must exist)
	Env     []string `json:"env,omitempty"`      // Required env vars
	Config  []string `json:"config,omitempty"`   // Required config paths
}

// InstallSpec describes how to install a skill's dependencies
type InstallSpec struct {
	ID      string   `json:"id"`                // Unique identifier
	Kind    string   `json:"kind"`              // "brew", "apt", "node", "go", "download"
	Label   string   `json:"label,omitempty"`   // Human-readable label
	Formula string   `json:"formula,omitempty"` // Homebrew formula
	Package string   `json:"package,omitempty"` // APT package name
	Bins    []string `json:"bins,omitempty"`    // Binaries provided by this install
}

// InvocationOpts controls how the skill can be invoked
type InvocationOpts struct {
	UserInvocable          bool `json:"user_invocable"`           // Can be invoked via /skill command
	DisableModelInvocation bool `json:"disable_model_invocation"` // Exclude from system prompt
}

// SkillFrontmatter represents the YAML frontmatter in a skill file
type SkillFrontmatter struct {
	Name               string                 `yaml:"name"`
	Description        string                 `yaml:"description"`
	Homepage           string                 `yaml:"homepage,omitempty"`
	UserInvocable      *bool                  `yaml:"user-invocable,omitempty"`
	DisableModelInvoke *bool                  `yaml:"disable-model-invocation,omitempty"`
	Metadata           map[string]interface{} `yaml:"metadata,omitempty"`
}

// EligibilityContext provides context for skill filtering
type EligibilityContext struct {
	OS      string            // Current OS ("darwin", "linux", "windows")
	Env     map[string]string // Available environment variables
	BinPath []string          // Directories in PATH
}

// SkillSnapshot represents skills serialized for remote execution
type SkillSnapshot struct {
	Prompt         string   `json:"prompt"`          // Pre-rendered prompt
	Skills         []string `json:"skills"`          // Skill names
	ResolvedSkills []Skill  `json:"resolved_skills"` // Full skill objects
	Version        int      `json:"version"`         // Snapshot version
}

// LoadOptions configures skill loading
type LoadOptions struct {
	BundledDir   string   // Path to bundled skills
	UserDir      string   // Path to user skills (~/.greg/skills)
	WorkspaceDir string   // Path to workspace skills
	PluginDirs   []string // Paths to plugin skill directories
	Verbose      bool     // Enable verbose logging
}

// Diagnostic represents a warning or error during skill loading
type Diagnostic struct {
	Level     string `json:"level"` // "info", "warning", "error"
	Message   string `json:"message"`
	FilePath  string `json:"file_path,omitempty"`
	SkillName string `json:"skill_name,omitempty"`
}
