package tools

import "testing"

func TestPatchHelpers(t *testing.T) {
	// Test collapseWS
	if got := collapseWS("   "); got != "" {
		t.Errorf("collapseWS = %v", got)
	}
	if got := collapseWS("a b c"); got != "a b c" {
		t.Errorf("collapseWS = %v", got)
	}

	// Test declName
	if got := declName("type Config struct {"); got != "Config" {
		t.Errorf("declName = %v", got)
	}
	// declName matches only func/type declarations — a var line yields "".
	if got := declName("var myVar int"); got != "" {
		t.Errorf("declName = %v", got)
	}

	// Test alnumOnly
	if got := alnumOnly("abc123"); got != "abc123" {
		t.Errorf("alnumOnly = %v", got)
	}
	if got := alnumOnly("abc 123!"); got != "abc123" {
		t.Errorf("alnumOnly = %v", got)
	}
}
