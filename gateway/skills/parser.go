package skills

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// frontmatterDelimiter matches YAML frontmatter delimiters (--- or +++)
	frontmatterDelimiter = regexp.MustCompile(`^(---|\\+\\+\\+)\s*$`)
)

// ParseSkillFile parses a skill markdown file with YAML frontmatter
func ParseSkillFile(filePath string) (*Skill, []Diagnostic, error) {
	var diagnostics []Diagnostic

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, diagnostics, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse frontmatter and content
	frontmatter, markdown, err := extractFrontmatter(string(content))
	if err != nil {
		return nil, diagnostics, fmt.Errorf("failed to extract frontmatter: %w", err)
	}

	// Parse YAML frontmatter
	var fm SkillFrontmatter
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, diagnostics, fmt.Errorf("failed to parse frontmatter YAML: %w", err)
		}
	}

	// Validate required fields
	if fm.Name == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Level:    "error",
			Message:  "skill name is required in frontmatter",
			FilePath: filePath,
		})
		return nil, diagnostics, fmt.Errorf("skill name is required")
	}

	if fm.Description == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Level:    "error",
			Message:  "skill description is required in frontmatter",
			FilePath: filePath,
		})
		return nil, diagnostics, fmt.Errorf("skill description is required")
	}

	// Validate name format
	if !isValidSkillName(fm.Name) {
		diagnostics = append(diagnostics, Diagnostic{
			Level:     "error",
			Message:   fmt.Sprintf("invalid skill name format: %s (must be lowercase a-z, 0-9, hyphens only)", fm.Name),
			FilePath:  filePath,
			SkillName: fm.Name,
		})
		return nil, diagnostics, fmt.Errorf("invalid skill name: %s", fm.Name)
	}

	// Build skill object
	skill := &Skill{
		Name:        fm.Name,
		Description: fm.Description,
		FilePath:    filePath,
		BaseDir:     getBaseDir(filePath),
		Content:     markdown,
	}

	// Parse invocation options
	if fm.UserInvocable != nil || fm.DisableModelInvoke != nil {
		skill.Invocation = &InvocationOpts{
			UserInvocable:          fm.UserInvocable == nil || *fm.UserInvocable, // Default true
			DisableModelInvocation: fm.DisableModelInvoke != nil && *fm.DisableModelInvoke,
		}
	}

	// Parse OpenClaw metadata if present
	if fm.Metadata != nil {
		if openclawData, ok := fm.Metadata["openclaw"].(map[string]interface{}); ok {
			metadata, diags := parseOpenClawMetadata(openclawData)
			diagnostics = append(diagnostics, diags...)
			skill.Metadata = metadata
		}
	}

	return skill, diagnostics, nil
}

// extractFrontmatter separates YAML frontmatter from markdown content
func extractFrontmatter(content string) (frontmatter, markdown string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	var inFrontmatter bool
	var frontmatterLines []string
	var contentLines []string
	delimiterCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter delimiter
		if frontmatterDelimiter.MatchString(line) {
			delimiterCount++
			if delimiterCount == 1 {
				// Start of frontmatter
				inFrontmatter = true
				continue
			} else if delimiterCount == 2 {
				// End of frontmatter
				inFrontmatter = false
				continue
			}
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		} else if delimiterCount >= 2 {
			// After frontmatter
			contentLines = append(contentLines, line)
		} else if delimiterCount == 0 {
			// No frontmatter, treat as content
			contentLines = append(contentLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	frontmatter = strings.Join(frontmatterLines, "\n")
	markdown = strings.Join(contentLines, "\n")

	return frontmatter, markdown, nil
}

// parseOpenClawMetadata parses the openclaw metadata section
func parseOpenClawMetadata(data map[string]interface{}) (*SkillMetadata, []Diagnostic) {
	var diagnostics []Diagnostic
	metadata := &SkillMetadata{}

	// Parse simple fields
	if emoji, ok := data["emoji"].(string); ok {
		metadata.Emoji = emoji
	}
	if homepage, ok := data["homepage"].(string); ok {
		metadata.Homepage = homepage
	}
	if skillKey, ok := data["skillKey"].(string); ok {
		metadata.SkillKey = skillKey
	}
	if primaryEnv, ok := data["primaryEnv"].(string); ok {
		metadata.PrimaryEnv = primaryEnv
	}
	if always, ok := data["always"].(bool); ok {
		metadata.Always = always
	}

	// Parse OS array
	if osData, ok := data["os"].([]interface{}); ok {
		for _, os := range osData {
			if osStr, ok := os.(string); ok {
				metadata.OS = append(metadata.OS, osStr)
			}
		}
	}

	// Parse requires
	if requiresData, ok := data["requires"].(map[string]interface{}); ok {
		metadata.Requires = &SkillRequirements{}

		if bins, ok := requiresData["bins"].([]interface{}); ok {
			for _, bin := range bins {
				if binStr, ok := bin.(string); ok {
					metadata.Requires.Bins = append(metadata.Requires.Bins, binStr)
				}
			}
		}

		if anyBins, ok := requiresData["anyBins"].([]interface{}); ok {
			for _, bin := range anyBins {
				if binStr, ok := bin.(string); ok {
					metadata.Requires.AnyBins = append(metadata.Requires.AnyBins, binStr)
				}
			}
		}

		if env, ok := requiresData["env"].([]interface{}); ok {
			for _, envVar := range env {
				if envStr, ok := envVar.(string); ok {
					metadata.Requires.Env = append(metadata.Requires.Env, envStr)
				}
			}
		}

		if config, ok := requiresData["config"].([]interface{}); ok {
			for _, cfg := range config {
				if cfgStr, ok := cfg.(string); ok {
					metadata.Requires.Config = append(metadata.Requires.Config, cfgStr)
				}
			}
		}
	}

	// Parse install specs
	if installData, ok := data["install"].([]interface{}); ok {
		for _, spec := range installData {
			if specMap, ok := spec.(map[string]interface{}); ok {
				installSpec := parseInstallSpec(specMap)
				metadata.Install = append(metadata.Install, installSpec)
			}
		}
	}

	return metadata, diagnostics
}

// parseInstallSpec parses an install specification
func parseInstallSpec(data map[string]interface{}) InstallSpec {
	spec := InstallSpec{}

	if id, ok := data["id"].(string); ok {
		spec.ID = id
	}
	if kind, ok := data["kind"].(string); ok {
		spec.Kind = kind
	}
	if label, ok := data["label"].(string); ok {
		spec.Label = label
	}
	if formula, ok := data["formula"].(string); ok {
		spec.Formula = formula
	}
	if pkg, ok := data["package"].(string); ok {
		spec.Package = pkg
	}

	if bins, ok := data["bins"].([]interface{}); ok {
		for _, bin := range bins {
			if binStr, ok := bin.(string); ok {
				spec.Bins = append(spec.Bins, binStr)
			}
		}
	}

	return spec
}

// isValidSkillName validates skill name format
// Rules: lowercase a-z, 0-9, hyphens only, max 64 chars, no leading/trailing hyphens
func isValidSkillName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}

	// Check for invalid characters
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}

	// No leading/trailing hyphens
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}

	// No consecutive hyphens
	if strings.Contains(name, "--") {
		return false
	}

	return true
}

// getBaseDir returns the parent directory of a file path
func getBaseDir(filePath string) string {
	// Remove filename to get directory
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return "."
	}
	return filePath[:lastSlash]
}
