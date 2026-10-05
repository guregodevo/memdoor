package tools

import (
	"strings"
	"testing"
)

// The shear cuts envelope JSON glued onto a written file. A Go file that only
// QUOTES that fragment (apply_patch.go's own doc comment, live 2026-09-30) was
// cut mid-line, stopped parsing, and every later edit to it was refused.
func TestShearParsesGate(t *testing.T) {
	quoting := "package p\n\n// the envelope ends with \"name\":\"apply_patch\"}\nfunc F() {}\n"
	if got := shearEnvelopeJunkLines("p.go", quoting); got != quoting {
		t.Fatalf("a Go file that parses must be left whole; got:\n%s", got)
	}
	glued := "package p\n\nfunc F() {}\",\"name\":\"apply_patch\"}\n"
	got := shearEnvelopeJunkLines("p.go", glued)
	if strings.Contains(got, "apply_patch") || goParseErr("p.go", got) != nil {
		t.Fatalf("junk glued onto a Go file must be cut to a parsing file; got:\n%s", got)
	}
	mod := "module m\n\ngo 1.21\n)\",\"name\":\"apply_patch\n"
	if got := shearEnvelopeJunkLines("go.mod", mod); strings.Contains(got, "apply_patch") {
		t.Fatalf("any other file is sheared as before; got:\n%s", got)
	}
}
