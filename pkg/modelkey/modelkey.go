// Package modelkey finds the model keys on a machine by their shape, whatever
// variable or file holds them: what `memdoor connect` offers before it asks for
// one, and what the agent's commands never inherit.
package modelkey

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Provider is a model provider a key is for, by the id `memdoor connect`
// knows it under (cmd/tui/ui ConnectKinds).
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGemini     Provider = "gemini"
	ProviderGroq       Provider = "groq"
	ProviderXAI        Provider = "xai"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderBaseten    Provider = "baseten"
	ProviderTypeSafe   Provider = "typesafe"
)

// keyShapes are the model providers' key formats, most specific first. A
// key is found by its value, never its variable's name (the maintainer, 2026-10-31:
// "if a key name does not match our memdoor key name"): OPENROUTER_API_KEY,
// OPEN_ROUTER_API_KEY and MY_OR_KEY are the same key. An Anthropic OAuth
// token (sk-ant-oat…, a Claude subscription's) and an Anthropic admin key
// (sk-ant-admin…) run no model through the API, so neither is one (notKeys);
// any other sk-ant- kind is (live 2026-10-31: an sk-ant-usr- key was missed),
// and the probe decides. Google's keys are AIza… or, newer, AQ.….
var keyShapes = []struct {
	re        *regexp.Regexp
	providers []Provider
}{
	{regexp.MustCompile(`^sk-or-v1-[0-9a-f]{32,}$`), []Provider{ProviderOpenRouter}},
	{regexp.MustCompile(`^sk-ant-[a-z]+[0-9]*-[A-Za-z0-9_\-]{32,}$`), []Provider{ProviderAnthropic}},
	{regexp.MustCompile(`^sk-(?:proj|svcacct)-[A-Za-z0-9_\-]{32,}$`), []Provider{ProviderOpenAI}},
	{regexp.MustCompile(`^AIza[0-9A-Za-z_\-]{35}$`), []Provider{ProviderGemini}},
	{regexp.MustCompile(`^AQ\.[0-9A-Za-z_\-]{30,}$`), []Provider{ProviderGemini}},
	{regexp.MustCompile(`^gsk_[A-Za-z0-9]{32,}$`), []Provider{ProviderGroq}},
	{regexp.MustCompile(`^xai-[A-Za-z0-9]{32,}$`), []Provider{ProviderXAI}},
	// DeepSeek's keys are sk- and 32 hex digits; an older OpenAI key is sk-
	// and 48 letters and digits. The probe decides between them.
	{regexp.MustCompile(`^sk-[0-9a-f]{32}$`), []Provider{ProviderDeepSeek, ProviderOpenAI}},
	{regexp.MustCompile(`^sk-[A-Za-z0-9]{40,}$`), []Provider{ProviderOpenAI, ProviderDeepSeek}},
}

// notKeys are shaped like a provider's key but run no model.
var notKeys = []string{"sk-ant-oat", "sk-ant-admin"}

// ProvidersOf is the providers a value is shaped like a key for, the likelier
// first; nil when it is not a model key.
func ProvidersOf(value string) []Provider {
	value = strings.TrimSpace(value)
	for _, not := range notKeys {
		if strings.HasPrefix(value, not) {
			return nil
		}
	}
	for _, s := range keyShapes {
		if s.re.MatchString(value) {
			return s.providers
		}
	}
	return nil
}

// providerNames are the providers' own variable names for the keys that
// have no shape of their own (Baseten's, TypeSafe's): under that name, a
// value is that provider's key. A shaped key needs no name.
var providerNames = map[string]Provider{
	"BASETEN_API_KEY":           ProviderBaseten,
	"TYPESAFE_API_KEY":          ProviderTypeSafe,
	"MEMDOOR_SYSTEMONE_API_KEY": ProviderTypeSafe,
}

// ProviderNames is the variable names a key with no shape is found under.
func ProviderNames() map[string]Provider {
	out := make(map[string]Provider, len(providerNames))
	for k, v := range providerNames {
		out[k] = v
	}
	return out
}

// keyLike says a value under a provider's own name could be a key: one word,
// long enough, no placeholder.
var keyLike = regexp.MustCompile(`^[A-Za-z0-9._\-]{20,}$`)

// providersFor is the providers a variable holds a key for: by the value's
// shape, the variable's name breaking a tie (DEEPSEEK_API_KEY holds DeepSeek's
// sk- key, not OpenAI's), else by the provider's own name for a key with no
// shape. named says the name chose the first.
func providersFor(name, value string) (ps []Provider, named bool) {
	if ps = ProvidersOf(value); len(ps) > 1 {
		upper := strings.ToUpper(strings.ReplaceAll(name, "_", ""))
		for i, p := range ps {
			if strings.Contains(upper, strings.ToUpper(string(p))) {
				return append([]Provider{p}, append(append([]Provider{}, ps[:i]...), ps[i+1:]...)...), true
			}
		}
		return ps, false
	}
	if ps != nil {
		return ps, false
	}
	if p, ok := providerNames[name]; ok && keyLike.MatchString(value) {
		return []Provider{p}, true
	}
	return nil, false
}

// Found is a model key found on this machine, and where.
type Found struct {
	Providers []Provider // the likelier first
	Named     bool       // the variable's name chose the first (a tie, or a key with no shape)
	Name      string     // the variable it is under
	File      string     // the file it was read from; "" for this process's environment
	Value     string
}

// Where says where the key is, in the words a person would look for it.
func (k Found) Where() string {
	switch {
	case k.File == "":
		return "$" + k.Name
	case k.Name == "":
		return k.File
	}
	return k.File + ": " + k.Name
}

// InEnvironment says whether the key is a variable of this process's
// environment, so a gateway started from the same shell can read it by name.
func (k Found) InEnvironment() bool { return k.File == "" }

// dotenvFiles are the project files a key is commonly kept in.
var dotenvFiles = []string{".env", ".env.local", ".envrc"}

// Find is every model key in the environment given (os.Environ's form),
// then in dir's .env files, each value once. Nothing is read but the
// environment and those files.
func Find(environ []string, dir string) []Found {
	var found []Found
	seen := map[string]bool{}
	add := func(name, value, file string) {
		value = strings.TrimSpace(value)
		ps, named := providersFor(name, value)
		if ps == nil || seen[value] {
			return
		}
		seen[value] = true
		found = append(found, Found{Providers: ps, Named: named, Name: name, File: file, Value: value})
	}
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if ok {
			add(name, value, "")
		}
	}
	if dir == "" {
		return found
	}
	for _, f := range dotenvFiles {
		path := filepath.Join(dir, f)
		for _, kv := range readDotenv(path) {
			add(kv[0], kv[1], f)
		}
	}
	return found
}

// readDotenv is the NAME=value pairs of a .env or .envrc file: `export`
// allowed, quotes taken off, comments and anything else skipped.
func readDotenv(path string) [][2]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out [][2]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.ContainsAny(name, " \t") {
			continue
		}
		out = append(out, [2]string{strings.TrimSpace(name), dotenvValue(value)})
	}
	return out
}

// dotenvValue is a value as written after the =: inside its quotes when
// quoted, else up to a " #" comment.
func dotenvValue(v string) string {
	v = strings.TrimSpace(v)
	if v != "" && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : end+1]
		}
		return v[1:]
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
