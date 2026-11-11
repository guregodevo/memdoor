package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func pathAsk(key, p string) string {
	b, _ := json.Marshal(map[string]string{key: p})
	return string(b)
}

// Different files of one deep project are different asks; the same file read
// again is still the same ask.
func TestDifferentFilesAreNotTheSameAsk(t *testing.T) {
	base := "/Users/dev/work/acme/platform/services/billing/internal/invoice/"
	for _, key := range []string{"path", "file_path", "dir"} {
		earlier := []string{pathAsk(key, base+"render.go"), pathAsk(key, base+"totals.go")}
		if n := nearSameAsks(earlier, pathAsk(key, base+"tax.go")); n != 0 {
			t.Errorf("%s: three different files counted as %d earlier same asks", key, n)
		}
		if n := nearSameAsks(earlier, pathAsk(key, base+"render.go")); n != 1 {
			t.Errorf("%s: the same file again counted %d", key, n)
		}
	}
}

// The loop-breaker names a failure only when a failure tripped it.
func TestALoopOfRepeatsIsNotCalledAFailure(t *testing.T) {
	ok := []ToolExecutionInfo{{Name: "read_file", Output: "x"}}
	if s := loopStop(ok, false); strings.Contains(s, "failing") || !strings.Contains(s, "repeating") {
		t.Errorf("stop after repeats: %q", s)
	}
	if s := loopNudge(ok, false); strings.Contains(s, "failing") {
		t.Errorf("nudge after repeats: %q", s)
	}
	failed := []ToolExecutionInfo{{Name: "bash", Error: "exit status 1"}}
	if s := loopStop(failed, true); !strings.Contains(s, "failing the same way — bash: exit status 1") {
		t.Errorf("stop after failures: %q", s)
	}
}
