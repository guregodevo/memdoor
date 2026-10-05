package gateway

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every host the binary can name is in docs/SECURITY.md, the page a company's
// security reviewer reads (Greg, 2026-10-02: a monitored laptop allows a tool
// only when where its data goes is known). A new host literal anywhere in
// non-test Go — a call, a default, a tool's description — fails this test
// until the page says what it is and when, if ever, it is contacted.
func TestEveryHostTheBinaryNamesIsDocumented(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "docs", "SECURITY.md"))
	if err != nil {
		t.Fatal(err)
	}
	host := regexp.MustCompile(`https?://([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	found := map[string][]string{}
	for _, root := range []string{".", "../cmd", "../pkg", "../tools"} {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, line := range strings.Split(string(src), "\n") {
				code := line
				if i := strings.Index(code, "//"); i >= 0 && !strings.Contains(code[:i], `"`) {
					code = code[:i] // a comment-only line names nothing the binary can reach
				}
				for _, m := range host.FindAllStringSubmatch(code, -1) {
					h := strings.ToLower(m[1])
					if len(found[h]) < 3 {
						found[h] = append(found[h], filepath.Clean(path))
					}
				}
			}
			return nil
		})
	}
	var missing []string
	for h, where := range found {
		if !strings.Contains(string(doc), "`"+h+"`") {
			missing = append(missing, h+" ("+strings.Join(where, ", ")+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("hosts the binary names that docs/SECURITY.md does not list:\n  %s", strings.Join(missing, "\n  "))
	}
}
