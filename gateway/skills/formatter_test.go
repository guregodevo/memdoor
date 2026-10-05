package skills

import (
	"strings"
	"testing"
)

func TestFormatSkillsPrompt(t *testing.T) {
	tests := []struct {
		name        string
		skills      []Skill
		contains    []string
		notContains []string
	}{
		{
			name: "single simple skill",
			skills: []Skill{
				{
					Name:        "test-skill",
					Description: "A test skill",
					Content:     "This is the skill content.",
				},
			},
			contains: []string{
				"<skills>",
				"</skills>",
				"<skill name=\"test-skill\">",
				"</skill>",
				"<!-- A test skill -->",
				"This is the skill content.",
			},
		},
		{
			name: "multiple skills",
			skills: []Skill{
				{
					Name:        "skill1",
					Description: "First skill",
					Content:     "Content 1",
				},
				{
					Name:        "skill2",
					Description: "Second skill",
					Content:     "Content 2",
				},
			},
			contains: []string{
				"<skill name=\"skill1\">",
				"<skill name=\"skill2\">",
				"Content 1",
				"Content 2",
			},
		},
		{
			name: "skill with metadata",
			skills: []Skill{
				{
					Name:        "github",
					Description: "GitHub integration",
					Content:     "GitHub skill content",
					Metadata: &SkillMetadata{
						Emoji:    "🐙",
						Homepage: "https://github.com",
					},
				},
			},
			contains: []string{
				"<skill name=\"github\">",
				"<!-- GitHub integration -->",
				"<!-- Homepage: https://github.com -->",
				"<!-- Emoji: 🐙 -->",
				"GitHub skill content",
			},
		},
		{
			name: "skill with disabled model invocation",
			skills: []Skill{
				{
					Name:        "disabled-skill",
					Description: "Should not appear",
					Content:     "Disabled content",
					Invocation: &InvocationOpts{
						DisableModelInvocation: true,
					},
				},
				{
					Name:        "enabled-skill",
					Description: "Should appear",
					Content:     "Enabled content",
				},
			},
			contains: []string{
				"<skill name=\"enabled-skill\">",
				"Enabled content",
			},
			notContains: []string{
				"disabled-skill",
				"Disabled content",
			},
		},
		{
			name:     "empty skills list",
			skills:   []Skill{},
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSkillsPrompt(tt.skills)

			// Check for expected strings
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("Expected output to contain '%s', but it doesn't.\nOutput:\n%s", str, result)
				}
			}

			// Check for strings that should NOT be present
			for _, str := range tt.notContains {
				if strings.Contains(result, str) {
					t.Errorf("Expected output to NOT contain '%s', but it does.\nOutput:\n%s", str, result)
				}
			}
		})
	}
}

func TestFormatSkillXML(t *testing.T) {
	skill := Skill{
		Name:        "test-skill",
		Description: "Test description",
		Content:     "Line 1\nLine 2\nLine 3",
	}

	result := formatSkillXML(skill)

	// Verify structure
	if !strings.Contains(result, "<skill name=\"test-skill\">") {
		t.Error("Missing skill opening tag")
	}
	if !strings.Contains(result, "</skill>") {
		t.Error("Missing skill closing tag")
	}
	if !strings.Contains(result, "<!-- Test description -->") {
		t.Error("Missing description comment")
	}

	// Verify content is indented
	lines := strings.Split(result, "\n")
	contentLines := 0
	for _, line := range lines {
		if strings.Contains(line, "Line") {
			contentLines++
			// Content should be indented with 4 spaces
			if !strings.HasPrefix(line, "    ") {
				t.Errorf("Content line not properly indented: '%s'", line)
			}
		}
	}

	if contentLines != 3 {
		t.Errorf("Expected 3 content lines, got %d", contentLines)
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"text with & ampersand", "text with &amp; ampersand"},
		{"text with < less than", "text with &lt; less than"},
		{"text with > greater than", "text with &gt; greater than"},
		{"text with \"quotes\"", "text with &quot;quotes&quot;"},
		{"text with 'apostrophes'", "text with &apos;apostrophes&apos;"},
		{"<tag>content</tag>", "&lt;tag&gt;content&lt;/tag&gt;"},
	}

	for _, tt := range tests {
		result := escapeXML(tt.input)
		if result != tt.expected {
			t.Errorf("escapeXML(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestEscapeXMLComment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"text with -- double dash", "text with - - double dash"},
		{"multiple -- instances --", "multiple - - instances - -"},
		{"---", "- - -"},
	}

	for _, tt := range tests {
		result := escapeXMLComment(tt.input)
		if result != tt.expected {
			t.Errorf("escapeXMLComment(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestIndentContent(t *testing.T) {
	input := "Line 1\nLine 2\n\nLine 4"
	expected := "    Line 1\n    Line 2\n\n    Line 4"

	result := indentContent(input, "    ")

	if result != expected {
		t.Errorf("indentContent failed.\nExpected:\n%s\nGot:\n%s", expected, result)
	}
}
