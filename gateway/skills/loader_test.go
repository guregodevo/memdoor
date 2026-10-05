package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverSkillFiles(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	// Create test skill files
	testFiles := []struct {
		path    string
		isSkill bool
		pattern string
	}{
		{"github.md", true, "root *.md"},
		{"notion.md", true, "root *.md"},
		{"coding-agent/SKILL.md", true, "subdirectory SKILL.md"},
		{"slack/SKILL.md", true, "subdirectory SKILL.md"},
		{"README.md", true, "root *.md"},          // Root .md files are skills
		{"subdirectory/README.md", false, "none"}, // Non-SKILL.md in subdirectory
		{"test.txt", false, "none"},               // Non-markdown file
		{"github/notes.md", false, "none"},        // Non-SKILL.md in subdirectory
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(tmpDir, tf.path)
		dir := filepath.Dir(fullPath)

		// Create directory if needed
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create file
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Discover skill files
	found, err := discoverSkillFiles(tmpDir)
	if err != nil {
		t.Fatalf("discoverSkillFiles failed: %v", err)
	}

	// Verify expected files were found
	expectedCount := 0
	for _, tf := range testFiles {
		if tf.isSkill {
			expectedCount++
		}
	}

	if len(found) != expectedCount {
		t.Errorf("Expected %d skill files, got %d", expectedCount, len(found))
		t.Logf("Found files: %v", found)
	}

	// Verify specific patterns
	foundMap := make(map[string]bool)
	for _, f := range found {
		relPath, _ := filepath.Rel(tmpDir, f)
		foundMap[relPath] = true
	}

	for _, tf := range testFiles {
		if tf.isSkill {
			if !foundMap[tf.path] {
				t.Errorf("Expected skill file not found: %s (pattern: %s)", tf.path, tf.pattern)
			}
		} else {
			if foundMap[tf.path] {
				t.Errorf("Unexpected file found as skill: %s", tf.path)
			}
		}
	}
}

func TestLoadSkillsPrecedence(t *testing.T) {
	// Create temporary directories for different sources
	bundledDir := t.TempDir()
	userDir := t.TempDir()
	workspaceDir := t.TempDir()

	// Create same skill in all three locations with different descriptions
	skillContent := func(source string) string {
		return `---
name: test-skill
description: Test skill from ` + source + `
---

Test content from ` + source
	}

	// Write skill files
	os.WriteFile(filepath.Join(bundledDir, "test-skill.md"), []byte(skillContent("bundled")), 0644)
	os.WriteFile(filepath.Join(userDir, "test-skill.md"), []byte(skillContent("user")), 0644)
	os.WriteFile(filepath.Join(workspaceDir, "test-skill.md"), []byte(skillContent("workspace")), 0644)

	// Load skills
	skills, _, err := LoadSkills(LoadOptions{
		BundledDir:   bundledDir,
		UserDir:      userDir,
		WorkspaceDir: workspaceDir,
		Verbose:      false,
	})

	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	// Should only have one skill (workspace takes precedence)
	if len(skills) != 1 {
		t.Fatalf("Expected 1 skill, got %d", len(skills))
	}

	// Verify it's the workspace version
	skill := skills[0]
	if skill.Source != "workspace" {
		t.Errorf("Expected source 'workspace', got '%s'", skill.Source)
	}

	if skill.Description != "Test skill from workspace" {
		t.Errorf("Expected workspace description, got '%s'", skill.Description)
	}
}

func TestLoadSkillsMultipleSources(t *testing.T) {
	// Create temporary directories
	bundledDir := t.TempDir()
	userDir := t.TempDir()

	// Create different skills in each source
	bundledSkill := `---
name: bundled-skill
description: Bundled skill
---

Bundled content`

	userSkill := `---
name: user-skill
description: User skill
---

User content`

	os.WriteFile(filepath.Join(bundledDir, "bundled-skill.md"), []byte(bundledSkill), 0644)
	os.WriteFile(filepath.Join(userDir, "user-skill.md"), []byte(userSkill), 0644)

	// Load skills
	skills, _, err := LoadSkills(LoadOptions{
		BundledDir: bundledDir,
		UserDir:    userDir,
		Verbose:    false,
	})

	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	// Should have both skills
	if len(skills) != 2 {
		t.Fatalf("Expected 2 skills, got %d", len(skills))
	}

	// Verify both are present
	foundBundled := false
	foundUser := false
	for _, skill := range skills {
		if skill.Name == "bundled-skill" {
			foundBundled = true
			if skill.Source != "bundled" {
				t.Errorf("bundled-skill has wrong source: %s", skill.Source)
			}
		}
		if skill.Name == "user-skill" {
			foundUser = true
			if skill.Source != "user" {
				t.Errorf("user-skill has wrong source: %s", skill.Source)
			}
		}
	}

	if !foundBundled {
		t.Error("bundled-skill not found")
	}
	if !foundUser {
		t.Error("user-skill not found")
	}
}

func TestFilterEligibleSkills(t *testing.T) {
	// Create test skills with various requirements
	skills := []Skill{
		{
			Name:        "no-requirements",
			Description: "Skill with no requirements",
			Metadata:    nil,
		},
		{
			Name:        "always-load",
			Description: "Skill that always loads",
			Metadata: &SkillMetadata{
				Always: true,
			},
		},
		{
			Name:        "darwin-only",
			Description: "macOS only skill",
			Metadata: &SkillMetadata{
				OS: []string{"darwin"},
			},
		},
		{
			Name:        "linux-only",
			Description: "Linux only skill",
			Metadata: &SkillMetadata{
				OS: []string{"linux"},
			},
		},
		{
			Name:        "requires-git",
			Description: "Requires git binary",
			Metadata: &SkillMetadata{
				Requires: &SkillRequirements{
					Bins: []string{"git"},
				},
			},
		},
		{
			Name:        "requires-missing-bin",
			Description: "Requires non-existent binary",
			Metadata: &SkillMetadata{
				Requires: &SkillRequirements{
					Bins: []string{"nonexistent-binary-xyz"},
				},
			},
		},
		{
			Name:        "requires-env",
			Description: "Requires TEST_ENV variable",
			Metadata: &SkillMetadata{
				Requires: &SkillRequirements{
					Env: []string{"TEST_ENV"},
				},
			},
		},
	}

	// Create eligibility context
	ctx := EligibilityContext{
		OS: runtime.GOOS,
		Env: map[string]string{
			"TEST_ENV": "test-value",
			"PATH":     os.Getenv("PATH"),
		},
		BinPath: filepath.SplitList(os.Getenv("PATH")),
	}

	// Filter eligible skills
	eligible, diags := FilterEligibleSkills(skills, ctx)

	// Log diagnostics
	t.Logf("Diagnostics: %d", len(diags))
	for _, diag := range diags {
		t.Logf("  [%s] %s", diag.Level, diag.Message)
	}

	// Verify expected eligible skills
	eligibleMap := make(map[string]bool)
	for _, skill := range eligible {
		eligibleMap[skill.Name] = true
	}

	// These should always be eligible
	expectedEligible := []string{
		"no-requirements",
		"always-load",
		"requires-git", // git is typically available
		"requires-env", // TEST_ENV is in context
	}

	// Add OS-specific skill
	if runtime.GOOS == "darwin" {
		expectedEligible = append(expectedEligible, "darwin-only")
	} else if runtime.GOOS == "linux" {
		expectedEligible = append(expectedEligible, "linux-only")
	}

	for _, name := range expectedEligible {
		if !eligibleMap[name] {
			t.Errorf("Expected skill '%s' to be eligible", name)
		}
	}

	// These should NOT be eligible
	if eligibleMap["requires-missing-bin"] {
		t.Error("Skill requiring missing binary should not be eligible")
	}

	t.Logf("Eligible skills: %d/%d", len(eligible), len(skills))
}

func TestIsBinaryAvailable(t *testing.T) {
	binPath := filepath.SplitList(os.Getenv("PATH"))

	// Test with common binaries that should exist
	commonBins := []string{"ls", "cat", "echo"}

	for _, bin := range commonBins {
		if runtime.GOOS == "windows" {
			// Windows uses different binary names
			continue
		}

		available := isBinaryAvailable(bin, binPath)
		if !available {
			t.Logf("Warning: Common binary '%s' not found in PATH", bin)
		}
	}

	// Test with non-existent binary
	if isBinaryAvailable("nonexistent-binary-xyz-123", binPath) {
		t.Error("Non-existent binary should not be available")
	}
}
