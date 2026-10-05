package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// An edit frame names every file its patch touches, not just the first.
func TestPatchFrameNamesEveryFile(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: cmd/cli/cmd/tui.go\n@@\n-a\n+b\n*** Update File: cmd/cli/cmd/tui_remote_test.go\n@@\n-c\n+d\n*** End Patch\n"
	in, _ := json.Marshal(map[string]string{"input": patch})
	got := viewFor("apply_patch").Label(string(in), 200)
	if !strings.Contains(got, "cmd/cli/cmd/tui.go") || !strings.Contains(got, "cmd/cli/cmd/tui_remote_test.go") {
		t.Fatalf("both files must be named: %q", got)
	}
	var many strings.Builder
	many.WriteString("*** Begin Patch\n")
	for _, f := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		many.WriteString("*** Add File: " + f + "\n+x\n")
	}
	many.WriteString("*** End Patch\n")
	in, _ = json.Marshal(map[string]string{"input": many.String()})
	if got := viewFor("apply_patch").Label(string(in), 200); !strings.Contains(got, "a.go, b.go, c.go, +2 more") {
		t.Fatalf("past three files the label counts the rest: %q", got)
	}
}
