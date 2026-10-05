package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Bootstrap file names (from OpenClaw pattern)
const (
	BootstrapAgentsFilename   = "AGENTS.md"
	BootstrapSoulFilename     = "SOUL.md"
	BootstrapToolsFilename    = "TOOLS.md"
	BootstrapIdentityFilename = "IDENTITY.md"
	BootstrapUserFilename     = "USER.md"
	BootstrapDefaultFilename  = "BOOTSTRAP.md"
)

// Bootstrap size limits (from OpenClaw)
const (
	DefaultBootstrapMaxChars      = 20000  // Per-file limit
	DefaultBootstrapTotalMaxChars = 150000 // Total bootstrap limit
	BootstrapHeadRatio            = 0.7    // 70% from beginning
	BootstrapTailRatio            = 0.2    // 20% from end
)

// BootstrapFile represents a single bootstrap file
type BootstrapFile struct {
	Name      string
	Path      string
	Content   string
	Missing   bool
	Truncated bool
	OrigLen   int
}

// BootstrapInjector handles bootstrap file injection
type BootstrapInjector struct {
	workspace     string
	maxChars      int
	totalMaxChars int
}

// NewBootstrapInjector creates a new bootstrap injector
func NewBootstrapInjector(workspace string) *BootstrapInjector {
	return &BootstrapInjector{
		workspace:     workspace,
		maxChars:      DefaultBootstrapMaxChars,
		totalMaxChars: DefaultBootstrapTotalMaxChars,
	}
}

// InjectBootstrapFiles loads and formats bootstrap files for system prompt
func (bi *BootstrapInjector) InjectBootstrapFiles() (string, error) {
	files := bi.loadBootstrapFiles()

	if len(files) == 0 {
		return "", nil // No bootstrap files found
	}

	// Enforce total size limit (Pattern: OpenClaw bootstrapTotalMaxChars)
	files = bi.enforceTotalSizeLimit(files)

	// Check if any files have content
	hasContent := false
	for _, file := range files {
		if !file.Missing && file.Content != "" {
			hasContent = true
			break
		}
	}

	// If no files have content, return empty string
	if !hasContent {
		return "", nil
	}

	var sb strings.Builder

	sb.WriteString("# Project Context\n\n")
	sb.WriteString("The following files from your workspace provide context about your identity, capabilities, and mission:\n\n")

	for _, file := range files {
		if file.Missing {
			// Skip missing files silently
			continue
		}

		if file.Content == "" {
			continue // Skip empty files
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", file.Name))
		sb.WriteString(file.Content)
		sb.WriteString("\n\n")

		// Add truncation warning if truncated
		if file.Truncated {
			// Warning is already embedded in content by trimBootstrapContent
		}
	}

	return sb.String(), nil
}

// enforceTotalSizeLimit enforces the total bootstrap size limit
// Pattern: OpenClaw total bootstrap size enforcement
func (bi *BootstrapInjector) enforceTotalSizeLimit(files []*BootstrapFile) []*BootstrapFile {
	totalChars := 0
	result := []*BootstrapFile{}

	for _, file := range files {
		if file.Missing || file.Content == "" {
			continue // Skip missing/empty files
		}

		// Check if adding this file would exceed total limit
		fileSize := len(file.Content)
		if totalChars+fileSize > bi.totalMaxChars {
			// Stop including files - total limit reached
			break
		}

		totalChars += fileSize
		result = append(result, file)
	}

	return result
}

// loadBootstrapFiles loads all bootstrap files from workspace
func (bi *BootstrapInjector) loadBootstrapFiles() []*BootstrapFile {
	fileNames := []string{
		BootstrapAgentsFilename,
		BootstrapSoulFilename,
		BootstrapToolsFilename,
		BootstrapIdentityFilename,
		BootstrapUserFilename,
		BootstrapDefaultFilename,
	}

	files := []*BootstrapFile{}

	for _, name := range fileNames {
		filePath := filepath.Join(bi.workspace, name)
		file := bi.loadBootstrapFile(name, filePath)
		files = append(files, file)
	}

	return files
}

// loadBootstrapFile loads a single bootstrap file
func (bi *BootstrapInjector) loadBootstrapFile(name string, path string) *BootstrapFile {
	content, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist or can't be read
		return &BootstrapFile{
			Name:    name,
			Path:    path,
			Missing: true,
		}
	}

	contentStr := string(content)
	trimmed := bi.trimBootstrapContent(contentStr, name)

	return &BootstrapFile{
		Name:      name,
		Path:      path,
		Content:   trimmed.Content,
		Truncated: trimmed.Truncated,
		OrigLen:   trimmed.OriginalLength,
		Missing:   false,
	}
}

// TrimResult holds the result of trimming bootstrap content
type TrimResult struct {
	Content        string
	Truncated      bool
	OriginalLength int
}

// trimBootstrapContent trims bootstrap content to maxChars with head/tail strategy
func (bi *BootstrapInjector) trimBootstrapContent(content string, fileName string) *TrimResult {
	// Trim trailing whitespace
	trimmed := strings.TrimRight(content, " \t\n\r")

	if len(trimmed) <= bi.maxChars {
		return &TrimResult{
			Content:        trimmed,
			Truncated:      false,
			OriginalLength: len(trimmed),
		}
	}

	// Calculate head and tail sizes
	headChars := int(float64(bi.maxChars) * BootstrapHeadRatio)
	tailChars := int(float64(bi.maxChars) * BootstrapTailRatio)

	// Extract head and tail
	head := trimmed[:headChars]
	tail := trimmed[len(trimmed)-tailChars:]

	// Build truncation marker
	marker := fmt.Sprintf("\n\n[...truncated, read %s for full content...]\n…(truncated %s: kept %d+%d chars of %d)…\n\n",
		fileName, fileName, headChars, tailChars, len(trimmed))

	// Combine head, marker, tail
	contentWithMarker := head + marker + tail

	return &TrimResult{
		Content:        contentWithMarker,
		Truncated:      true,
		OriginalLength: len(trimmed),
	}
}
