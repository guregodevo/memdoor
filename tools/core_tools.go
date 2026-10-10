package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"memdoor/pkg/sandbox"
	"memdoor/pkg/shared"

	"github.com/bmatcuk/doublestar/v4"
)

// ===============================================
// TOOL 1: write_file
// ===============================================

// WriteFileInput represents the input for write_file
type WriteFileInput struct {
	Path    string `json:"path" jsonschema_description:"File path to write (absolute or relative)"`
	Content string `json:"content" jsonschema_description:"Content to write to file"`
}

var WriteFileInputSchema = GenerateSchema[WriteFileInput]()

// WriteFileDefinition defines the write_file tool
var WriteFileDefinition = ToolDefinition{
	Name: "write_file",
	Description: `Write content to a file. Creates new file or completely overwrites existing file.

PARAMETERS:
• path: File path to write (REQUIRED)
• content: File content to write (REQUIRED)

⚠️ CONTENT SIZE LIMITS:
• Small files (<5KB): Use write_file with content parameter
• Large files (>5KB): Use bash with echo/cat/heredoc instead
• Very large reports/analysis: Generate incrementally with bash commands

USAGE:

Small files with content (RECOMMENDED for <5KB):
  write_file(path="config.json", content="{...}")
  ✓ File is created and populated in one step
  ✓ Best for: configs, small scripts, brief notes

Large files with bash (RECOMMENDED for >5KB):
  bash(command="cat > report.md << 'EOF'\n# Report\n...\nEOF")
  ✓ No size limits
  ✓ Best for: reports, comparisons, large analyses

Use cases:
- Creating new files with content (<5KB)
- Completely rewriting existing files
- Generating configs, small scripts, brief documentation

Note: This overwrites files without warning. For editing existing files, use edit_file instead.`,
	InputSchema:         WriteFileInputSchema,
	Function:            WriteFile,            // Legacy function (no sandbox enforcement)
	FunctionWithContext: WriteFileWithContext, // Sandbox-aware function
}

