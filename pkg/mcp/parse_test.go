package mcp

import (
	"strings"
	"testing"
)

// What a person pastes: the transport follows from it, and a name is
// suggested.
func TestParseAdd(t *testing.T) {
	for in, want := range map[string]string{
		"https://mcp.linear.app/mcp":                                                   "linear http https://mcp.linear.app/mcp",
		"https://api.githubcopilot.com/mcp/":                                           "githubcopilot http https://api.githubcopilot.com/mcp/",
		"npx -y @modelcontextprotocol/server-everything":                               "everything stdio npx -y @modelcontextprotocol/server-everything",
		"uvx mcp-server-git --repository '/path with spaces/repo'":                     "git stdio uvx mcp-server-git --repository|/path with spaces/repo",
		"python3 ./tools/my_server.py":                                                 "my_server stdio python3 ./tools/my_server.py",
		"GITHUB_TOKEN=ghp_x npx -y @modelcontextprotocol/server-github@1":              "github stdio npx -y @modelcontextprotocol/server-github@1 [GITHUB_TOKEN]",
		`{"command":"npx","args":["-y","@upstash/context7-mcp"]}`:                      "context7 stdio npx -y @upstash/context7-mcp",
		`{"mcpServers":{"sentry":{"type":"http","url":"https://mcp.sentry.dev/mcp"}}}`: "sentry http https://mcp.sentry.dev/mcp",
	} {
		got, err := ParseAdd(in)
		if err != nil || len(got) != 1 {
			t.Errorf("%s: %+v, %v", in, got, err)
			continue
		}
		g := got[0]
		desc := g.Name + " "
		if g.Entry.URL != "" {
			desc += "http " + g.Entry.URL
		} else {
			args := strings.Join(g.Entry.Args, " ")
			if strings.Contains(in, "spaces") {
				args = strings.Join(g.Entry.Args[:len(g.Entry.Args)-1], " ") + "|" + g.Entry.Args[len(g.Entry.Args)-1]
			}
			desc += "stdio " + g.Entry.Command + " " + args
			for k := range g.Entry.Env {
				desc += " [" + k + "]"
			}
		}
		if desc != want {
			t.Errorf("%s:\n got %s\nwant %s", in, desc, want)
		}
	}
	if _, err := ParseAdd(`uvx "unclosed`); err == nil {
		t.Error("an unclosed quote must be refused")
	}
	two, err := ParseAdd(`{"mcpServers":{"a":{"command":"x"},"b":{"url":"https://b.example/mcp"}}}`)
	if err != nil || len(two) != 2 || two[0].Name != "a" || two[1].Name != "b" {
		t.Errorf("several servers: %+v %v", two, err)
	}
}
