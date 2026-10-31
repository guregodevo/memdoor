package modelkey

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Each tool's own format, read by value: Codex's auth.json, OpenCode's
// (an oauth entry is no key), llm's keys.json, aider's YAML with
// provider=key. A value in two stores is offered once.
func TestFindInTools(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".codex", "auth.json"), `{"OPENAI_API_KEY": "`+fakeOpenAI+`", "tokens": null}`)
	write(t, filepath.Join(home, ".local", "share", "opencode", "auth.json"),
		`{"anthropic": {"type": "oauth", "refresh": "`+fakeOAuth+`"}, "openrouter": {"type": "api", "key": "`+fakeOpenRouter+`"}}`)
	write(t, filepath.Join(home, ".config", "io.datasette.llm", "keys.json"), `{"gemini": "`+fakeGemini+`", "openai": "`+fakeOpenAI+`"}`)
	write(t, filepath.Join(home, ".aider.conf.yml"), "model: x\napi-key:\n  - groq="+fakeGroq+"\n")
	// Claude Code keeps a subscription's OAuth token beside an API key: only
	// the API key is a key.
	write(t, filepath.Join(home, ".claude.json"), `{"numStartups": 9, "oauthAccount": {"accessToken": "`+fakeOAuth+`"}, "primaryApiKey": "`+fakeAnthropic+`"}`)
	// Hermes holds the same OpenRouter key as OpenCode: offered once.
	write(t, filepath.Join(home, ".hermes", ".env"), "OPENROUTER_API_KEY="+fakeOpenRouter+"\n")
	write(t, filepath.Join(home, ".config", "goose", "secrets.yaml"), "XAI_API_KEY: "+fakeXAI+"\n")

	stores := ToolStores(home)
	var tools []string
	for _, s := range stores {
		tools = append(tools, s.Tool)
	}
	if !reflect.DeepEqual(tools, []string{"Codex", "OpenCode", "Claude Code", "Hermes", "Goose", "llm", "aider"}) {
		t.Fatalf("stores found: %v (only the files that exist)", tools)
	}
	var got []string
	for _, k := range FindInTools(stores, home) {
		got = append(got, string(k.Providers[0])+" "+k.Where())
	}
	want := []string{
		"openai Codex (~/.codex/auth.json)",
		"openrouter OpenCode (~/.local/share/opencode/auth.json)",
		"anthropic Claude Code (~/.claude.json)",
		"xai Goose (~/.config/goose/secrets.yaml)",
		"gemini llm (~/.config/io.datasette.llm/keys.json)",
		"groq aider (~/.aider.conf.yml)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("found %v\nwant  %v", got, want)
	}
}

// Negative paths: a store that is a directory, unreadable, too large, or
// holds no key of a known shape offers nothing and never fails the scan.
func TestToolStoresNegativePaths(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex", "auth.json"), 0o700); err != nil { // a directory, not a file
		t.Fatal(err)
	}
	write(t, filepath.Join(home, ".hermes", ".env"), "OPENROUTER_API_KEY=not-a-key\nNOTHING=here\n")
	write(t, filepath.Join(home, ".aider.conf.yml"), "model: x\n")
	unreadable := filepath.Join(home, ".gemini", ".env")
	write(t, unreadable, "GEMINI_API_KEY="+fakeGemini+"\n")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	big := filepath.Join(home, ".claude.json")
	write(t, big, `{"primaryApiKey": "`+fakeAnthropic+`", "pad": "`+strings.Repeat("x", toolStoreLimit)+`"}`)

	stores := ToolStores(home)
	for _, s := range stores {
		if s.Tool == "Codex" {
			t.Fatal("a directory is no store")
		}
	}
	got := FindInTools(stores, home)
	for _, k := range got {
		if k.Value == fakeAnthropic {
			t.Fatal("a file past the limit is not read")
		}
		if k.Value == fakeGemini && os.Geteuid() != 0 {
			t.Fatal("an unreadable file offers nothing")
		}
	}
	if len(got) != 0 && os.Geteuid() != 0 {
		t.Fatalf("no key of a known shape: got %v", got)
	}
}

func TestNoToolStores(t *testing.T) {
	if s := ToolStores(t.TempDir()); s != nil {
		t.Fatalf("an empty home has no store, got %v", s)
	}
}
