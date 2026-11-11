package gateway

import "testing"

// zsh words "command not found" its own way; both shells' forms are caught.
func TestShellRanApplyPatchReadsZshToo(t *testing.T) {
	for _, out := range []string{
		"Output: bash: line 1: apply_patch: command not found",
		"Output: zsh:2: command not found: apply_patch",
		"zsh: command not found: apply_patch",
	} {
		if !shellRanApplyPatch.MatchString(out) {
			t.Errorf("missed %q", out)
		}
	}
	if shellRanApplyPatch.MatchString("zsh: command not found: applyx") {
		t.Error("matched another command")
	}
}
