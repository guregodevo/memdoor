package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Where servers are configured: the project's .mcp.json (Claude Code's
// format, so an existing one just works) and ~/.memdoor/mcp.json for every
// project. A name in both is the project's.

// Scope says which file a server comes from.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

// ProjectFile is the project's config file name.
const ProjectFile = ".mcp.json"

// Entry is one server as the file writes it.
type Entry struct {
	Type     string            `json:"type,omitempty"` // "stdio" or "http"; inferred when empty
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
}

// Remote reports whether the entry is a URL server.
func (e Entry) Remote() bool { return e.URL != "" || e.Type == "http" || e.Type == "sse" }

// Server is one configured server: its name, where it comes from, and its
// entry as written (variables not expanded).
type Server struct {
	Name  string
	Scope Scope
	Path  string
	Entry Entry
}

// Spec is the server ready to connect: ${VAR} and ${VAR:-default}
// expanded from env, the command run in projectDir.
func (s Server) Spec(projectDir string, env func(string) (string, bool)) (Spec, error) {
	var missing []string
	expand := func(v string) string {
		return varRef.ReplaceAllStringFunc(v, func(m string) string {
			sub := varRef.FindStringSubmatch(m)
			if val, ok := env(sub[1]); ok && val != "" {
				return val
			}
			if sub[2] != "" {
				return strings.TrimPrefix(sub[2], ":-")
			}
			missing = append(missing, sub[1])
			return ""
		})
	}
	sp := Spec{Name: s.Name, Command: expand(s.Entry.Command), URL: expand(s.Entry.URL), Dir: projectDir}
	for _, a := range s.Entry.Args {
		sp.Args = append(sp.Args, expand(a))
	}
	if len(s.Entry.Env) > 0 {
		sp.Env = map[string]string{}
		for k, v := range s.Entry.Env {
			sp.Env[k] = expand(v)
		}
	}
	if len(s.Entry.Headers) > 0 {
		sp.Headers = map[string]string{}
		for k, v := range s.Entry.Headers {
			sp.Headers[k] = expand(v)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		// Every surface names the server already, so the reason does not.
		return sp, fmt.Errorf("needs %s set in the environment", strings.Join(dedupe(missing), ", "))
	}
	sp.SSE = s.Entry.Type == "sse"
	return sp, nil
}

var varRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-[^}]*)?\}`)

var nameOK = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)

// ValidName reports why name cannot name a server, or nil. Tools are named
// mcp__<server>__<tool>, so the name keeps to letters, digits, - and _.
func ValidName(name string) error {
	if !nameOK.MatchString(name) {
		return fmt.Errorf("a server name is 1-48 letters, digits, - or _ (got %q)", name)
	}
	return nil
}

// Store is where server configs and the per-project trust live.
type Store interface {
	// Servers is every server for projectDir, project entries first; a name
	// in both files is the project's.
	Servers(projectDir string) ([]Server, error)
	// Add writes a server into the scope's file, replacing one of that name.
	Add(scope Scope, projectDir, name string, e Entry) error
	// Remove deletes a server from the file that holds it.
	Remove(scope Scope, projectDir, name string) error
	// SetDisabled turns a server off or back on in its file.
	SetDisabled(scope Scope, projectDir, name string, disabled bool) error
	// Trusted reports whether the project's servers, as they are now, were
	// allowed to start; Trust records that they are.
	Trusted(projectDir string) (bool, error)
	Trust(projectDir string) error
}

// NewFileStore is the Store over the files: the user file and the trust
// record under memdoorDir (~/.memdoor).
func NewFileStore(memdoorDir string) Store {
	return &fileStore{dir: memdoorDir}
}

type fileStore struct {
	dir string
	mu  sync.Mutex
}

func (f *fileStore) path(scope Scope, projectDir string) string {
	if scope == ScopeProject {
		return filepath.Join(projectDir, ProjectFile)
	}
	return filepath.Join(f.dir, "mcp.json")
}

// readFile is the file's top-level object and its servers; a missing file
// is empty.
func readFile(path string) (map[string]json.RawMessage, map[string]Entry, error) {
	top := map[string]json.RawMessage{}
	servers := map[string]Entry{}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return top, servers, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return top, servers, nil
	}
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if raw, ok := top["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, nil, fmt.Errorf("%s: mcpServers: %w", path, err)
		}
	}
	return top, servers, nil
}

func (f *fileStore) Servers(projectDir string) ([]Server, error) {
	var out []Server
	seen := map[string]bool{}
	for _, scope := range []Scope{ScopeProject, ScopeUser} {
		if scope == ScopeProject && projectDir == "" {
			continue
		}
		p := f.path(scope, projectDir)
		_, servers, err := readFile(p)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(servers))
		for n := range servers {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, Server{Name: n, Scope: scope, Path: p, Entry: servers[n]})
		}
	}
	return out, nil
}

// edit rewrites the scope's file through fn, which sees each server's raw
// JSON: a field this package does not model (Claude Code's "timeout")
// survives an edit, as do the file's other top-level keys. The write is
// atomic.
func (f *fileStore) edit(scope Scope, projectDir string, fn func(map[string]json.RawMessage) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.path(scope, projectDir)
	top, _, err := readFile(p)
	if err != nil {
		return err
	}
	raw := map[string]json.RawMessage{}
	if r, ok := top["mcpServers"]; ok {
		if err := json.Unmarshal(r, &raw); err != nil {
			return fmt.Errorf("%s: mcpServers: %w", p, err)
		}
	}
	if err := fn(raw); err != nil {
		return err
	}
	enc, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	top["mcpServers"] = enc
	// A file left holding no servers and nothing else is not kept: an empty
	// .mcp.json in a repository reads as configuration and is committed as
	// one (live 2026-10-01, after a refused add).
	if len(raw) == 0 && len(top) == 1 {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	b, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600) // the user file can hold keys
	if scope == ScopeProject {
		mode = 0o644 // a project file is meant to be committed
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".mcp-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (f *fileStore) Add(scope Scope, projectDir, name string, e Entry) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if (e.Command == "") == (e.URL == "") {
		return fmt.Errorf("server %q needs a command or a URL, not both", name)
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return f.edit(scope, projectDir, func(s map[string]json.RawMessage) error {
		s[name] = b
		return nil
	})
}

func (f *fileStore) Remove(scope Scope, projectDir, name string) error {
	return f.edit(scope, projectDir, func(s map[string]json.RawMessage) error {
		if _, ok := s[name]; !ok {
			return fmt.Errorf("no server %q in %s", name, f.path(scope, projectDir))
		}
		delete(s, name)
		return nil
	})
}

func (f *fileStore) SetDisabled(scope Scope, projectDir, name string, disabled bool) error {
	return f.edit(scope, projectDir, func(s map[string]json.RawMessage) error {
		r, ok := s[name]
		if !ok {
			return fmt.Errorf("no server %q in %s", name, f.path(scope, projectDir))
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(r, &fields); err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
		if disabled {
			fields["disabled"] = json.RawMessage("true")
		} else {
			delete(fields, "disabled")
		}
		b, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		s[name] = b
		return nil
	})
}

// projectHash is what the trust covers: the project file's server entries,
// on or off aside. Any other change (a new server, another command) asks
// again.
func (f *fileStore) projectHash(projectDir string) (string, bool, error) {
	_, servers, err := readFile(f.path(ScopeProject, projectDir))
	if err != nil {
		return "", false, err
	}
	if len(servers) == 0 {
		return "", false, nil
	}
	// On or off is not what runs: turning a server off and back on keeps
	// the trust.
	for n, e := range servers {
		e.Disabled = false
		servers[n] = e
	}
	b, _ := json.Marshal(servers) // map keys marshal sorted
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true, nil
}

func (f *fileStore) trustPath() string { return filepath.Join(f.dir, "mcp-trust.json") }

func (f *fileStore) readTrust() (map[string]string, error) {
	t := map[string]string{}
	b, err := os.ReadFile(f.trustPath())
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", f.trustPath(), err)
	}
	return t, nil
}

func (f *fileStore) Trusted(projectDir string) (bool, error) {
	h, has, err := f.projectHash(projectDir)
	if err != nil || !has {
		return !has, err // no project servers: nothing to trust
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.readTrust()
	if err != nil {
		return false, err
	}
	return t[filepath.Clean(projectDir)] == h, nil
}

func (f *fileStore) Trust(projectDir string) error {
	h, has, err := f.projectHash(projectDir)
	if err != nil || !has {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.readTrust()
	if err != nil {
		return err
	}
	t[filepath.Clean(projectDir)] = h
	b, _ := json.MarshalIndent(t, "", "  ")
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(f.trustPath(), append(b, '\n'), 0o600)
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}
