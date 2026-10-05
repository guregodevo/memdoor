package buildscope

import (
	"reflect"
	"testing"
)

func TestSinglePackage(t *testing.T) {
	s := Derive("/repo", []string{"gateway/flow/registry.go"})
	if s.Full {
		t.Fatal("single Go file should scope, not fall back")
	}
	if !reflect.DeepEqual(s.Patterns, []string{"./gateway/flow/..."}) {
		t.Errorf("patterns = %v", s.Patterns)
	}
	if !reflect.DeepEqual(s.BuildArgs(), []string{"build", "./gateway/flow/..."}) {
		t.Errorf("build args = %v", s.BuildArgs())
	}
}

func TestSameDirCollapses(t *testing.T) {
	s := Derive("/repo", []string{"pkg/x/a.go", "pkg/x/b.go"})
	if !reflect.DeepEqual(s.Patterns, []string{"./pkg/x/..."}) {
		t.Errorf("same-dir files should collapse to one pattern: %v", s.Patterns)
	}
}

func TestMultipleDirsSortedDeterministic(t *testing.T) {
	s := Derive("/repo", []string{"pkg/z/z.go", "pkg/a/a.go"})
	want := []string{"./pkg/a/...", "./pkg/z/..."}
	if !reflect.DeepEqual(s.Patterns, want) {
		t.Errorf("patterns = %v, want %v (sorted)", s.Patterns, want)
	}
}

func TestRootFileScopesRootPackage(t *testing.T) {
	s := Derive("/repo", []string{"main.go"})
	if s.Full || !reflect.DeepEqual(s.Patterns, []string{"."}) {
		t.Errorf("root file should scope to \".\", got %+v", s)
	}
}

func TestNonGoFallsBackToFull(t *testing.T) {
	for _, changed := range [][]string{
		{"README.md"},
		{"web/index.html", "Makefile"},
		nil,
	} {
		s := Derive("/repo", changed)
		if !s.Full || !reflect.DeepEqual(s.Patterns, []string{"./..."}) {
			t.Errorf("changed=%v should be Full ./..., got %+v", changed, s)
		}
	}
}

func TestTooManyPackagesFallsBack(t *testing.T) {
	var many []string
	for _, d := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		many = append(many, "pkg/"+d+"/x.go")
	}
	if s := Derive("/repo", many); !s.Full {
		t.Errorf("9 distinct packages should fall back to full, got %+v", s)
	}
}

func TestAbsolutePathsResolvedAgainstRoot(t *testing.T) {
	s := Derive("/repo", []string{"/repo/gateway/flow/registry.go"})
	if s.Full || !reflect.DeepEqual(s.Patterns, []string{"./gateway/flow/..."}) {
		t.Errorf("absolute path not resolved: %+v", s)
	}
}

func TestOutsideRootFallsBack(t *testing.T) {
	s := Derive("/repo", []string{"/other/place/x.go"})
	if !s.Full {
		t.Errorf("path outside root should fall back to full, got %+v", s)
	}
}

func TestMixedGoAndNonGoScopesOnGo(t *testing.T) {
	// A doc change alongside a Go change should not force a full build.
	s := Derive("/repo", []string{"pkg/x/a.go", "docs/readme.md"})
	if s.Full || !reflect.DeepEqual(s.Patterns, []string{"./pkg/x/..."}) {
		t.Errorf("mixed changes should scope on the Go file: %+v", s)
	}
}
