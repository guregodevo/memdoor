// Package buildscope derives the Go build/test scope for a set of changed files
// — the affected package(s) — so the coder's verify step compiles just those
// instead of the whole module (LARGE_REPO_DESIGN.md Phase 5). Scoping keeps the
// small model's context clean: it sees only errors from what it touched, not
// unrelated packages.
//
// It falls back to the whole module (./...) whenever scoping would be unsafe or
// unhelpful: non-Go changes, nothing derivable, a path outside the root, or too
// many distinct packages to be a "tight" loop.
package buildscope

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// maxDistinctPackages caps how many packages still count as a scoped (tight)
// build; beyond it, a full build is both simpler and no less noisy.
const maxDistinctPackages = 8

// Scope is a resolved build/test target set.
type Scope struct {
	Patterns []string // go package patterns, e.g. ["./gateway/flow/...", "."]
	Full     bool     // true when this fell back to the whole module (./...)
}

// full returns the whole-module scope.
func full() Scope { return Scope{Patterns: []string{"./..."}, Full: true} }

// Derive returns the build scope for changed (repo-relative or absolute paths),
// resolved against root. See package doc for fallback conditions.
func Derive(root string, changed []string) Scope {
	dirs := map[string]bool{}
	sawGo := false
	for _, f := range changed {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		sawGo = true
		rel, under := relativeTo(root, f)
		if !under {
			return full() // outside the root — don't guess a scope
		}
		dirs[path0(path.Dir(rel))] = true
	}
	if !sawGo || len(dirs) == 0 || len(dirs) > maxDistinctPackages {
		return full()
	}
	pats := make([]string, 0, len(dirs))
	for d := range dirs {
		if d == "." {
			pats = append(pats, ".")
		} else {
			pats = append(pats, "./"+d+"/...")
		}
	}
	sort.Strings(pats)
	return Scope{Patterns: pats, Full: false}
}

// path0 normalizes an empty dir ("") to ".".
func path0(d string) string {
	if d == "" {
		return "."
	}
	return d
}

// BuildArgs returns the `go` args to build this scope, e.g. ["build","./x/..."].
func (s Scope) BuildArgs() []string {
	return append([]string{"build"}, s.Patterns...)
}

// relativeTo returns f as a slash path under root, and whether it is under root
// at all.
//
// A changed path is what git prints, so it holds slashes on every OS — and
// "/repo/x.go" has no volume name, which on Windows filepath.IsAbs calls
// relative. So the leading slash decides, not the platform, and the arithmetic
// is done on slash paths (a Windows path is slash-normalized first).
func relativeTo(root, f string) (string, bool) {
	r := strings.TrimSuffix(filepath.ToSlash(root), "/")
	p := filepath.ToSlash(f)
	if !strings.HasPrefix(p, "/") && !filepath.IsAbs(f) {
		if p == ".." || strings.HasPrefix(p, "../") {
			return p, false
		}
		return p, true
	}
	if r == "" || !strings.HasPrefix(p, r+"/") {
		return p, false
	}
	return strings.TrimPrefix(p, r+"/"), true
}
