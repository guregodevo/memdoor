package ui

import (
	"memdoor/pkg/filelist"
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
	inputMaxLines   = 8
	mentionMaxMatch = 200
	fileListTTL     = 30 * time.Second
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
// loadFileList and matchFiles are pkg/filelist's: the same walk and ranking
// run in the gateway for a window whose project is on another machine.
func loadFileList(root string) []string { return filelist.List(root) }

func matchFiles(files []string, query string, limit int) []string {
	return filelist.Match(files, query, limit)
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
		if files, err := m.fsOf().List(SessionDir()); err == nil {
			m.fileList = files
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
