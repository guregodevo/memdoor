package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/config"
	"memdoor/pkg/shared"
)

// Workspace resolution chain (highest precedence first):
//
//   1. Explicit -w/--workspace flag (already populated into the
//      package-level workspaceSlug var by cobra before PreRunE fires).
//   2. $MEMDOOR_WORKSPACE environment variable.
//   3. .memdoor/workspace file discovered by walking up from cwd to
//      $HOME (or filesystem root, whichever comes first). Single line,
//      the slug. Same shape as .nvmrc / .python-version / .ruby-version.
//   4. config.LoadConfig().WorkspaceID — the per-install default in
//      ~/.memdoor/config.json that pre-existed this feature.
//
// Mirrors the precedence pattern users already know from git, kubectl
// --context, and direnv: explicit > env > directory walk > global
// default. No new conventions to learn.

// discoveryFileName is the filename the walker looks for in each
// directory. Lives under .memdoor/ so future workspace-local config
// (.memdoor/config.toml etc.) can sit alongside without colliding.
const (
	discoveryFileName = "workspace"
	envWorkspaceVar   = "MEMDOOR_WORKSPACE"
)

// workspaceSource labels the resolution chain step that produced the
// slug, for verbose logging and `memdoor workspace which`.
type workspaceSource struct {
	kind string // "flag" | "env" | "file" | "config"
	loc  string // path or env var name; "" for flag
}

func (s workspaceSource) String() string {
	switch s.kind {
	case "flag":
		return "flag -w"
	case "env":
		return "env " + envWorkspaceVar
	case "file":
		return "file " + s.loc
	case "config":
		return "config " + s.loc
	default:
		return s.kind
	}
}

// resolveWorkspaceSlug runs the precedence chain and returns
// (slug, source, ok). When ok is false the caller decides whether
// the missing workspace is actually fatal (some commands are exempt
// via noWorkspaceTopLevels in root.go).
//
// The flag check uses cobra's Changed field instead of "workspaceSlug
// != \"\"". This matters because requireWorkspaceSlug (PreRunE) writes
// the resolved slug back into workspaceSlug so RunE keeps working
// without changes — but that means by the time `workspace which`'s
// RunE re-resolves, workspaceSlug is already populated from a non-flag
// source. Only Changed tells us the user actually typed -w.
func resolveWorkspaceSlug() (string, workspaceSource, bool) {
	// 1. Explicit flag — only true when the user actually typed -w.
	if f := rootCmd.PersistentFlags().Lookup("workspace"); f != nil && f.Changed {
		return f.Value.String(), workspaceSource{kind: "flag"}, true
	}
	// 2. Environment variable.
	if v := strings.TrimSpace(os.Getenv(envWorkspaceVar)); v != "" {
		return v, workspaceSource{kind: "env"}, true
	}
	// 3. Walk up from cwd looking for .memdoor/workspace.
	if cwd, err := os.Getwd(); err == nil {
		home, _ := os.UserHomeDir()
		if slug, path, ok := discoverWorkspaceFromDir(cwd, home); ok {
			return slug, workspaceSource{kind: "file", loc: path}, true
		}
	}
	// 4. Per-install config default.
	if cfg, err := config.LoadConfig(); err == nil && cfg.WorkspaceID != "" {
		path, _ := config.ResolveConfigPath()
		return cfg.WorkspaceID, workspaceSource{kind: "config", loc: path}, true
	}
	return "", workspaceSource{}, false
}

// discoverWorkspaceFromDir walks up from start looking for
// <dir>/.memdoor/workspace. Stops at home (inclusive — we still check
// home itself) or at the filesystem root, whichever comes first.
//
// Skips entries where .memdoor/workspace is a directory rather than a
// regular file. This matters because `memdoor setup` creates
// ~/.memdoor/workspace/ as the bootstrap dir for the install — the
// walker hitting $HOME would otherwise trip on that and report a
// confusing "is a directory" error.
func discoverWorkspaceFromDir(start, home string) (slug, path string, ok bool) {
	dir := filepath.Clean(start)
	for {
		candidate := filepath.Join(dir, shared.MemdoorDirName, discoveryFileName)
		stat, err := os.Stat(candidate)
		if err == nil && !stat.IsDir() {
			data, err := os.ReadFile(candidate)
			if err == nil {
				if s := firstNonEmptyLine(string(data)); s != "" {
					return s, candidate, true
				}
			}
		}
		// Stop when we've checked $HOME (or hit root). Stopping at
		// $HOME prevents `cd /tmp/foo` from picking up an
		// .memdoor/workspace from way up the tree.
		if home != "" && dir == home {
			return "", "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false // reached root
		}
		dir = parent
	}
}

