package ui

import (
	"strings"
	"testing"
)

func TestListedSkillsIsCodingOnly(t *testing.T) {
	// chrome stays: the coder drives a browser to check a page it changed.
	all := []string{"review", "chrome", "documentary", "clipping", "scraping",
		"hello-world", "system-prompts", "quick-test", "refactoring"}
	got := strings.Join(ListedSkills(all), " ")
	if got != "review chrome quick-test refactoring" {
		t.Fatalf("menu = %q", got)
	}

}
