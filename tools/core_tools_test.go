package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===============================================
// EDIT_FILE TOOL TESTS
// ===============================================

// TestEditFile_Success verifies that edit_file successfully replaces exact strings
func TestEditFile_Success(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	originalContent := `package main

func example() {
    return true
}`

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test edit
	input := `{
		"file_path": "` + testFile + `",
		"old_string": "    return true",
		"new_string": "    return false"
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	// Verify result message
	if !strings.Contains(result, "Updated") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedContent := `package main

func example() {
    return false
}`

	if string(content) != expectedContent {
		t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expectedContent, string(content))
	}
}

// TestEditFile_MissingFilePath verifies validation of file_path parameter
func TestEditFile_MissingFilePath(t *testing.T) {
	input := `{
		"old_string": "foo",
		"new_string": "bar"
	}`

	_, err := EditFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when file_path is missing, but got nil")
	}

	if err.Error() != "file_path is required" {
		t.Errorf("Expected error 'file_path is required', got: %v", err)
	}
}

// A missing/empty old_string is no longer an error — in the create+edit
// consolidation it means "overwrite the whole file with new_string". Exercises
// that on an EXISTING file (the create-when-missing case is covered separately).
func TestEditFile_EmptyOldStringOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"new_string": "bar"
	}`

	if _, err := EditFile([]byte(input)); err != nil {
		t.Fatalf("empty old_string should overwrite the file, got error: %v", err)
	}
	b, _ := os.ReadFile(testFile)
	if string(b) != "bar" {
		t.Errorf("overwrite mismatch: got %q want %q", string(b), "bar")
	}
}

// old_string == new_string is still rejected on the surgical-edit path. The file
// must EXIST with the anchor to reach that path — the create/overwrite path
// (missing file, or empty old_string) intentionally skips the check.
func TestEditFile_SameOldAndNew(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("foo"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": "foo",
		"new_string": "foo"
	}`

	_, err := EditFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when old_string equals new_string, but got nil")
	}

	if err.Error() != "old_string and new_string must be different" {
		t.Errorf("Expected error 'old_string and new_string must be different', got: %v", err)
	}
}

// edit_file is the single create+edit tool: editing a path that doesn't exist
// yet CREATES it with new_string (so a small model needn't juggle a separate
// write_file), rather than erroring. Updated from the pre-consolidation contract
// where a missing file was a "failed to read file" error.
func TestEditFile_CreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "created.txt")
	input := `{"file_path": "` + fp + `", "old_string": "", "new_string": "hello\n"}`

	if _, err := EditFile([]byte(input)); err != nil {
		t.Fatalf("edit_file should CREATE a missing file, got error: %v", err)
	}
	b, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("expected created file, read error: %v", err)
	}
	if string(b) != "hello\n" {
		t.Errorf("created content mismatch: got %q want %q", string(b), "hello\n")
	}
}

// TestEditFile_StringNotFound verifies error when old_string doesn't exist in file
func TestEditFile_StringNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("Hello world"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": "nonexistent",
		"new_string": "replacement"
	}`

	_, err = EditFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when old_string not found, but got nil")
	}

	expectedMsg := "old_string not found in file"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}

	// Verify helpful guidance is included
	if !strings.Contains(err.Error(), "Read the file first") {
		t.Error("Expected error to include guidance about reading file first")
	}
}

// TestEditFile_MultipleMatches verifies error when old_string appears multiple times
func TestEditFile_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "foo bar foo baz"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": "foo",
		"new_string": "qux"
	}`

	_, err = EditFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when old_string appears multiple times, but got nil")
	}

	expectedMsg := "old_string appears 2 times in file"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}

	// Verify suggestion for search_replace is included
	if !strings.Contains(err.Error(), "search_replace") {
		t.Error("Expected error to suggest search_replace tool")
	}
}

// TestEditFile_PreservesIndentation verifies that tabs and spaces are preserved exactly
func TestEditFile_PreservesIndentation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Mix of tabs and spaces (realistic Go code with tabs)
	originalContent := "func main() {\n\treturn true\n}"

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use exact indentation with tab character
	input := `{
		"file_path": "` + testFile + `",
		"old_string": "\treturn true",
		"new_string": "\treturn false"
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	if !strings.Contains(result, "Updated") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedContent := "func main() {\n\treturn false\n}"
	if string(content) != expectedContent {
		t.Errorf("Indentation not preserved.\nExpected:\n%q\nGot:\n%q", expectedContent, string(content))
	}
}

// TestEditFile_MultilineReplacement verifies replacing multiline strings
func TestEditFile_MultilineReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	originalContent := `Line 1
Line 2
Line 3
Line 4`

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": "Line 2\nLine 3",
		"new_string": "New Line 2\nNew Line 3"
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	if !strings.Contains(result, "Updated") {
		t.Errorf("Expected success message, got: %s", result)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedContent := `Line 1
New Line 2
New Line 3
Line 4`

	if string(content) != expectedContent {
		t.Errorf("Multiline replacement failed.\nExpected:\n%s\nGot:\n%s", expectedContent, string(content))
	}
}

