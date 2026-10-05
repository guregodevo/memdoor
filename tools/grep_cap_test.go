package tools

import (
	"strings"
	"testing"
)

func TestCapGrepOutput(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		b.WriteString("pkg/x/file.go:12:  some matching line of code here\n")
	}
	big := strings.TrimRight(b.String(), "\n")
	out := capGrepOutput(big, 2000, 0)
	if len(out) > grepOutputMax+300 || !strings.Contains(out, "2000 matches") || !strings.Contains(out, "use jgrep") {
		t.Fatalf("len %d tail %q", len(out), out[len(out)-200:])
	}
	if !strings.HasSuffix(strings.Split(out, "\n…[")[0], "code here") {
		t.Fatal("cut mid-line")
	}
	if capGrepOutput(big, 2000, 50) != big {
		t.Fatal("head_limit set: must not cap")
	}
	if capGrepOutput("a:1:x", 1, 0) != "a:1:x" {
		t.Fatal("small output changed")
	}
}
