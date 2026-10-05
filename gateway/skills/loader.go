package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSkills loads skills from configured directories
func LoadSkills(opts LoadOptions) ([]Skill, []Diagnostic, error) {
	var allSkills []Skill
	var diagnostics []Diagnostic

	// Load skills from each source in precedence order
	sources := []struct {
		dir    string
		source string
	}{
		{opts.WorkspaceDir, "workspace"},
		{opts.UserDir, "user"},
		{opts.BundledDir, "bundled"},
	}

	// Add plugin directories
	for _, pluginDir := range opts.PluginDirs {
		sources = append(sources, struct {
			dir    string
			source string
		}{pluginDir, "plugin"})
	}

	// Track loaded skills by name for deduplication
	loadedNames := make(map[string]bool)

	// Load from each source
	for _, src := range sources {
		if src.dir == "" {
			continue
		}

		// Check if directory exists
		if _, err := os.Stat(src.dir); os.IsNotExist(err) {
			if opts.Verbose {
				diagnostics = append(diagnostics, Diagnostic{
					Level:   "info",
					Message: fmt.Sprintf("skill directory does not exist: %s", src.dir),
				})
			}
			continue
		}

		// Discover skill files
		skillFiles, err := discoverSkillFiles(src.dir)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Level:   "warning",
				Message: fmt.Sprintf("failed to discover skills in %s: %v", src.dir, err),
			})
			continue
		}

		// Parse each skill file
		for _, filePath := range skillFiles {
			skill, skillDiags, err := ParseSkillFile(filePath)

			// Collect diagnostics
			diagnostics = append(diagnostics, skillDiags...)

			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{
					Level:    "error",
					Message:  fmt.Sprintf("failed to parse skill file %s: %v", filePath, err),
					FilePath: filePath,
				})
				continue
			}

			// Skip if already loaded (precedence)
			if loadedNames[skill.Name] {
				if opts.Verbose {
					diagnostics = append(diagnostics, Diagnostic{
						Level:     "info",
						Message:   fmt.Sprintf("skill '%s' already loaded from higher precedence source, skipping", skill.Name),
						FilePath:  filePath,
						SkillName: skill.Name,
					})
				}
				continue
			}

			// Set source
			skill.Source = src.source

			// Add to loaded skills
			allSkills = append(allSkills, *skill)
			loadedNames[skill.Name] = true

			if opts.Verbose {
				diagnostics = append(diagnostics, Diagnostic{
					Level:     "info",
					Message:   fmt.Sprintf("loaded skill '%s' from %s", skill.Name, src.source),
					FilePath:  filePath,
					SkillName: skill.Name,
				})
			}
		}
	}

	return allSkills, diagnostics, nil
}

// discoverSkillFiles finds all SKILL.md files in a directory
func discoverSkillFiles(dir string) ([]string, error) {
	var skillFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file matches skill patterns
		if isSkillFile(path, dir) {
			skillFiles = append(skillFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return skillFiles, nil
}

// isSkillFile checks if a file matches skill naming patterns
func isSkillFile(path, baseDir string) bool {
	filename := filepath.Base(path)
	relPath, _ := filepath.Rel(baseDir, path)

	// Pattern 1: SKILL.md in subdirectories (e.g., github/SKILL.md)
	if filename == "SKILL.md" && filepath.Dir(relPath) != "." {
		return true
	}

	// Pattern 2: *.md in root directory (e.g., github.md)
	// Must be in the root directory and end with .md
	if filepath.Dir(relPath) == "." && strings.HasSuffix(filename, ".md") {
		return true
	}

	return false
}

// FilterEligibleSkills filters skills based on system context
func FilterEligibleSkills(skills []Skill, ctx EligibilityContext) ([]Skill, []Diagnostic) {
	var eligible []Skill
	var diagnostics []Diagnostic

	for _, skill := range skills {
		if skill.Metadata == nil {
			// No metadata, skill is eligible
			eligible = append(eligible, skill)
			continue
		}

		// Check if skill is always loaded
		if skill.Metadata.Always {
			eligible = append(eligible, skill)
			continue
		}

		// Check OS compatibility
		if len(skill.Metadata.OS) > 0 {
			osMatch := false
			for _, os := range skill.Metadata.OS {
				if os == ctx.OS {
					osMatch = true
					break
				}
			}
			if !osMatch {
				diagnostics = append(diagnostics, Diagnostic{
					Level:     "info",
					Message:   fmt.Sprintf("skill '%s' not compatible with OS '%s'", skill.Name, ctx.OS),
					SkillName: skill.Name,
				})
				continue
			}
		}

		// Check requirements
		if skill.Metadata.Requires != nil {
			meetsRequirements := true

			// Check required binaries (ALL must exist)
			if len(skill.Metadata.Requires.Bins) > 0 {
				for _, bin := range skill.Metadata.Requires.Bins {
					if !isBinaryAvailable(bin, ctx.BinPath) {
						meetsRequirements = false
						diagnostics = append(diagnostics, Diagnostic{
							Level:     "info",
							Message:   fmt.Sprintf("skill '%s' requires binary '%s' which is not available", skill.Name, bin),
							SkillName: skill.Name,
						})
						break
					}
				}
			}

			// Check anyBins (ANY must exist)
			if len(skill.Metadata.Requires.AnyBins) > 0 {
				anyBinFound := false
				for _, bin := range skill.Metadata.Requires.AnyBins {
					if isBinaryAvailable(bin, ctx.BinPath) {
						anyBinFound = true
						break
					}
				}
				if !anyBinFound {
					meetsRequirements = false
					diagnostics = append(diagnostics, Diagnostic{
						Level:     "info",
						Message:   fmt.Sprintf("skill '%s' requires at least one of %v binaries", skill.Name, skill.Metadata.Requires.AnyBins),
						SkillName: skill.Name,
					})
				}
			}

			// Check environment variables
			if len(skill.Metadata.Requires.Env) > 0 {
				for _, envVar := range skill.Metadata.Requires.Env {
					if _, exists := ctx.Env[envVar]; !exists {
						meetsRequirements = false
						diagnostics = append(diagnostics, Diagnostic{
							Level:     "info",
							Message:   fmt.Sprintf("skill '%s' requires environment variable '%s'", skill.Name, envVar),
							SkillName: skill.Name,
						})
						break
					}
				}
			}

			if !meetsRequirements {
				continue
			}
		}

		// Skill is eligible
		eligible = append(eligible, skill)
	}

	return eligible, diagnostics
}

// isBinaryAvailable checks if a binary exists in PATH
func isBinaryAvailable(binName string, binPath []string) bool {
	for _, dir := range binPath {
		binPath := filepath.Join(dir, binName)
		if _, err := os.Stat(binPath); err == nil {
			return true
		}
	}
	return false
}
