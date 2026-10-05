package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The input box's two table stakes (docs/roadmap/MUST.md, "TUI table
// stakes", 2026-09-26): a multi-line prompt and @file mentions.
//
// Multi-line: alt+enter or ctrl+j inserts a newline, and a line ending in a
// backslash continues on the next (Claude Code's convention; works in every
// terminal, where shift+enter does not). The box grows with its lines, up to
// inputMaxLines.
//
// Mentions: typing @ opens a picker over the files under the working
// directory — the coder's workdir, sent with every turn — filtered as the
// person types; tab or enter puts the chosen path in the box, still marked
// with @, and the coder's prompt says an @word is a file to read first.

const (
	inputMaxLines    = 8
	mentionMaxMatch  = 200
	fileListMax      = 20000
	fileListMaxDepth = 6
	fileListTTL      = 30 * time.Second
)

// mentionAt finds an @mention being typed at the end of value: the last "@"
// that starts the text or follows whitespace, with no whitespace after it.
// It returns where the "@" is and what follows it.
func mentionAt(value string) (start int, query string, ok bool) {
	at := strings.LastIndex(value, "@")
	if at < 0 {
		return 0, "", false
	}
	if at > 0 {
		prev := value[at-1]
		if prev != ' ' && prev != '\n' && prev != '\t' {
			return 0, "", false
		}
	}
	rest := value[at+1:]
	if strings.ContainsAny(rest, " \n\t") {
		return 0, "", false
	}
	return at, rest, true
}

// insertMention replaces the @query at start with the chosen path.
func insertMention(value string, start int, path string) string {
	return value[:start] + "@" + path + " "
}

// newlineOnBackslash: a line ending in "\" continues instead of sending.
func newlineOnBackslash(value string) (string, bool) {
	if strings.HasSuffix(value, "\\") {
		return value[:len(value)-1] + "\n", true
	}
	return value, false
}

// skippedDirs are never listed: dependencies, builds and VCS internals.
var skippedDirs = map[string]bool{"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true, "__pycache__": true, ".venv": true, ".git": true}

// loadFileList walks root for files and folders to mention, shallow first,
// relative to root with "/" separators; a folder ends in "/". Hidden
// directories and skippedDirs are left out; the walk stops at fileListMax
// entries or fileListMaxDepth levels.
func loadFileList(root string) []string {
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
			if skippedDirs[name] || strings.HasPrefix(name, ".") || strings.Count(rel, string(filepath.Separator)) >= fileListMaxDepth {
				return filepath.SkipDir
			}
			if len(out) >= fileListMax {
				return filepath.SkipAll
			}
			out = append(out, filepath.ToSlash(rel)+"/")
			return nil
		}
		if len(out) >= fileListMax {
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

// matchFiles ranks the files for a query: a basename that starts with it
// first, then paths that contain it, shorter first. An empty query is the
// list as it is.
func matchFiles(files []string, query string, limit int) []string {
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

// refreshMentions opens, filters or closes the @file picker for the input's
// current text. The file list is read from the working directory when the
// picker opens and kept for fileListTTL.
func (m *Model) refreshMentions() {
	start, query, ok := mentionAt(m.input.Value())
	if !ok || m.showAutocomplete {
		m.showFileMentions = false
		return
	}
	if m.fileList == nil || time.Since(m.fileListAt) > fileListTTL {
		if wd, err := os.Getwd(); err == nil {
			m.fileList = loadFileList(wd)
			m.fileListAt = time.Now()
		}
	}
	m.mentionStart = start
	m.fileMatches = matchFiles(m.fileList, query, mentionMaxMatch)
	m.showFileMentions = len(m.fileMatches) > 0
	if m.fileIndex >= len(m.fileMatches) {
		m.fileIndex = 0
	}
}

// growInput sizes the box to its lines, one to inputMaxLines.
func (m *Model) growInput() {
	lines := strings.Count(m.input.Value(), "\n") + 1
	if lines > inputMaxLines {
		lines = inputMaxLines
	}
	if lines < 1 {
		lines = 1
	}
	// A box that was never built (tests construct a bare Model) has no
	// height to change; SetHeight on it dereferences nothing.
	if h := m.input.Height(); h <= 0 || h == lines {
		return
	}
	m.input.SetHeight(lines)
}
