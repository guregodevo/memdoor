package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// The official MCP registry (registry.modelcontextprotocol.io): public, no
// sign-in. A search result becomes a config entry the way the person would
// write it — a hosted server's URL, or the command that runs its package —
// with every value it needs as ${VAR}, so a missing one is named when it
// connects instead of a key being written into a file.

// RegistryURL is the registry's base URL (a var so tests serve their own).
var RegistryURL = "https://registry.modelcontextprotocol.io"

// RegistryServer is one search result, ready to add.
type RegistryServer struct {
	Name        string // the registry's name: io.github.owner/server
	Suggested   string // the name it would get here
	Description string
	Version     string
	Repository  string
	Via         string // hosted, npm, pypi, docker
	Needs       []RegistryVar
	Entry       Entry
}

// RegistryVar is a value a server needs from the environment.
type RegistryVar struct {
	Name        string
	Description string
	Secret      bool
}

type regInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsRequired  bool   `json:"isRequired"`
	IsSecret    bool   `json:"isSecret"`
	Value       string `json:"value"`
	Default     string `json:"default"`
	Type        string `json:"type"`
}

type regServer struct {
	Server struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     string `json:"version"`
		Repository  struct {
			URL string `json:"url"`
		} `json:"repository"`
		Remotes []struct {
			Type    string     `json:"type"`
			URL     string     `json:"url"`
			Headers []regInput `json:"headers"`
		} `json:"remotes"`
		Packages []struct {
			RegistryType     string                `json:"registryType"`
			Identifier       string                `json:"identifier"`
			Version          string                `json:"version"`
			Transport        struct{ Type string } `json:"transport"`
			RuntimeArguments []regInput            `json:"runtimeArguments"`
			PackageArguments []regInput            `json:"packageArguments"`
			EnvVars          []regInput            `json:"environmentVariables"`
		} `json:"packages"`
	} `json:"server"`
	Meta map[string]struct {
		IsLatest bool `json:"isLatest"`
	} `json:"_meta"`
}

// SearchRegistry finds servers whose name matches query, the latest
// version of each, at most limit.
func SearchRegistry(ctx context.Context, hc *http.Client, query string, limit int) ([]RegistryServer, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	// version=latest asks the registry to drop the old versions itself: the
	// same results in a tenth of the bytes (2026-10-01: 12 KB, not 138 KB,
	// because a page of 90 rows held 11 current servers). The isLatest
	// filter below stays for a server the registry does not flag.
	rows := limit
	if rows <= 0 || rows > 100 {
		rows = 100
	}
	u := fmt.Sprintf("%s/v0/servers?version=latest&limit=%d&search=%s",
		RegistryURL, rows, url.QueryEscape(strings.TrimSpace(query)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the MCP registry did not answer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the MCP registry answered HTTP %d", resp.StatusCode)
	}
	var page struct {
		Servers []regServer `json:"servers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&page); err != nil {
		return nil, fmt.Errorf("the MCP registry's answer: %w", err)
	}
	var out []RegistryServer
	seen := map[string]bool{}
	for _, s := range page.Servers {
		if m, ok := s.Meta["io.modelcontextprotocol.registry/official"]; ok && !m.IsLatest {
			continue
		}
		if seen[s.Server.Name] {
			continue
		}
		rs, ok := fromRegistry(s)
		if !ok {
			continue
		}
		seen[s.Server.Name] = true
		out = append(out, rs)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// fromRegistry is a result as an entry: a hosted remote first (nothing to
// install), else a package run over stdio.
func fromRegistry(s regServer) (RegistryServer, bool) {
	sv := s.Server
	rs := RegistryServer{Name: sv.Name, Description: sv.Description, Version: sv.Version, Repository: sv.Repository.URL}
	short := sv.Name
	if i := strings.LastIndex(short, "/"); i >= 0 {
		short = short[i+1:]
	}
	rs.Suggested = cleanName(short)
	if rs.Suggested == "" {
		rs.Suggested = "server"
	}
	for _, kind := range []string{"streamable-http", "sse"} {
		for _, r := range sv.Remotes {
			if r.Type != kind || r.URL == "" {
				continue
			}
			rs.Via, rs.Entry = "hosted", Entry{Type: "http", URL: templateVars(r.URL, nil)}
			if kind == "sse" {
				rs.Entry.Type = "sse"
			}
			for _, h := range r.Headers {
				if !h.IsRequired && h.Value == "" {
					continue
				}
				if rs.Entry.Headers == nil {
					rs.Entry.Headers = map[string]string{}
				}
				if h.Value == "" { // the whole value is the person's: X-Api-Key → ${X_API_KEY}
					rs.Entry.Headers[h.Name] = "${" + envName(h.Name) + "}"
					rs.Needs = append(rs.Needs, RegistryVar{Name: envName(h.Name), Description: h.Description, Secret: h.IsSecret})
					continue
				}
				rs.Entry.Headers[h.Name] = templateVars(h.Value, &rs.Needs) // "Bearer {api_key}"
			}
			return rs, true
		}
	}
	for _, p := range sv.Packages {
		if p.Transport.Type != "" && p.Transport.Type != "stdio" {
			continue
		}
		var cmd string
		var args []string
		switch p.RegistryType {
		case "npm":
			cmd, args = "npx", []string{"-y", withVersion(p.Identifier, "@", p.Version)}
			rs.Via = "npm"
		case "pypi":
			cmd, args = "uvx", []string{withVersion(p.Identifier, "==", p.Version)}
			rs.Via = "pypi"
		case "oci":
			cmd, args = "docker", []string{"run", "-i", "--rm"}
			for _, e := range p.EnvVars {
				if e.IsRequired {
					args = append(args, "-e", e.Name)
				}
			}
			args = append(args, p.Identifier)
			rs.Via = "docker"
		default:
			continue
		}
		for _, a := range p.PackageArguments {
			if a.Type == "positional" && a.Value != "" {
				args = append(args, templateVars(a.Value, &rs.Needs))
			}
		}
		rs.Entry = Entry{Command: cmd, Args: args}
		for _, e := range p.EnvVars {
			if !e.IsRequired {
				continue // the server has its own default
			}
			if rs.Entry.Env == nil {
				rs.Entry.Env = map[string]string{}
			}
			rs.Entry.Env[e.Name] = "${" + e.Name + "}"
			rs.Needs = append(rs.Needs, RegistryVar{Name: e.Name, Description: e.Description, Secret: e.IsSecret})
		}
		return rs, true
	}
	return rs, false
}

func withVersion(id, sep, version string) string {
	if version == "" || strings.Contains(id, sep) {
		return id
	}
	return id + sep + version
}

var templateRef = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_.-]*)\}`)

// templateVars turns the registry's {name} placeholders into ${NAME}
// references, recording each as needed.
func templateVars(v string, needs *[]RegistryVar) string {
	return templateRef.ReplaceAllStringFunc(v, func(m string) string {
		name := envName(templateRef.FindStringSubmatch(m)[1])
		if needs != nil {
			*needs = append(*needs, RegistryVar{Name: name, Secret: true})
		}
		return "${" + name + "}"
	})
}

var envJunk = regexp.MustCompile(`[^A-Z0-9_]+`)

// envName is a variable name for a placeholder or a header: API_KEY for
// {api_key}, X_API_KEY for X-Api-Key.
func envName(s string) string {
	return strings.Trim(envJunk.ReplaceAllString(strings.ToUpper(s), "_"), "_")
}
