package gateway

import "testing"

// A WORKSPACE NAME BECOMES AN ID WITHOUT A LIBRARY. The transliteration
// dependency came in with a deleted feature for page slugs and left with it
// (2026-09-27); a workspace id only has to be a stable, URL-safe key.
func TestSlugifyWorkspace(t *testing.T) {
	for in, want := range map[string]string{
		"My Project":        "my-project",
		"  spaced   out  ":  "spaced-out",
		"a|b / c":           "a-b-c",
		"Café Noir":         "café-noir", // letters in any script are kept
		"---":               "untitled",  // never an empty id
		"":                  "untitled",
		"already-a-slug":    "already-a-slug",
		"Project 2026 (v2)": "project-2026-v2",
	} {
		if got := slugifyWorkspace(in); got != want {
			t.Errorf("slugifyWorkspace(%q) = %q, want %q", in, got, want)
		}
	}
}
