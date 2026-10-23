// Package filelist is what the window shows of a project directory: the
// files and folders to mention or open, ranked for a query, and a file's
// preview or diff. It runs where the directory is: in the window when the
// gateway is on this machine, in the gateway when the window is on a laptop
// and the project on a box that stays on (GET /api/fs). One walk, one
// ranking, one preview, so both doors show the same thing.
package filelist

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// Max is the most entries a listing holds; MaxDepth how deep it looks.
	Max      = 20000
	MaxDepth = 6
	// PreviewBytes caps what a preview reads; PreviewLines how many lines it
	// keeps before saying how many more there are.
	PreviewBytes = 512 * 1024
	PreviewLines = 160
)

// SkippedDirs are never walked: build output and dependency trees.
var SkippedDirs = map[string]bool{"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true, "__pycache__": true, ".venv": true, ".git": true}

// List walks root for files and folders, shallow first, relative to root
// with "/" separators; a folder ends in "/". Hidden directories and
// SkippedDirs are left out; the walk stops at Max entries or MaxDepth levels.
func List(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil || rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if SkippedDirs[name] || strings.HasPrefix(name, ".") || strings.Count(rel, string(filepath.Separator)) >= MaxDepth {
				return filepath.SkipDir
			}
			if len(out) >= Max {
				return filepath.SkipAll
			}
			out = append(out, filepath.ToSlash(rel)+"/")
			return nil
		}
		if len(out) >= Max {
			return filepath.SkipAll
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/")
		if di != dj {
			return di < dj
		}
		return out[i] < out[j]
	})
	return out
}

// Match ranks the files for a query: a basename that starts with it first,
// then paths that start with it, then paths that contain it, shorter first.
// An empty query is the list as it is, capped at limit.
func Match(files []string, query string, limit int) []string {
	if query == "" {
		if len(files) > limit {
			return files[:limit]
		}
		return files
	}
	q := strings.ToLower(query)
	type hit struct {
		path string
		rank int
	}
	var hits []hit
	for _, f := range files {
		lf := strings.ToLower(f)
		base := strings.TrimSuffix(lf, "/") // a folder's name is before its slash
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		switch {
		case strings.HasPrefix(base, q):
			hits = append(hits, hit{f, 0})
		case strings.HasPrefix(lf, q):
			hits = append(hits, hit{f, 1})
		case strings.Contains(lf, q):
			hits = append(hits, hit{f, 2})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return len(hits[i].path) < len(hits[j].path)
	})
	out := make([]string, 0, min(limit, len(hits)))
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.path)
	}
	return out
}

// Preview is the right column of the files panel for one entry under root:
// a folder's entries, a file's numbered lines, or with diff the file's
// uncommitted change (a file git does not know is shown whole, as added).
func Preview(root, rel string, diff bool) []string {
	full := filepath.Join(root, filepath.FromSlash(rel))
	if strings.HasSuffix(rel, "/") {
		entries, err := os.ReadDir(full)
		if err != nil {
			return []string{err.Error()}
		}
		var out []string
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			out = append(out, name)
		}
		return out
	}
	if diff {
		r := filepath.FromSlash(rel)
		out, _ := exec.Command("git", "-C", root, "diff", "--", r).CombinedOutput()
		if strings.TrimSpace(string(out)) == "" {
			out, _ = exec.Command("git", "-C", root, "diff", "--cached", "--", r).CombinedOutput()
		}
		if strings.TrimSpace(string(out)) == "" {
			if exec.Command("git", "-C", root, "ls-files", "--error-unmatch", r).Run() != nil {
				out, _ = exec.Command("git", "-C", root, "diff", "--no-index", "--", os.DevNull, r).CombinedOutput()
			}
		}
		if strings.TrimSpace(string(out)) == "" {
			return []string{"(no uncommitted change in this file)"}
		}
		return strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return []string{err.Error()}
	}
	if len(b) > PreviewBytes {
		b = b[:PreviewBytes]
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > PreviewLines*4 {
		lines = append(lines[:PreviewLines*4], fmt.Sprintf("… %d more lines", len(lines)-PreviewLines*4))
	}
	for i, l := range lines {
		lines[i] = fmt.Sprintf("%4d  %s", i+1, strings.ReplaceAll(l, "\t", "    "))
	}
	return lines
}

// Stat says whether path is a directory on this machine, with "~" expanded
// to this user's home and the path cleaned to an absolute one.
func Stat(path string) (abs string, isDir bool, exists bool) {
	p := strings.TrimSpace(path)
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	if !filepath.IsAbs(p) {
		if a, err := filepath.Abs(p); err == nil {
			p = a
		}
	}
	p = filepath.Clean(p)
	fi, err := os.Stat(p)
	if err != nil {
		return p, false, false
	}
	return p, fi.IsDir(), true
}
