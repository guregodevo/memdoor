package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// model_links: every path and URL the TUI shows is clickable.
//
// Greg, 2026-09-02: "the links are always clickable, the path as well". A
// person reads "/Users/…/main.go:42" in a tool frame and wants to open it,
// not retype it. OSC 8 hyperlinks make that a cmd-click in Ghostty, kitty,
// WezTerm, iTerm2; terminals that ignore OSC 8 show the plain text. The
// escapes are applied AFTER wrapping, so a wrap can never split one.

// linkTargets finds http(s) URLs and filesystem paths. A path starts with "~/"
// or "/" and — for "/" — carries at least one more slash, so a slash command
// ("/model", "/exit") stays plain text.
var linkTargets = regexp.MustCompile(`(https?://[^\s<>"')\]]+)|((?:~|/)[\w.\-+@%/]+)`)

// bareFiles finds a file name the model wrote without a directory —
// "REVIEW.md", "notes.txt", "out.json" — when it is not
// already part of a path (the character before it is not "/" or "~"). An
// agent names its outputs relative to the session directory (Greg,
// 2026-09-03: "the files should be clickable"); such a name links only when
// the file exists there, so "app.js" in prose about code stays plain.
var bareFiles = regexp.MustCompile(`(^|[\s(\[{"'` + "`" + `,;:])([\w][\w.\-]*\.(?:gif|jpg|jpeg|png|webp|txt|json|md|csv|pdf))\b`)

// linkifyLine wraps each URL and path in an OSC 8 hyperlink, then each bare
// file name that exists in the session directory. A line already carrying a
// link (a markdown citation) is left alone: re-linking text inside a link
// corrupts it.
func linkifyLine(line string) string {
	if strings.Contains(line, "\x1b]8;;") {
		return line
	}
	if strings.ContainsAny(line, "/~") {
		line = linkifyPaths(line)
	}
	return linkifyBareFiles(line)
}

// linkifyBareFiles links file names relative to the process directory —
// the session's directory, which /dir re-roots — when they exist.
func linkifyBareFiles(line string) string {
	if !strings.Contains(line, ".") {
		return line
	}
	cwd, err := os.Getwd()
	if err != nil {
		return line
	}
	return bareFiles.ReplaceAllStringFunc(line, func(match string) string {
		sub := bareFiles.FindStringSubmatch(match)
		prefix, name := sub[1], sub[2]
		full := filepath.Join(cwd, name)
		if st, err := os.Stat(full); err != nil || st.IsDir() {
			return match
		}
		return prefix + hyperlink("file://"+full, name)
	})
}

func linkifyPaths(line string) string {
	return linkTargets.ReplaceAllStringFunc(line, func(match string) string {
		// Trailing punctuation belongs to the sentence, not the target.
		trimmed := strings.TrimRight(match, ".,;:)]'\"")
		tail := match[len(trimmed):]
		href := linkHref(trimmed)
		if href == "" {
			return match
		}
		return hyperlink(href, trimmed) + tail
	})
}

// hyperlink is a PLAIN OSC 8 — never the tmux DCS passthrough osc8 uses for
// citations. Passthrough hands the bytes to the outer terminal, out of tmux's
// own screen grid: live 2026-09-02, "Session directory is now  —" rendered
// with the path missing. tmux ≥ 3.4 draws plain OSC 8 as a hyperlink itself;
// older tmux drops the escape and keeps the text. Either way the path shows.
func hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// linkHref turns a match into a URL, or "" when it is not worth linking.
func linkHref(target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	if strings.HasPrefix(target, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return "file://" + filepath.Join(home, target[2:])
	}
	// "/x" alone is a slash command or a bare root segment; a path has depth.
	if strings.Count(target, "/") < 2 || strings.Trim(target, "/") == "" {
		return ""
	}
	return "file://" + target
}
