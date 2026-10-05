package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hunks are located forward-only from the previous match, which assumes the
// model emits them in file order. It does not always.
//
// Measured 2026-08-30 on a live coder turn against news_summarizer.py: hunk 1
// deleted main() at the END of the file, advancing the search past it; hunk 2's
// context was `except Exception as exc:` from a function EARLIER in the file.
// The search started past it, and the whole patch failed with "failed to find
// expected lines" — on context that was letter-perfect and right there.
func TestOutOfOrderHunksApply(t *testing.T) {
	orig := `#!/usr/bin/env python3
"""doc"""


def fetch(url):
    try:
        return get(url)
    except Exception as exc:
        print(f"  ! {url} failed: {exc}")
        return None


def main():
    print("starting...")


if __name__ == "__main__":
    main()
`
	// hunk 1 touches the END, hunk 2 the MIDDLE — the order the model used.
	patch := `*** Begin Patch
*** Update File: t.py
@@ def main():
-def main():
-    print("starting...")
+def main():
+    print("started")
@@ except Exception as exc:
     except Exception as exc:
         print(f"  ! {url} failed: {exc}")
         return None
+        # recovered
*** End Patch`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "t.py"), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := runPatch(t, dir, patch, "t.py")
	if err != nil {
		t.Fatalf("out-of-order hunks must still apply, got: %v", err)
	}
	if !strings.Contains(got, `print("started")`) {
		t.Error("the later-in-file hunk was lost")
	}
	if !strings.Contains(got, "# recovered") {
		t.Error("the earlier-in-file hunk was lost — this is the measured failure")
	}
}

// A backwards match must never land inside a span already being replaced: two
// edits spliced over the same lines corrupt both.
func TestBackwardsMatchRefusesAClaimedSpan(t *testing.T) {
	repls := []replacement{{start: 10, oldLen: 5}}
	for _, c := range []struct {
		start, length int
		want          bool
	}{
		{10, 1, true},  // exactly on it
		{12, 1, true},  // inside it
		{8, 4, true},   // overlaps the front
		{14, 3, true},  // overlaps the back
		{5, 5, false},  // ends where it begins
		{15, 3, false}, // starts where it ends
	} {
		if got := overlapsReplacement(repls, c.start, c.length); got != c.want {
			t.Errorf("overlapsReplacement(start=%d len=%d) = %v, want %v", c.start, c.length, got, c.want)
		}
	}
}
