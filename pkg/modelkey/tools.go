package modelkey

import (
	"os"
	"path/filepath"
	"strings"
)

// ToolStore is a file where another coding tool keeps a person's model keys.
// Memdoor reads one only after the person said yes (the maintainer, 2026-10-31): it is
// another program's secret.
type ToolStore struct {
	Tool string // the tool, as a person names it
	Path string // the file
}

// ToolStores is where other coding tools keep keys under home, the ones that
// exist. A file that is not there, or holds no key of a known shape, offers
// nothing: the list can be generous.
func ToolStores(home string) []ToolStore {
	all := []ToolStore{
		{"Codex", filepath.Join(home, ".codex", "auth.json")},
		{"OpenCode", filepath.Join(home, ".local", "share", "opencode", "auth.json")},
		{"Claude Code", filepath.Join(home, ".claude.json")}, // an API key only: a subscription's token is no key
		{"Hermes", filepath.Join(home, ".hermes", ".env")},
		{"Goose", filepath.Join(home, ".config", "goose", "secrets.yaml")},
		{"Crush", filepath.Join(home, ".local", "share", "crush", "crush.json")},
		{"Gemini CLI", filepath.Join(home, ".gemini", ".env")},
		{"Continue", filepath.Join(home, ".continue", "config.yaml")},
		{"Continue", filepath.Join(home, ".continue", "config.json")},
		{"OpenClaw", filepath.Join(home, ".openclaw", "openclaw.json")},
		{"OpenClaw", filepath.Join(home, ".openclaw", ".env")},
		{"llm", filepath.Join(home, "Library", "Application Support", "io.datasette.llm", "keys.json")},
		{"llm", filepath.Join(home, ".config", "io.datasette.llm", "keys.json")},
		{"aider", filepath.Join(home, ".aider.conf.yml")},
		{"aider", filepath.Join(home, ".aider", "oauth-keys.env")},
	}
	var out []ToolStore
	for _, s := range all {
		if fi, err := os.Stat(s.Path); err == nil && fi.Mode().IsRegular() {
			out = append(out, s)
		}
	}
	return out
}

// toolStoreLimit is the most of a store read: ~/.claude.json carries its
// history and runs to megabytes; anything past this is not a key file.
const toolStoreLimit = 16 << 20

// FindInTools is every model key in the stores given, by value: any string
// in the file shaped like one (JSON, YAML or `provider=key`, whatever the
// tool wrote), each value once. An OAuth token (OpenCode's, Codex's ChatGPT
// sign-in) is no key and is not offered.
func FindInTools(stores []ToolStore, home string) []Found {
	var found []Found
	seen := map[string]bool{}
	for _, s := range stores {
		fi, err := os.Stat(s.Path)
		if err != nil || fi.Size() > toolStoreLimit {
			continue
		}
		b, err := os.ReadFile(s.Path)
		if err != nil {
			continue
		}
		where := s.Tool + " (" + shortHome(s.Path, home) + ")"
		for _, tok := range keyTokens(string(b)) {
			ps := ProvidersOf(tok)
			if ps == nil || seen[tok] {
				continue
			}
			seen[tok] = true
			found = append(found, Found{Providers: ps, File: where, Value: tok})
		}
	}
	return found
}

// keyTokens splits text at every character a key never holds.
func keyTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	})
}

func shortHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(filepath.Separator)) {
		// Slashes, on every OS: a person looking for the file reads "~/.codex/
		// auth.json", not "~\.codex\auth.json".
		return "~/" + filepath.ToSlash(path[len(home)+1:])
	}
	return path
}
