package tools

import (
	"testing"
)

// TestGenerateSchema_RequiredFields verifies that GenerateSchema properly
// includes the Required field in the generated schema
func TestGenerateSchema_RequiredFields(t *testing.T) {
	// Test with WriteFileInput which has both path and content as required
	schema := GenerateSchema[WriteFileInput]()

	// Verify Required field is present and not empty
	if schema.Required == nil {
		t.Fatal("Expected Required field to be set, but it was nil")
	}

	// Verify both path and content are in the Required list
	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}

	if !requiredFields["path"] {
		t.Error("Expected 'path' to be in Required fields")
	}

	if !requiredFields["content"] {
		t.Error("Expected 'content' to be in Required fields")
	}

	// Verify we have exactly 2 required fields
	if len(schema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d: %v", len(schema.Required), schema.Required)
	}
}

// TestGenerateSchema_Properties verifies that properties are still generated
func TestGenerateSchema_Properties(t *testing.T) {
	schema := GenerateSchema[WriteFileInput]()

	// Verify Properties field is present
	if schema.Properties == nil {
		t.Fatal("Expected Properties field to be set, but it was nil")
	}

	// Properties is of type 'any', so we just verify it's not nil
	// The actual property validation happens in the API
	t.Logf("Properties generated successfully: %T", schema.Properties)
}

// TestGenerateSchema_OptionalFields verifies that optional fields are NOT
// in the Required list
func TestGenerateSchema_OptionalFields(t *testing.T) {
	// Define a test struct with optional fields (using omitempty)
	type TestInput struct {
		RequiredField string `json:"required_field"`
		OptionalField string `json:"optional_field,omitempty"`
	}

	schema := GenerateSchema[TestInput]()

	// Verify Required field exists
	if schema.Required == nil {
		t.Fatal("Expected Required field to be set")
	}

	// Build map of required fields
	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}

	// Verify required_field is required
	if !requiredFields["required_field"] {
		t.Error("Expected 'required_field' to be in Required fields")
	}

	// Verify optional_field is NOT required
	if requiredFields["optional_field"] {
		t.Error("Expected 'optional_field' to NOT be in Required fields (has omitempty)")
	}
}

// TestWriteFile_MissingContent verifies that write_file properly validates
// the content parameter
func TestWriteFile_MissingContent(t *testing.T) {
	// Simulate the error case: calling write_file with only path
	input := `{"path":"test.txt"}`

	_, err := WriteFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when content is missing, but got nil")
	}

	if err.Error() != "content is required" {
		t.Errorf("Expected error 'content is required', got: %v", err)
	}
}

// TestWriteFile_MissingPath verifies that write_file properly validates
// the path parameter
func TestWriteFile_MissingPath(t *testing.T) {
	input := `{"content":"test content"}`

	_, err := WriteFile([]byte(input))

	if err == nil {
		t.Fatal("Expected error when path is missing, but got nil")
	}

	if err.Error() != "path is required" {
		t.Errorf("Expected error 'path is required', got: %v", err)
	}
}