// firstNonEmptyLine returns the first non-empty trimmed line of s.
// Multi-line files are tolerated; only the first slug-looking line is
// used. Empty / whitespace-only files return "".
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// writePinFile creates `<cwd>/.memdoor/<basename>` with the given slug
// as its single line. Every pin command has the same fragility — the path
// `<cwd>/.memdoor/workspace` collides with the on-disk bootstrap dir
// `memdoor setup` creates at $HOME, so a naive WriteFile from $HOME
// errors with EISDIR and the user has no idea why.
//
// Refuses to write when:
//   - cwd is $HOME (discoveryWalker stops at $HOME, so pinning here is
//     functionally meaningless AND ~/.memdoor/workspace is the install's
//     bootstrap dir — same path, different semantics).
//   - the target path exists as a directory (covers cwd inside the
//     install dir or any other collision).
//
// Returns the absolute pin path on success so callers can echo it back.
func writePinFile(basename, slug string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	if home, herr := os.UserHomeDir(); herr == nil && home != "" {
		// Resolve both sides through symlinks before comparing. On macOS
		// /tmp and /var are symlinks to /private/tmp and /private/var,
		// so a fakeHome under TempDir comes back from Getwd with the
		// /private prefix while $HOME stays without it — a plain
		// filepath.Clean comparison misses. EvalSymlinks gives a stable
		// canonical form on both sides.
		if sameDir(cwd, home) {
			return "", fmt.Errorf(
				"refusing to pin %q from $HOME: the discovery walker stops AT $HOME, "+
					"so a pin file here only applies to commands run literally from $HOME — "+
					"and `memdoor setup` already created ~/.memdoor/workspace as the install's "+
					"bootstrap directory.\n  Fix: cd into a project directory, then re-run.",
				basename)
		}
	}
	dir := filepath.Join(cwd, shared.MemdoorDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, basename)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "", fmt.Errorf(
			"%s already exists as a directory — cannot write a pin file with the same name. "+
				"This usually means cwd is inside an memdoor install dir (~/.memdoor).\n  Fix: cd into a project directory, then re-run.",
			path)
	}
	if err := os.WriteFile(path, []byte(slug+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// sameDir reports whether two directory paths refer to the same
// location after resolving symlinks. Used by writePinFile's $HOME
// guard so macOS /var ↔ /private/var symlink expansion doesn't
// silently let a $HOME pin slip through. Falls back to a cleaned
// string compare when EvalSymlinks fails — better to enforce the
// guard conservatively than skip it on read errors.
func sameDir(a, b string) bool {
	clean := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Clean(abs)
		}
		return filepath.Clean(p)
	}
	return clean(a) == clean(b)
}

// THERE IS A DEFAULT WORKSPACE (Greg, 2026-09-27, walking onboarding: "there is
// a default workspace"). When a person has exactly ONE, asking which one is a
// question with a single possible answer, and answering it wrong — by refusing —
// is what a command did in any directory they had not pinned.
//
// So the chain in resolveWorkspaceSlug (flag → env → .memdoor/workspace walked
// up → config default) gains a last step: the only workspace on the machine.
// With several, the ambiguity is real and the error names them instead of
// reciting four ways to set one.
func soleWorkspace() (string, bool) {
	slugs := listAllWorkspaceSlugs()
	if len(slugs) == 1 {
		return slugs[0], true
	}
	return "", false
}

// noWorkspaceErrorMessage builds the actionable error returned when a
// workspace-scoped command runs with nothing resolved and no single default.
// It names the real candidates when there are some, because "set one of" is
// unhelpful to a person who has three and does not remember the spelling.
func noWorkspaceErrorMessage(topName string) error {
	if slugs := listAllWorkspaceSlugs(); len(slugs) > 1 {
		return fmt.Errorf(
			"several workspaces here — say which one:\n"+
				"  memdoor -w %s %s ...          (once)\n"+
				"  memdoor workspace use %s      (pins this directory)\n"+
				"  yours: %s",
			slugs[0], topName, slugs[0], strings.Join(slugs, ", "))
	}
	return fmt.Errorf(
		"nothing is set up on this machine yet — `memdoor setup` makes the first workspace.\n"+
			"  already have one elsewhere? memdoor workspace use <slug>, or memdoor -w <slug> %s ...",
		topName)
}
