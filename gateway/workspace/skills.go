package workspace

// Skills directory structure following OpenClaw pattern
// Pattern: OpenClaw src/skills/ directory structure
//
// Skills loading order (workspace wins on name conflict):
// 1. Bundled skills (shipped with install) - not implemented yet
// 2. Managed skills: ~/.memdoor/skills - global user skills
// 3. Workspace skills: <workspace>/skills - per-workspace override

const (
	// SkillsDirName is the skills directory name
	SkillsDirName = "skills"

	// SkillFileName is the skill definition file
	SkillFileName = "SKILL.md"
)

// Skill represents a skill definition
type Skill struct {
	Name        string   // Skill name (directory name)
	Path        string   // Full path to skill directory
	Description string   // From SKILL.md
	Files       []string // Additional skill files
}