// TestEditFile_EmptyStringReplacement verifies replacing with empty string (deletion)
func TestEditFile_EmptyStringReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	originalContent := "Hello world!"

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": " world",
		"new_string": ""
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	if !strings.Contains(result, "Updated") {
		t.Errorf("Expected success message, got: %s", result)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedContent := "Hello!"
	if string(content) != expectedContent {
		t.Errorf("Empty string replacement failed.\nExpected: %q\nGot: %q", expectedContent, string(content))
	}
}

// TestEditFile_LineNumberReporting verifies that line number is reported
func TestEditFile_LineNumberReporting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	originalContent := `Line 1
Line 2
Line 3
Line 4`

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	input := `{
		"file_path": "` + testFile + `",
		"old_string": "Line 3",
		"new_string": "Modified Line 3"
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	// The diff reports the changed line's number in its gutter (line 3): the old
	// text as "- 3 …" and the new text as "+ 3 …".
	if !strings.Contains(result, "- 3 Line 3") || !strings.Contains(result, "+ 3 Modified Line 3") {
		t.Errorf("Expected line-numbered diff for line 3, got: %s", result)
	}
}

// TestEditFile_WhitespaceMatching verifies exact whitespace matching
func TestEditFile_WhitespaceMatching(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Content with specific whitespace
	originalContent := "foo  bar" // Two spaces

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to match with single space (should fail)
	input := `{
		"file_path": "` + testFile + `",
		"old_string": "foo bar",
		"new_string": "baz"
	}`

	_, err = EditFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when whitespace doesn't match exactly, but got nil")
	}

	if !strings.Contains(err.Error(), "old_string not found") {
		t.Errorf("Expected 'old_string not found' error, got: %v", err)
	}

	// Now try with exact whitespace (two spaces) - should succeed
	input = `{
		"file_path": "` + testFile + `",
		"old_string": "foo  bar",
		"new_string": "baz"
	}`

	result, err := EditFile([]byte(input))
	if err != nil {
		t.Fatalf("EditFile with exact whitespace failed: %v", err)
	}

	if !strings.Contains(result, "Updated") {
		t.Errorf("Expected success message, got: %s", result)
	}
}

// ===============================================
// EDIT_FILE SCHEMA TESTS
// ===============================================

// TestEditFileInputSchema_RequiredFields verifies schema generation
func TestEditFileInputSchema_RequiredFields(t *testing.T) {
	schema := GenerateSchema[EditFileInput]()

	if schema.Required == nil {
		t.Fatal("Expected Required field to be set, but it was nil")
	}

	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}

	// edit_file is create+edit: file_path and new_string are required; old_string
	// is optional — an empty old_string means "create the file / overwrite it with
	// new_string". (Pre-consolidation this was inverted: old_string required,
	// new_string optional for deletion.)
	if !requiredFields["file_path"] {
		t.Error("Expected 'file_path' to be in Required fields")
	}

	if !requiredFields["new_string"] {
		t.Error("Expected 'new_string' to be in Required fields")
	}

	// old_string should NOT be required (has omitempty; empty = create/overwrite).
	if requiredFields["old_string"] {
		t.Error("Expected 'old_string' to NOT be in Required fields (has omitempty)")
	}

	if len(schema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d: %v", len(schema.Required), schema.Required)
	}
}

// TestFormatEditDiff checks the line-numbered +/- diff: the changed line shows as
// a paired -/+ with correct numbers, and surrounding lines are context (marker ' ').
func TestFormatEditDiff(t *testing.T) {
	old := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	nw := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"

	diff := formatEditDiff(old, nw, 2)
	lines := strings.Split(diff, "\n")

	var hasDel, hasAdd bool
	for _, l := range lines {
		if strings.HasPrefix(l, "-") && strings.Contains(l, `println("hi")`) {
			hasDel = true
		}
		if strings.HasPrefix(l, "+") && strings.Contains(l, `println("hello")`) {
			hasAdd = true
		}
	}
	if !hasDel {
		t.Errorf("expected a '-' line for the old text.\n%s", diff)
	}
	if !hasAdd {
		t.Errorf("expected a '+' line for the new text.\n%s", diff)
	}
	// Context line (the func signature) must be present with a space marker.
	if !strings.Contains(diff, "func main() {") {
		t.Errorf("expected context line in diff.\n%s", diff)
	}
	// Unchanged tail/head shouldn't be marked +/-.
	for _, l := range lines {
		if strings.Contains(l, "package main") && (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) {
			t.Errorf("unchanged line marked as change: %q", l)
		}
	}
}
