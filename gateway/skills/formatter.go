package skills

import (
	"fmt"
	"strings"
)

// FormatSkillsPrompt formats skills into XML for system prompt injection
func FormatSkillsPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("<skills>\n")

	for _, skill := range skills {
		// Skip skills that disable model invocation
		if skill.Invocation != nil && skill.Invocation.DisableModelInvocation {
			continue
		}

		// Format skill as XML
		sb.WriteString(formatSkillXML(skill))
	}

	sb.WriteString("</skills>")

	return sb.String()
}

// formatSkillXML formats a single skill as XML
func formatSkillXML(skill Skill) string {
	var sb strings.Builder

	// Open skill tag with name attribute
	sb.WriteString(fmt.Sprintf("  <skill name=\"%s\">\n", escapeXML(skill.Name)))

	// Add description as XML comment
	if skill.Description != "" {
		sb.WriteString(fmt.Sprintf("    <!-- %s -->\n", escapeXMLComment(skill.Description)))
	}

	// Add metadata if present
	if skill.Metadata != nil {
		if skill.Metadata.Homepage != "" {
			sb.WriteString(fmt.Sprintf("    <!-- Homepage: %s -->\n", escapeXMLComment(skill.Metadata.Homepage)))
		}
		if skill.Metadata.Emoji != "" {
			sb.WriteString(fmt.Sprintf("    <!-- Emoji: %s -->\n", skill.Metadata.Emoji))
		}
	}

	// Add skill content (markdown)
	content := strings.TrimSpace(skill.Content)
	if content != "" {
		// Indent content
		indentedContent := indentContent(content, "    ")
		sb.WriteString(indentedContent)
		sb.WriteString("\n")
	}

	// Close skill tag
	sb.WriteString("  </skill>\n")

	return sb.String()
}

// indentContent indents each line of content
func indentContent(content string, indent string) string {
	lines := strings.Split(content, "\n")
	var indented []string

	for _, line := range lines {
		if line == "" {
			indented = append(indented, "")
		} else {
			indented = append(indented, indent+line)
		}
	}

	return strings.Join(indented, "\n")
}

// escapeXML escapes special XML characters
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// escapeXMLComment escapes content for XML comments (-- is not allowed)
func escapeXMLComment(s string) string {
	// Replace all consecutive dashes with spaced dashes
	// We need to handle multiple consecutive dashes like "---" or "----"
	var result strings.Builder
	prevDash := false

	for _, ch := range s {
		if ch == '-' {
			if prevDash {
				// Add space before dash to break sequence
				result.WriteRune(' ')
			}
			result.WriteRune(ch)
			prevDash = true
		} else {
			result.WriteRune(ch)
			prevDash = false
		}
	}

	return result.String()
}