// WriteFile creates or overwrites a file with content
func WriteFile(input json.RawMessage) (string, error) {
	var params WriteFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required parameters
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	defer LockFiles(params.Path)()

	// Create parent directories if needed
	dir := filepath.Dir(params.Path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write file
	if err := os.WriteFile(params.Path, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("File written successfully: %s (%d bytes)", params.Path, len(params.Content)), nil
}

// WriteFileWithContext writes a file with sandbox path validation
func WriteFileWithContext(input json.RawMessage, ctx interface{}) (string, error) {
	var params WriteFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required parameters
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	defer LockFiles(params.Path)()

	// Extract sandbox context
	sandboxCtx, ok := ctx.(sandbox.SandboxContext)
	if !ok {
		// No sandbox context available - fall back to unrestricted access
		return WriteFile(input)
	}

	// Normalize path (convert relative to virtual path)
	virtualPath := sandboxCtx.NormalizePath(params.Path)

	// Resolve virtual path to real filesystem path
	realPath, err := sandboxCtx.ResolvePath(virtualPath)
	if err != nil {
		return "", fmt.Errorf("❌ **Sandbox Security: Access Denied**\n\nPath: %s\nVirtual: %s\nAgent Scope: %s\n\nThis agent does not have permission to write to this path.\n\nAllowed paths:\n%s",
			params.Path,
			virtualPath,
			sandboxCtx.AgentScope,
			sandboxCtx.GetAllowedPathsDescription())
	}

	// Create parent directories if needed
	dir := filepath.Dir(realPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write file
	if err := os.WriteFile(realPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("✅ File written successfully!\n\nVirtual path: %s\nReal path: %s\nSize: %d bytes", virtualPath, realPath, len(params.Content)), nil
}

// ===============================================
// TOOL 2: edit_file
// ===============================================

// EditFileInput represents the input for edit_file. It is the single file-writing
// tool: with old_string it does a surgical replace; with old_string empty (or a
// file that doesn't exist yet) it writes new_string as the WHOLE file — i.e. create
// or overwrite. path is accepted as an alias for file_path.
type EditFileInput struct {
	FilePath  string `json:"file_path" jsonschema_description:"File path to write/edit (absolute or relative)."`
	Path      string `json:"path,omitempty" jsonschema_description:"Alias for file_path."`
	OldString string `json:"old_string,omitempty" jsonschema_description:"Exact existing text to replace (whitespace-exact). Leave EMPTY to create a new file or overwrite the whole file with new_string."`
	NewString string `json:"new_string" jsonschema_description:"Replacement text, or — when old_string is empty — the full content of the file to create/overwrite."`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

// EditFileDefinition defines the edit_file tool (matches Claude CLI Edit tool)
var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `The single tool for writing files — it CREATES, overwrites, and edits.

TWO MODES:
• CREATE / overwrite a whole file: leave old_string EMPTY and put the full file
  content in new_string. Use this to create a new file or rewrite one entirely.
    edit_file(path="main.go", new_string="package main\n\nfunc main() {}\n")
    edit_file(path="app.py", new_string="def main():\n    pass\n")
• SURGICAL edit of an existing file: set old_string to the exact text to replace.
    edit_file(path="app.py", old_string="def main():", new_string="def main(name):")

PARAMETERS:
• path (or file_path): File to write/edit (REQUIRED)
• old_string: Exact existing text to replace; EMPTY to create/overwrite the file
• new_string: Replacement text, or the full file content when old_string is empty

⚠️ CRITICAL REQUIREMENTS:
1. MUST read file first using Read tool to see exact content
2. old_string must match EXACTLY (including all whitespace, tabs, indentation)
3. old_string must appear EXACTLY ONCE in the file (fails if 0 or >1 matches)
4. Preserve exact indentation from Read tool output (tabs vs spaces)
5. old_string and new_string must be different

WORKFLOW:
1. Use Read tool to view file content
2. Copy exact text from Read output (preserve all whitespace)
3. Call edit_file with exact old_string and new new_string

EXAMPLES:

Correct usage (preserving indentation):
  Read output shows:
    func example() {
        return true
    }

  edit_file(
    file_path="example.go",
    old_string="    return true",
    new_string="    return false"
  )

Incorrect usage (wrong indentation):
  ❌ old_string="return true"  (missing indentation)
  ❌ old_string="  return true"  (spaces instead of tabs)

Common mistakes:
- Not reading file first
- Not matching exact indentation
- String appears multiple times (use search_replace for bulk changes)
- Trying to match partial lines

Note: For bulk replacements across multiple files, use search_replace instead.`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

// EditFile performs exact string replacement in a file (matches Claude CLI Edit tool)
func EditFile(input json.RawMessage) (string, error) {
	var params EditFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Accept `path` as an alias for `file_path` (consistent with read_file/write).
	if params.FilePath == "" {
		params.FilePath = params.Path
	}
	if params.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}
	defer LockFiles(params.FilePath)()

	// Create / whole-file overwrite: with no anchor to match — an empty old_string,
	// or a file that doesn't exist yet — write new_string as the entire file. This is
	// what makes edit_file the single file tool (create + edit), so a small model
	// doesn't have to juggle a separate write_file.
	existing, readErr := os.ReadFile(params.FilePath)
	if params.OldString == "" || os.IsNotExist(readErr) {
		if err := os.WriteFile(params.FilePath, []byte(params.NewString), 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		return fmt.Sprintf("Wrote %s (%d bytes)", params.FilePath, len(params.NewString)), nil
	}
	if readErr != nil {
		return "", fmt.Errorf("failed to read file %s: %w", params.FilePath, readErr)
	}

	// Surgical replace path.
	if params.OldString == params.NewString {
		return "", fmt.Errorf("old_string and new_string must be different")
	}
	content := existing

	fileContent := string(content)

	// Count occurrences of old_string
	count := strings.Count(fileContent, params.OldString)

	// Validate exactly one match
	if count == 0 {
		return "", fmt.Errorf("old_string not found in file. Make sure to:\n1. Read the file first using Read tool\n2. Copy the exact text including all whitespace/indentation\n3. Check for typos or whitespace mismatches")
	}
	if count > 1 {
		return "", fmt.Errorf("old_string appears %d times in file (must appear exactly once). For bulk replacements, use search_replace tool instead", count)
	}

	// Perform replacement
	newContent := strings.Replace(fileContent, params.OldString, params.NewString, 1)

	// Write back to file
	if err := os.WriteFile(params.FilePath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return a line-numbered +/- diff (claude-source style) so the human sees
	// exactly what changed. The marker is column 0 (+ added, - removed, space
	// context) followed by the line number, which the TUI colors.
	diff := formatEditDiff(fileContent, newContent, 3)
	return fmt.Sprintf("Updated %s\n%s", params.FilePath, diff), nil
}

// formatEditDiff renders a single-hunk unified diff between old and new file
// content with line numbers, in the "<marker> <lineno> <content>" layout the TUI
// color-renders. Because edit_file performs one old→new replacement, the change is
// a contiguous region: everything before the first differing line and after the
// last differing line is shared, so a common-prefix/suffix scan yields the hunk
// without a full LCS. contextLines shared lines are shown on each side.
func formatEditDiff(oldContent, newContent string, contextLines int) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Common prefix.
	p := 0
	for p < len(oldLines) && p < len(newLines) && oldLines[p] == newLines[p] {
		p++
	}
	// Common suffix (not overlapping the prefix).
	s := 0
	for s < len(oldLines)-p && s < len(newLines)-p &&
		oldLines[len(oldLines)-1-s] == newLines[len(newLines)-1-s] {
		s++
	}

	removed := oldLines[p : len(oldLines)-s]
	added := newLines[p : len(newLines)-s]

	// Width of the largest line number shown, for right-alignment.
	maxNo := len(newLines)
	if len(oldLines) > maxNo {
		maxNo = len(oldLines)
	}
	w := len(fmt.Sprintf("%d", maxNo))

	var b strings.Builder
	line := func(marker string, no int, text string) {
		fmt.Fprintf(&b, "%s %*d %s\n", marker, w, no, text)
	}

	// Leading context (shared lines just before the change).
	ctxStart := p - contextLines
	if ctxStart < 0 {
		ctxStart = 0
	}
	for i := ctxStart; i < p; i++ {
		line(" ", i+1, oldLines[i])
	}
	// Removed lines carry OLD numbering, added lines carry NEW numbering.
	for i, t := range removed {
		line("-", p+i+1, t)
	}
	for i, t := range added {
		line("+", p+i+1, t)
	}
	// Trailing context (shared lines just after the change), numbered in the new file.
	tailStart := len(newLines) - s
	tailEnd := tailStart + contextLines
	if tailEnd > len(newLines) {
		tailEnd = len(newLines)
	}
	for i := tailStart; i < tailEnd; i++ {
		line(" ", i+1, newLines[i])
	}

	return strings.TrimRight(b.String(), "\n")
}

// ===============================================
// TOOL 3: glob
// ===============================================

// GlobInput represents the input for glob
type GlobInput struct {
	Pattern string `json:"pattern" jsonschema_description:"Glob pattern (e.g., '**/*.go', 'src/**/*.ts', 'docs/*.md')"`
	Path    string `json:"path,omitempty" jsonschema_description:"Directory to search in (default: current directory)"`
}

var GlobInputSchema = GenerateSchema[GlobInput]()

// GlobDefinition defines the glob tool
var GlobDefinition = ToolDefinition{
	Name:        "glob",
	Description: `Find files by NAME pattern ("**/*.go", "src/**/*.ts"); returns the paths, most recently modified first. It does not look inside files: to find text in them use jgrep or grep.`,
	InputSchema: GlobInputSchema,
	Function:    Glob,
}

// Glob finds files matching a glob pattern (implements Claude Code Glob tool spec)
func Glob(input json.RawMessage) (string, error) {
	var params GlobInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate pattern
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Set default path
	searchPath := params.Path
	if searchPath == "" {
		searchPath = "."
	}

	// Find matching files using doublestar
	type FileWithTime struct {
		Path    string
		ModTime int64
	}

	var filesWithTime []FileWithTime

	// VCS internals, dependency trees and Memdoor's own state are never
	// listed, as grep never searches them: `**/*` in a project with a .git
	// returned ~400 paths, most of them .git/objects and hook samples, into
	// the window and the model's context (live 2026-10-05).
	err := doublestar.GlobWalk(os.DirFS(searchPath), params.Pattern, func(path string, d os.DirEntry) error {
		if d.IsDir() && path != "." && searchSkipDirs[d.Name()] {
			return fs.SkipDir
		}
		if underSkippedDir(path) {
			return nil
		}
		if !d.IsDir() {
			// Prepend searchPath if not current directory
			fullPath := path
			if searchPath != "." {
				fullPath = filepath.Join(searchPath, path)
			}

			// Get modification time
			info, err := d.Info()
			var modTime int64
			if err == nil {
				modTime = info.ModTime().Unix()
			}

			filesWithTime = append(filesWithTime, FileWithTime{
				Path:    fullPath,
				ModTime: modTime,
			})
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	// Sort by modification time (most recent first, as Claude Code does)
	for i := 0; i < len(filesWithTime); i++ {
		for j := i + 1; j < len(filesWithTime); j++ {
			if filesWithTime[j].ModTime > filesWithTime[i].ModTime {
				filesWithTime[i], filesWithTime[j] = filesWithTime[j], filesWithTime[i]
			}
		}
	}

	// Extract paths
	var matches []string
	for _, f := range filesWithTime {
		matches = append(matches, f.Path)
	}

	// Return newline-separated file paths (Claude Code format)
	if len(matches) == 0 {
		return "No files found matching pattern: " + params.Pattern, nil
	}

	return strings.Join(matches, "\n"), nil
}

// ===============================================
// TOOL 4: grep
// ===============================================

// GrepInput represents the input for grep (matches Claude Code Grep tool schema)
// GrepInput: -A, -B, type and multiline are still read when sent, but not
// offered: every schema rides on every request (2026-09-29).
type GrepInput struct {
	Pattern    string `json:"pattern" jsonschema_description:"Regular expression to search for."`
	Path       string `json:"path,omitempty" jsonschema_description:"File or directory to search. Default: the working directory."`
	Glob       string `json:"glob,omitempty" jsonschema_description:"Only files matching this glob, e.g. '*.go' or '*.{ts,tsx}'."`
	Type       string `json:"type,omitempty" jsonschema:"-"` // glob covers it; not in the schema
	I          bool   `json:"-i,omitempty" jsonschema_description:"true to match case-insensitively (default false)."`
	N          bool   `json:"-n,omitempty" jsonschema_description:"true to prefix line numbers (content mode only)."`
	A          int    `json:"-A,omitempty" jsonschema:"-"` // -C covers both; not in the schema
	B          int    `json:"-B,omitempty" jsonschema:"-"`
	C          int    `json:"-C,omitempty" jsonschema_description:"Lines around each match (content mode)."`
	OutputMode string `json:"output_mode,omitempty" jsonschema_description:"files_with_matches (default), content, or count."`
	HeadLimit  int    `json:"head_limit,omitempty" jsonschema_description:"At most this many lines or entries."`
	Multiline  bool   `json:"multiline,omitempty" jsonschema:"-"` // rare; not in the schema
}

var GrepInputSchema = GenerateSchema[GrepInput]()

// GrepDefinition defines the grep tool
var GrepDefinition = ToolDefinition{
	Name:        "grep",
	Description: `Search file contents with a regular expression (ripgrep syntax: escape literal braces, e.g. interface\{\}). Returns EVERY match; when you need only the matches that matter to a question, use jgrep. Use this rather than grep or rg in bash. Default output is the matching file paths; output_mode "content" shows the lines, "count" the counts.`,
	InputSchema: GrepInputSchema,
	Function:    Grep,
}

// Grep searches file contents for a pattern (implements Claude Code Grep tool spec)
func Grep(input json.RawMessage) (string, error) {
	var params GrepInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	outputMode := params.OutputMode
	if outputMode == "" {
		outputMode = "files_with_matches" // Default mode
	}
	re, err := compileGrepPattern(params)
	if err != nil {
		return "", err
	}

	type FileMatch struct {
		File    string
		Line    int
		Content string
		Context []string // For -A/-B/-C context lines
	}
	var fileMatches []string        // For files_with_matches mode
	var contentMatches []FileMatch  // For content mode
	var countMatches map[string]int // For count mode
	if outputMode == "count" {
		countMatches = make(map[string]int)
	}
	if params.C > 0 {
		params.B = params.C
		params.A = params.C
	}
	resultCount := 0

	err = grepWalk(params, re, func(path string, lines []string, hits []int) error {
		if params.Multiline {
			if re.MatchString(strings.Join(lines, "\n")) {
				if outputMode == "files_with_matches" {
					fileMatches = append(fileMatches, path)
					resultCount++
				} else if outputMode == "count" {
					countMatches[path]++
				}
			}
			return nil
		}
		if len(hits) == 0 {
			return nil
		}
		switch outputMode {
		case "files_with_matches":
			fileMatches = append(fileMatches, path)
			resultCount++
			if params.HeadLimit > 0 && resultCount >= params.HeadLimit {
				return filepath.SkipAll
			}
		case "count":
			countMatches[path] += len(hits)
		default:
			for _, lineNum := range hits {
				match := FileMatch{File: path, Line: lineNum + 1, Content: lines[lineNum]}
				if params.B > 0 || params.A > 0 {
					match.Context = lines[max(0, lineNum-params.B):min(len(lines), lineNum+params.A+1)]
				}
				contentMatches = append(contentMatches, match)
				resultCount++
				if params.HeadLimit > 0 && resultCount >= params.HeadLimit {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("grep failed: %w", err)
	}

	// Format output based on mode
	if outputMode == "files_with_matches" {
		return strings.Join(fileMatches, "\n"), nil
	} else if outputMode == "count" {
		var lines []string
		for file, count := range countMatches {
			lines = append(lines, fmt.Sprintf("%s: %d", file, count))
		}
		return strings.Join(lines, "\n"), nil
	}
	var lines []string
	for _, match := range contentMatches {
		if params.N {
			lines = append(lines, fmt.Sprintf("%s:%d:%s", match.File, match.Line, match.Content))
		} else {
			lines = append(lines, fmt.Sprintf("%s:%s", match.File, match.Content))
		}
		for _, ctx := range match.Context {
			lines = append(lines, "  "+ctx)
		}
	}
	return capGrepOutput(strings.Join(lines, "\n"), len(contentMatches), params.HeadLimit), nil
}

// grepOutputMax bounds grep's content output when no head_limit was given. A
// broad regex returned 86 KB (2026-09-25), all of it resent on every later
// call; past this the head comes back with the count and the way to the
// matches that matter.
const grepOutputMax = 24 << 10

func capGrepOutput(out string, matches, headLimit int) string {
	if headLimit > 0 || len(out) <= grepOutputMax {
		return out
	}
	cut := strings.LastIndexByte(out[:grepOutputMax], '\n')
	if cut < 0 {
		cut = grepOutputMax
	}
	shown := strings.Count(out[:cut], "\n") + 1
	return out[:cut] + fmt.Sprintf("\n…[%d matches, %d KB: the first %d lines are shown. For the matches that matter use jgrep with the same pattern and a task; or narrow path, glob or pattern; or set head_limit]",
		matches, len(out)>>10, shown)
}

// compileGrepPattern applies the i / multiline flags. Shared by grep and jgrep.
func compileGrepPattern(params GrepInput) (*regexp.Regexp, error) {
	if params.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	flags := ""
	if params.I {
		flags = "(?i)"
	}
	if params.Multiline {
		flags += "(?s)" // . matches newlines
	}
	re, err := regexp.Compile(flags + params.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return re, nil
}

// underSkippedDir says whether a relative path has a never-listed directory
// among its parents: a pattern that matches files only (`**/*.sample`) never
// hands the walk the directory itself to skip.
func underSkippedDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		if searchSkipDirs[part] {
			return true
		}
	}
	return false
}

// searchSkipDirs are never searched or listed (grep, glob): VCS internals,
// dependency trees and Memdoor's own state.
var searchSkipDirs = map[string]bool{".git": true, shared.MemdoorDirName: true, "node_modules": true, ".venv": true, "__pycache__": true} // .memdoor holds the undo snapshots (gateway/checkpoint.go)

// grepWalk is THE file walk behind grep and jgrep: path default ".", glob and
// type filters, binary skip, and per-file line matching. visit gets each
// text file's lines and the 0-based indices of matching lines (multiline mode
// leaves matching to the caller).
func grepWalk(params GrepInput, re *regexp.Regexp, visit func(path string, lines []string, hits []int) error) error {
	searchPath := params.Path
	if searchPath == "" {
		searchPath = "."
	}
	return filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != searchPath && searchSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if params.Glob != "" && !grepGlobMatch(params.Glob, filepath.Base(path)) {
			return nil
		}
		if params.Type != "" && !grepTypeMatch(params.Type, filepath.Ext(path)) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(content[:min(512, len(content))], 0) >= 0 {
			return nil // unreadable or binary
		}
		lines := strings.Split(string(content), "\n")
		var hits []int
		if !params.Multiline {
			for i, line := range lines {
				if re.MatchString(line) {
					hits = append(hits, i)
				}
			}
		}
		return visit(path, lines, hits)
	})
}

// grepGlobMatch accepts "*.go" and brace sets like "*.{ts,tsx}" (rg --glob).
func grepGlobMatch(glob, name string) bool {
	if i, j := strings.Index(glob, "{"), strings.Index(glob, "}"); i >= 0 && j > i {
		for _, alt := range strings.Split(glob[i+1:j], ",") {
			if ok, _ := filepath.Match(glob[:i]+alt+glob[j+1:], name); ok {
				return true
			}
		}
		return false
	}
	ok, _ := filepath.Match(glob, name)
	return ok
}

func grepTypeMatch(typ, ext string) bool {
	switch typ {
	case "rust":
		return ext == ".rs"
	default:
		return ext == "."+typ
	}
}

// ===============================================
// TOOL 5: ask_user_question
// ===============================================

// AskUserQuestionInput represents the input for ask_user_question
type AskUserQuestionInput struct {
	Question string   `json:"question" jsonschema_description:"The specific decision you cannot make yourself, in one sentence. Name the thing you are choosing between, not the whole task: \"Which eviction policy should the cache use?\", not \"How should I do this?\". The user is looking at a picker, not a transcript, so it must stand alone."`
	Options  []string `json:"options" jsonschema_description:"2-5 CONCRETE choices the user can pick between, each a short phrase naming an actual option (e.g. [\"LRU\", \"FIFO\", \"random eviction\"]). Not open-ended prompts, not \"yes\"/\"no\" on a question you could answer yourself by reading the code."`
	Default  string   `json:"default,omitempty" jsonschema_description:"The option to use if the user just presses Enter. Must be one of the strings in options. Pick the one you would have chosen alone — this is what makes asking cheap rather than blocking."`
}

var AskUserQuestionInputSchema = GenerateSchema[AskUserQuestionInput]()

// AskUserQuestionDefinition defines the ask_user_question tool
var AskUserQuestionDefinition = ToolDefinition{
	Name:        "ask_user_question",
	Description: `Ask the user a multiple-choice question (they pick by number). Only for a real choice or ambiguity you cannot resolve from the code.`,
	InputSchema: AskUserQuestionInputSchema,
	Function:    AskUserQuestion,
}

// AskUserQuestion asks the user an interactive question
func AskUserQuestion(input json.RawMessage) (string, error) {
	var params AskUserQuestionInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate
	if params.Question == "" {
		return "", fmt.Errorf("question is required")
	}
	if len(params.Options) < 2 || len(params.Options) > 5 {
		return "", fmt.Errorf("must provide 2-5 options")
	}

	// Display question
	fmt.Println("\n" + params.Question)
	for i, option := range params.Options {
		defaultMarker := ""
		if option == params.Default {
			defaultMarker = " (default)"
		}
		fmt.Printf("%d. %s%s\n", i+1, option, defaultMarker)
	}

	// Get user input
	fmt.Print("\nYour choice (1-" + strconv.Itoa(len(params.Options)) + "): ")
	reader := bufio.NewReader(os.Stdin)
	userInput, _ := reader.ReadString('\n')
	userInput = strings.TrimSpace(userInput)

	// Parse choice
	var selected string
	if userInput == "" && params.Default != "" {
		selected = params.Default
	} else {
		choice, err := strconv.Atoi(userInput)
		if err != nil || choice < 1 || choice > len(params.Options) {
			return "", fmt.Errorf("invalid choice: please enter a number between 1 and %d", len(params.Options))
		}
		selected = params.Options[choice-1]
	}

	// Format result
	result := map[string]any{
		"question": params.Question,
		"selected": selected,
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}

// ===============================================
// TOOL 6: todo_write
// ===============================================

// TodoWriteInput represents the input for todo_write
type TodoWriteInput struct {
	Todos []TodoItem `json:"todos" jsonschema_description:"List of todo items with status"`
}

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string `json:"content" jsonschema_description:"Task description (imperative form, e.g., 'Run tests')"`
	Status     string `json:"status" jsonschema_description:"Status: pending, in_progress, or completed"`
	ActiveForm string `json:"active_form" jsonschema_description:"Present continuous form (e.g., 'Running tests')"`
}

var TodoWriteInputSchema = GenerateSchema[TodoWriteInput]()

// TodoWriteDefinition defines the todo_write tool
var TodoWriteDefinition = ToolDefinition{
	Name:        "todo_write",
	Description: `Track a multi-step task. Pass the FULL list every call with each item's status (pending, in_progress, completed). The reply names the single next action, or says the task is done and must be verified; keep going while it names one.`,
	InputSchema: TodoWriteInputSchema,
	Function:    TodoWrite,
}

// TodoWrite is the STANDALONE (non-session) formatter — it echoes the list plus
// the follow-up directive via FormatTodos. The DURABLE, channel-shared behavior
// lives in the gateway's executeTool special-case (which has the session/channel
// key): it persists the list to the shared task store so the planner and coder
// read the same live list across turns. This standalone path is the fallback for
// callers without a session (tests, direct tool invocation).
func TodoWrite(input json.RawMessage) (string, error) {
	var params TodoWriteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if len(params.Todos) == 0 {
		return "", fmt.Errorf("at least one todo item is required")
	}
	return FormatTodos(params.Todos), nil
}

// TodoReadInput is empty — todo_read takes no arguments; it returns the current
// shared task list for the channel.
type TodoReadInput struct{}

// TodoReadInputSchema is the (empty) schema for todo_read.
var TodoReadInputSchema = GenerateSchema[TodoReadInput]()

// TodoReadDefinition defines todo_read — the planner's follow-up read path. The
// real (channel-scoped) read is the executeTool special-case; this standalone
// Function is a stub for callers without a session.
var TodoReadDefinition = ToolDefinition{
	Name:        "todo_read",
	Description: `Read the current shared task list (the todos maintained with todo_write) and the next step to follow up on. Use this to check what is still pending before deciding the next action or declaring the task done.`,
	InputSchema: TodoReadInputSchema,
	Function: func(json.RawMessage) (string, error) {
		return "No task list available (call from within a session).", nil
	},
}

// ===============================================
// TOOL 7: search_replace
// ===============================================

// SearchReplaceInput represents the input for search_replace
type SearchReplaceInput struct {
	Search      string `json:"search" jsonschema_description:"Text to search for (exact match)"`
	Replace     string `json:"replace" jsonschema_description:"Replacement text"`
	FilePattern string `json:"file_pattern,omitempty" jsonschema_description:"File pattern to filter (e.g., '*.go', '*.ts')"`
	Path        string `json:"path,omitempty" jsonschema_description:"Directory to search in (default: current directory)"`
	DryRun      bool   `json:"dry_run,omitempty" jsonschema_description:"Preview changes without applying them (default: false)"`
}

var SearchReplaceInputSchema = GenerateSchema[SearchReplaceInput]()

// SearchReplaceDefinition defines the search_replace tool
var SearchReplaceDefinition = ToolDefinition{
	Name: "search_replace",
	Description: `Find and replace text across multiple files.

Use this to:
- Rename variables/functions across codebase
- Update configuration values
- Refactor code patterns
- Mass text replacements

IMPORTANT: Use dry_run=true first to preview changes before applying them.`,
	InputSchema: SearchReplaceInputSchema,
	Function:    SearchReplace,
}

// SearchReplace finds and replaces text across files
func SearchReplace(input json.RawMessage) (string, error) {
	var params SearchReplaceInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate
	if params.Search == "" {
		return "", fmt.Errorf("search string is required")
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath = "."
	}

	var changes []map[string]any

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Filter by file pattern if specified
		if params.FilePattern != "" {
			matched, _ := filepath.Match(params.FilePattern, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		// Read file — this file is ours until the callback returns.
		defer LockFiles(path)()
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check if file contains search string
		if !strings.Contains(string(content), params.Search) {
			return nil
		}

		// Count occurrences
		occurrences := strings.Count(string(content), params.Search)

		// Replace
		newContent := strings.ReplaceAll(string(content), params.Search, params.Replace)

		// Record change
		changes = append(changes, map[string]any{
			"file":        path,
			"occurrences": occurrences,
		})

		// Apply if not dry run
		if !params.DryRun {
			if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("search-replace failed: %w", err)
	}

	// Format result
	mode := "dry-run"
	if !params.DryRun {
		mode = "applied"
	}

	result := map[string]any{
		"search":        params.Search,
		"replace":       params.Replace,
		"files_changed": len(changes),
		"mode":          mode,
		"changes":       changes,
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}

// ===============================================
// TOOL 8: agents_list
// ===============================================

// AgentsListInput represents the input for agents_list. LLM
// provider/model are workspace-wide via byok — there's no per-agent
// model field anymore.
type AgentsListInput struct {
	Action string   `json:"action" jsonschema_description:"Action: 'list' (default), 'get', 'create', 'update'"`
	Name   string   `json:"name,omitempty" jsonschema_description:"Agent name (required for get/create/update)"`
	Tools  []string `json:"tools,omitempty" jsonschema_description:"Tool names for create/update"`
	Emoji  string   `json:"emoji,omitempty" jsonschema_description:"Avatar emoji for create"`
	Scope  string   `json:"scope,omitempty" jsonschema_description:"Sandbox scope for create/update: user, channel, workspace"`

	LearningEnabled *bool `json:"learning_enabled,omitempty" jsonschema_description:"Enable learning loop (for update)"`
}

var AgentsListInputSchema = GenerateSchema[AgentsListInput]()

// AgentsListDefinition defines the agents_list tool
var AgentsListDefinition = ToolDefinition{
	Name: "agents_list",
	Description: `Manage agents in the workspace.

Actions:
- list: List all agents (default)
- get: Get details for a specific agent (requires name)
- create: Create a new agent (requires name; optional: tools, emoji, scope)
- update: Update an existing agent (requires name; optional: tools, scope, learning_enabled)

Examples:
  {"action": "list"}
  {"action": "get", "name": "writer"}
  {"action": "create", "name": "monitor", "emoji": "🔍", "tools": ["bash", "read_file", "memory"], "scope": "workspace"}
  {"action": "update", "name": "writer", "learning_enabled": true}`,
	InputSchema: AgentsListInputSchema,
	Function:    AgentsList,
}

// AgentsList manages agents via the gateway API
func AgentsList(input json.RawMessage) (string, error) {
	var params AgentsListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	action := params.Action
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		return agentsAPICall("GET", apiURL("/api/agents"), nil)
	case "get":
		if params.Name == "" {
			return "", fmt.Errorf("name is required for get action")
		}
		return agentsAPICall("GET", apiURL("/api/agents")+"/"+params.Name, nil)
	case "create":
		if params.Name == "" {
			return "", fmt.Errorf("name is required for create action")
		}
		body := map[string]any{
			"name":         params.Name,
			"avatar_emoji": params.Emoji,
		}
		if len(params.Tools) > 0 {
			body["tools"] = params.Tools
		}
		if params.Scope != "" {
			body["sandbox_scope"] = params.Scope
		}
		return agentsAPICall("POST", apiURL("/api/agents"), body)
	case "update":
		if params.Name == "" {
			return "", fmt.Errorf("name is required for update action")
		}
		body := map[string]any{}
		if len(params.Tools) > 0 {
			body["tools"] = params.Tools
		}
		if params.Scope != "" {
			body["sandbox_scope"] = params.Scope
		}
		if params.LearningEnabled != nil {
			body["learning_enabled"] = *params.LearningEnabled
		}
		if len(body) == 0 {
			return "", fmt.Errorf("no fields to update")
		}
		return agentsAPICall("PUT", apiURL("/api/agents")+"/"+params.Name, body)
	default:
		return "", fmt.Errorf("unknown action: %s (use list, get, create, update)", action)
	}
}

// agentsAPICall makes an HTTP request to the gateway agents API
func agentsAPICall(method, url string, body map[string]any) (string, error) {
	var reqBody io.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewReader(bodyJSON)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "agents")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Pretty-print JSON response
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	pretty, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return string(respBody), nil
	}
	return string(pretty), nil
}

// ===============================================
// TOOL 10: channels
// ===============================================

// ChannelsInput represents the input for channels tool
type ChannelsInput struct {
	Action      string `json:"action" jsonschema_description:"Action: 'list', 'create', 'get', 'add_member', 'list_members', 'send_message'"`
	Name        string `json:"name,omitempty" jsonschema_description:"Channel name (required for create)"`
	ChannelID   string `json:"channel_id,omitempty" jsonschema_description:"Channel ID (required for get, add_member, list_members, send_message)"`
	Type        string `json:"type,omitempty" jsonschema_description:"Channel type for create: 'public' (default) or 'private'"`
	Description string `json:"description,omitempty" jsonschema_description:"Channel description (optional for create)"`
	OwnerID     string `json:"owner_id,omitempty" jsonschema_description:"Owner actor ID for create (required). Added as admin member automatically. e.g. 'human:uuid' or 'agent:name'"`
	ActorID     string `json:"actor_id,omitempty" jsonschema_description:"Actor ID to add as member (required for add_member, e.g. 'human:uuid' or 'agent:name')"`
	Role        string `json:"role,omitempty" jsonschema_description:"Role for add_member: 'member' (default) or 'admin'"`
	Text        string `json:"text,omitempty" jsonschema_description:"Message text (required for send_message)"`
	SenderID    string `json:"sender_id,omitempty" jsonschema_description:"Sender actor ID for send_message (e.g. 'agent:personalagent'). If omitted, sends as system."`
}

var ChannelsInputSchema = GenerateSchema[ChannelsInput]()

// ChannelsDefinition defines the channels tool
var ChannelsDefinition = ToolDefinition{
	Name: "channels",
	Description: `Manage workspace channels: create, list, add members, and send messages.

Actions:
- list: List all accessible channels
- create: Create a new channel (requires name, owner_id; optional: type, description). Owner is auto-added as admin.
- get: Get channel details and members (requires channel_id)
- add_member: Add a user or agent to a channel (requires channel_id, actor_id)
- list_members: List members of a channel (requires channel_id)
- send_message: Send a message to a channel (requires channel_id, text)

Examples:
  {"action": "list"}
  {"action": "create", "name": "project-alpha", "type": "private", "description": "Alpha project discussion", "owner_id": "human:uuid"}
  {"action": "add_member", "channel_id": "uuid", "actor_id": "agent:writer", "role": "member"}
  {"action": "send_message", "channel_id": "uuid", "text": "Daily standup: all systems green."}`,
	InputSchema: ChannelsInputSchema,
	Function:    Channels,
}

// Channels implements the channels tool
func Channels(input json.RawMessage) (string, error) {
	var params ChannelsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if params.Action == "" {
		return "", fmt.Errorf("action is required (list, create, get, add_member, list_members, send_message)")
	}

	switch params.Action {
	case "list":
		return agentsAPICall("GET", apiURL("/api/channels"), nil)

	case "create":
		if params.Name == "" {
			return "", fmt.Errorf("name is required for create action")
		}
		if params.OwnerID == "" {
			return "", fmt.Errorf("owner_id is required for create action (e.g. 'human:uuid' or 'agent:name')")
		}
		channelType := params.Type
		if channelType == "" {
			channelType = "public"
		}
		body := map[string]any{
			"name":     params.Name,
			"type":     channelType,
			"owner_id": params.OwnerID,
		}
		if params.Description != "" {
			body["description"] = params.Description
		}
		return agentsAPICall("POST", apiURL("/api/channels"), body)

	case "get":
		if params.ChannelID == "" {
			return "", fmt.Errorf("channel_id is required for get action")
		}
		return agentsAPICall("GET", apiURL("/api/channels")+"/"+params.ChannelID, nil)

	case "add_member":
		if params.ChannelID == "" {
			return "", fmt.Errorf("channel_id is required for add_member action")
		}
		if params.ActorID == "" {
			return "", fmt.Errorf("actor_id is required for add_member action (e.g. 'human:uuid' or 'agent:name')")
		}
		role := params.Role
		if role == "" {
			role = "member"
		}
		body := map[string]any{
			"channel_id": params.ChannelID,
			"actor_id":   params.ActorID,
			"role":       role,
		}
		return agentsAPICall("POST", apiURL("/api/channels")+"/members", body)

	case "list_members":
		if params.ChannelID == "" {
			return "", fmt.Errorf("channel_id is required for list_members action")
		}
		return agentsAPICall("GET", apiURL("/api/channels")+"/"+params.ChannelID+"/members", nil)

	case "send_message":
		if params.ChannelID == "" {
			return "", fmt.Errorf("channel_id is required for send_message action")
		}
		if params.Text == "" {
			return "", fmt.Errorf("text is required for send_message action")
		}
		body := map[string]any{
			"channel_id": params.ChannelID,
			"text":       params.Text,
		}
		if params.SenderID != "" {
			body["sender_id"] = params.SenderID
		}
		return agentsAPICall("POST", apiURL("/api/messages"), body)

	default:
		return "", fmt.Errorf("unknown action: %s (use list, create, get, add_member, list_members, send_message)", params.Action)
	}
}

// ===============================================
// TOOL: send_invite
// ===============================================

// SendInviteInput represents the input for send_invite tool
type SendInviteInput struct {
	Email       string `json:"email" jsonschema_description:"Email address to invite"`
	ChannelName string `json:"channel_name,omitempty" jsonschema_description:"Channel name to invite the user to (optional)"`
	Message     string `json:"message,omitempty" jsonschema_description:"Personal message to include in the invite (optional)"`
	InviterName string `json:"inviter_name,omitempty" jsonschema_description:"Name of the person inviting (defaults to 'Memdoor')"`
}

var SendInviteInputSchema = GenerateSchema[SendInviteInput]()

// SendInviteDefinition defines the send_invite tool
var SendInviteDefinition = ToolDefinition{
	Name: "send_invite",
	Description: `Send an email invitation to join the Memdoor workspace.

PARAMETERS:
- email: Email address to invite (REQUIRED)
- channel_name: Channel to invite to (optional)
- message: Personal message to include (optional)
- inviter_name: Who is inviting (optional, defaults to "Memdoor")

EXAMPLE:
  send_invite(email="alice@example.com", channel_name="engineering", message="Join us to collaborate with AI agents!")`,
	InputSchema: SendInviteInputSchema,
	Function:    SendInvite,
}

// SendInvite implements the send_invite tool
func SendInvite(input json.RawMessage) (string, error) {
	var params SendInviteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Email == "" {
		return "", fmt.Errorf("email is required")
	}

	inviteAPIBase := apiURL("/api/invite")
	body := map[string]any{
		"email": params.Email,
	}
	if params.ChannelName != "" {
		body["channel_name"] = params.ChannelName
	}
	if params.Message != "" {
		body["message"] = params.Message
	}
	if params.InviterName != "" {
		body["inviter_name"] = params.InviterName
	}
	return agentsAPICall("POST", inviteAPIBase, body)
}

// ===============================================
// TOOL: send_email
// ===============================================

// SendEmailInput represents the input for send_email tool
type SendEmailInput struct {
	To      string `json:"to" jsonschema_description:"Recipient email address"`
	Subject string `json:"subject" jsonschema_description:"Email subject line"`
	Body    string `json:"body" jsonschema_description:"Email body content (plain text or HTML)"`
	IsHTML  bool   `json:"is_html,omitempty" jsonschema_description:"Whether body is HTML (default: false, plain text)"`
}

var SendEmailInputSchema = GenerateSchema[SendEmailInput]()

// SendEmailDefinition defines the send_email tool
var SendEmailDefinition = ToolDefinition{
	Name: "send_email",
	Description: `Send an email on behalf of the user. Use this to communicate with people outside the workspace.

PARAMETERS:
- to: Recipient email address (REQUIRED)
- subject: Email subject (REQUIRED)
- body: Email body content (REQUIRED)
- is_html: Set to true for HTML emails (optional, default: plain text)

EXAMPLES:
  send_email(to="alice@example.com", subject="Meeting notes", body="Here are the notes from today...")
  send_email(to="team@company.com", subject="Weekly Report", body="<h1>Report</h1><p>...</p>", is_html=true)`,
	InputSchema: SendEmailInputSchema,
	Function:    SendEmail,
}

// SendEmail implements the send_email tool
func SendEmail(input json.RawMessage) (string, error) {
	var params SendEmailInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.To == "" {
		return "", fmt.Errorf("to is required")
	}
	if params.Subject == "" {
		return "", fmt.Errorf("subject is required")
	}
	if params.Body == "" {
		return "", fmt.Errorf("body is required")
	}

	emailAPIBase := apiURL("/api/email/send")
	body := map[string]any{
		"to":      params.To,
		"subject": params.Subject,
		"body":    params.Body,
		"is_html": params.IsHTML,
	}
	return agentsAPICall("POST", emailAPIBase, body)
}

// ===============================================
// TOOL 11: gateway
// ===============================================

// GatewayInput represents the input for gateway tool
type GatewayInput struct {
	Action string `json:"action,omitempty" jsonschema_description:"Action to perform: 'status' (default), 'health', 'stats'"`
}

var GatewayInputSchema = GenerateSchema[GatewayInput]()

// GatewayDefinition defines the gateway tool
var GatewayDefinition = ToolDefinition{
	Name: "gateway",
	Description: `Introspect gateway state and health.

Use this to:
- Check system health and uptime
- Get queue depths and active runs
- Monitor system resources
- Debug gateway issues

Actions:
- status: Overall gateway status (default)
- health: Health check with detailed metrics
- stats: Statistics about runs, sessions, queues`,
	InputSchema: GatewayInputSchema,
	Function:    Gateway,
}

// Gateway provides gateway introspection
func Gateway(input json.RawMessage) (string, error) {
	var params GatewayInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	action := params.Action
	if action == "" {
		action = "status"
	}

	// TODO: Wire with actual gateway components (QueueManager, RunTracker, etc.)
	// For now, return basic runtime info
	result := map[string]any{
		"action": action,
		"status": "running",
		"note":   "Full gateway introspection requires runtime wiring",
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}
