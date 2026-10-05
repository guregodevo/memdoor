package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Named is a server parsed from what the person typed or pasted, with the
// name it suggests.
type Named struct {
	Name  string
	Entry Entry
}

// ParseAdd reads what the person gave /mcp add: a URL (an HTTP server), a
// .mcp.json snippet (one server or several), or a command line (a local
// server, quotes respected, leading NAME=value pairs its environment).
// The transport follows from it; nobody is asked.
func ParseAdd(input string) ([]Named, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil, fmt.Errorf("give a URL, a command, or a .mcp.json snippet")
	}
	if strings.HasPrefix(s, "{") {
		return parseSnippet(s)
	}
	if u, err := url.Parse(s); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && !strings.ContainsAny(s, " \t") {
		return []Named{{Name: nameFromURL(u), Entry: Entry{Type: "http", URL: s}}}, nil
	}
	words, err := shellWords(s)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for len(words) > 0 && envAssign.MatchString(words[0]) {
		k, v, _ := strings.Cut(words[0], "=")
		env[k] = v
		words = words[1:]
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("no command after the environment variables")
	}
	e := Entry{Command: words[0], Args: words[1:]}
	if len(env) > 0 {
		e.Env = env
	}
	return []Named{{Name: nameFromCommand(words), Entry: e}}, nil
}

var envAssign = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// parseSnippet reads {"mcpServers":{...}}, {"name":{...}} or a bare entry.
func parseSnippet(s string) ([]Named, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &top); err != nil {
		return nil, fmt.Errorf("that is not valid JSON: %w", err)
	}
	if raw, ok := top["mcpServers"]; ok {
		top = map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &top); err != nil {
			return nil, fmt.Errorf("mcpServers: %w", err)
		}
	} else if _, isEntry := top["command"]; isEntry {
		top = map[string]json.RawMessage{"": json.RawMessage(s)}
	} else if _, isEntry := top["url"]; isEntry {
		top = map[string]json.RawMessage{"": json.RawMessage(s)}
	}
	var out []Named
	for name, raw := range top {
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
		if (e.Command == "") == (e.URL == "") {
			return nil, fmt.Errorf("server %q needs a command or a URL", name)
		}
		if name == "" {
			if e.URL != "" {
				if u, err := url.Parse(e.URL); err == nil {
					name = nameFromURL(u)
				}
			} else {
				name = nameFromCommand(append([]string{e.Command}, e.Args...))
			}
		}
		out = append(out, Named{Name: name, Entry: e})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// shellWords splits a command line the way a shell would: quotes group,
// backslashes escape.
func shellWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	esc := false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && quote != '\'':
			esc, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("an unclosed %c quote", quote)
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

var nameJunk = regexp.MustCompile(`[^a-z0-9_-]+`)

// cleanName turns a hint into a server name, or "".
func cleanName(s string) string {
	s = nameJunk.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	for _, cut := range []string{"mcp-server-", "server-", "mcp-"} {
		s = strings.TrimPrefix(s, cut)
	}
	for _, cut := range []string{"-mcp-server", "-server", "-mcp"} {
		s = strings.TrimSuffix(s, cut)
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return strings.Trim(s, "-")
}

// nameFromURL: mcp.linear.app → linear, api.githubcopilot.com/mcp → githubcopilot.
func nameFromURL(u *url.URL) string {
	parts := strings.Split(u.Hostname(), ".")
	for _, p := range parts {
		switch p {
		case "mcp", "api", "www", "app", "server", "com", "io", "ai", "dev", "net", "org", "localhost":
			continue
		}
		if n := cleanName(p); n != "" {
			return n
		}
	}
	if n := cleanName(u.Hostname()); n != "" {
		return n
	}
	return "server"
}

// nameFromCommand: the package or script the command runs —
// npx -y @modelcontextprotocol/server-everything → everything,
// uvx mcp-server-git → git, python3 ./tools/my_server.py → my_server.
func nameFromCommand(words []string) string {
	runners := map[string]bool{"npx": true, "bunx": true, "pnpx": true, "uvx": true, "pipx": true, "uv": true,
		"node": true, "python": true, "python3": true, "deno": true, "bun": true, "docker": true, "go": true}
	cand := words[0]
	if runners[pathBase(words[0])] {
		for _, w := range words[1:] {
			if strings.HasPrefix(w, "-") || w == "run" || w == "exec" || w == "tool" {
				continue
			}
			cand = w
			break
		}
	}
	cand = pathBase(cand)
	if i := strings.LastIndex(cand, "@"); i > 0 {
		cand = cand[:i] // a version: pkg@1.2
	}
	cand = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(cand, ".py"), ".js"), ".ts")
	if n := cleanName(cand); n != "" {
		return n
	}
	return "server"
}

func pathBase(s string) string {
	s = strings.TrimSuffix(s, "/")
	if i := strings.LastIndexAny(s, "/\\"); i >= 0 {
		return s[i+1:]
	}
	return s
}
