package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sharedctx "memdoor/pkg/shared/context"
)

// agent_runtime_coder: codebase-agent detection and coder-workdir confinement of tool inputs.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// agentWorksOnCodebase reports whether the buddy is a codebase task agent — its
// palette includes a filesystem tool (read/write/edit/bash/grep/glob). Both the
// coder (edits + runs) and the planner (researches read-only) qualify; a
// chat-style agent with only web/log tools does not. Such an agent:
//
//   - gets an identity-only system prompt (PromptModeNone) so its OWN seeded
//     prompt is the whole instruction, instead of the chat-mode "stay silent /
//     REPLY_SKIP" collaboration block (which makes a small model skip the task);
//   - has its file tools resolve against the REAL project cwd (see
//     doerFilesystemTools) rather than the ~/.memdoor/sandbox tree.
//
// (Forced grounding and the citation guard need no check here: a codebase
// palette cannot invoke them.)
func (ar *AgentRuntime) agentWorksOnCodebase(ctx context.Context) bool {
	buddyTools, _ := ctx.Value(sharedctx.BuddyToolsKey).([]string)
	for _, t := range buddyTools {
		switch t {
		case "bash", "edit_file", "write_file", "read_file", "grep", "glob":
			return true
		}
	}
	return false
}

// doerFilesystemTools are the sandbox-aware file tools whose paths a codebase
// agent should resolve against the REAL cwd (like bash/edit_file) rather than
// the ~/.memdoor/sandbox tree — so the coder's write→verify loop and the
// planner's read-only research both see the actual project on one filesystem.
var doerFilesystemTools = map[string]bool{
	"read_file":  true,
	"write_file": true,
	"list_files": true,
}

// Permission modes (Claude-style, cycled with Shift+Tab). They gate tool
// execution on the SAME agent: plan is read-only, acceptEdits/default allow edits.
const (
	permissionModeDefault     = "default"
	permissionModeAcceptEdits = "acceptEdits"
	permissionModePlan        = "plan"
)

// doerCompactionKeepRecent is how many recent messages the fit ladder's first
// stubbing rung keeps (agent_runtime_fit.go). It is deliberately small: a
// codebase agent reaches the threshold with only a few large tool-output
// messages, so keeping ~20 would no-op the prune. Six leaves the current tool
// cycle plus a little slack while guaranteeing the oldest bloat is dropped.
const doerCompactionKeepRecent = 6

// coderWorkdir returns the directory a codebase agent's files/bash run in, creating
// it if needed. It is NEVER the gateway's cwd (which in dev IS this repo, so a
// relative "write_file main.go" would clobber the entrypoint). Point it at a real
// project by launching the TUI there; otherwise it's ~/memdoor-coder.
func coderWorkdir() string {
	home, _ := os.UserHomeDir()
	d := filepath.Join(home, "memdoor-coder")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// turnWorkdir resolves the working directory for THIS turn: the client's
// launch directory when it sent one (Claude-CLI semantics — the TUI's cwd),
// else the coder's sandbox.
func turnWorkdir(ctx context.Context) string {
	if wd, _ := ctx.Value(sharedctx.WorkdirKey).(string); wd != "" {
		return wd
	}
	return coderWorkdir()
}

// manglePathReason reports why a model-supplied path cannot be a real one, or
// "" when it looks like a filename.
//
// Narrow on purpose. The failure it catches is a tool call whose arguments got
// mangled into a schema fragment, and the two markers of that are characters no
// legitimate source path carries. Rejecting on anything broader would start
// refusing filenames people actually use.
func manglePathReason(p string) string {
	switch {
	case strings.ContainsAny(p, "\"\n\r"):
		return "contains a quote or newline — this is a mangled tool argument, not a path"
	case len(p) > 512:
		return "longer than any real path — a mangled tool argument"
	}
	return ""
}

// absCd matches a `cd` to an ABSOLUTE path, quoted or bare. Relative cds are
// left alone: `cd internal/foo` stays inside the workdir by construction.
var absCd = regexp.MustCompile(`(?:^|[;&|]\s*)cd\s+('|")?(/[^\s;&|'"]*)`)

// escapesWorkdir returns the first absolute directory a command cd's to that
// lies outside wd, or "" when the command stays put.
//
// Only `cd` is inspected. A command may still READ an absolute path — that is
// the deliberate "may target a real project on purpose" behaviour below — but
// changing directory out of the workdir is how the confinement stops meaning
// anything, so it is refused rather than silently honoured.
func escapesWorkdir(cmd, wd string) string {
	if wd == "" {
		return ""
	}
	root := filepath.Clean(wd)
	for _, m := range absCd.FindAllStringSubmatch(cmd, -1) {
		target := filepath.Clean(m[2])
		if target == root || strings.HasPrefix(target, root+string(filepath.Separator)) {
			continue
		}
		return target
	}
	return ""
}

// shellQuote wraps s for safe use inside single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// coderPathEscape returns a refusal message when a codebase agent's WRITING
// tool names an ABSOLUTE path outside the turn's workdir, or "" when the call
// is fine. The bash branch has refused `cd` escapes since 2026-08-29 for
// exactly this failure — a model INVENTS an absolute path — and writes are
// the stray-main.go incident with a different tool. Same fence, same voice.
//
// READS GO ANYWHERE (Greg, 2026-10-05: "like omp" — oh-my-pi confines no
// read in any mode; --add-dir only tells the model more roots exist). A
// coder reading a dependency, another repo or a system file is doing its
// job; refusing read_file while `sed` through bash read the same file anyway
// (live 2026-10-05, thirty reads of this repo from a scratch project) was a
// fence with a gate open beside it. Writes stay confined.
func coderPathEscape(toolName string, input json.RawMessage, wd string) string {
	switch toolName {
	case "write_file", "edit_file":
	default:
		return ""
	}
	if wd == "" {
		return ""
	}
	var in map[string]any
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	root := filepath.Clean(wd)
	for _, key := range []string{"path", "file_path"} {
		p, ok := in[key].(string)
		if !ok || !filepath.IsAbs(p) {
			continue
		}
		target := filepath.Clean(p)
		if target == root || strings.HasPrefix(target, root+string(filepath.Separator)) {
			continue
		}
		return "refused: " + p + " is outside your working directory (" + wd +
			"). You are ALREADY in the right directory — use paths relative to it."
	}
	return ""
}

// confineToCoderWorkdir rewrites a codebase agent's tool input so relative file
// paths land in the coder workspace (not the gateway cwd) and bash runs there.
// Absolute paths pass through this REWRITE untouched — but an absolute path
// outside the workdir never reaches a tool: coderPathEscape refuses it at the
// call site first (2026-09-01; the old "may target a real project on purpose"
// theory died the same way the bash cd passthrough did — a model inventing
// absolute paths into this repo).
func confineToCoderWorkdir(toolName string, input json.RawMessage, wd string) json.RawMessage {
	var in map[string]any
	if json.Unmarshal(input, &in) != nil {
		return input
	}
	changed := false
	switch toolName {
	case "bash":
		cmd, _ := in["command"].(string)
		// A small model sometimes nests the REAL command under "parameters"/"input"
		// and leaves the tool name ("bash") in "command". Unwrap it BEFORE confining,
		// else we'd prepend cd to the literal "bash" (a no-op) and lose the command.
		if strings.TrimSpace(cmd) == "" || strings.TrimSpace(cmd) == "bash" {
			for _, key := range []string{"parameters", "input"} {
				if nested, ok := in[key].(map[string]any); ok {
					if nc, ok := nested["command"].(string); ok && strings.TrimSpace(nc) != "" {
						cmd = nc
						break
					}
				}
			}
		}
		if strings.TrimSpace(cmd) != "" {
			// The cd prefix is only a DEFAULT directory, not a fence: the model's
			// own `cd` runs after it and wins. Observed 2026-08-29 on a 27B,
			// which emitted
			//   cd '/Users/you/memdoor-coder' && cd /Users/you/.memdoor/workdir/tetris && go run stats.go
			// — a path it invented, from no prompt or skill in this repo. Two tool
			// calls were wasted on the failure, and had it invented a path that
			// EXISTS, the command would have run there. That is the likely
			// mechanism behind the stray main.go written into the repo root and
			// gateway/client/ (AGENTS.md, "three times this weekend"), because the
			// coder runs always-auto with no permission prompt.
			if esc := escapesWorkdir(cmd, wd); esc != "" {
				in["command"] = "printf %s " + shellQuote(
					"refused: this command cd's to "+esc+", outside your working directory ("+wd+
						"). You are ALREADY in the right directory — drop the cd and use paths relative to it.") + " >&2; exit 1"
				changed = true
				break
			}
			// The workdir is passed as cwd, the same way apply_patch, locate and
			// skill already receive it — the tool sets cmd.Dir and the operating
			// system enforces it. Prefixing `cd '<wd>' && ` expressed the same
			// intent to a SHELL, which the model's own cd could then override.
			in["command"] = cmd
			in["cwd"] = wd
			changed = true
		}
	case "write_file", "edit_file", "read_file", "list_files", "grep", "glob", "jgrep", "jread":
		// An ABSENT path is the dangerous case, not a wrong one.
		//
		// glob and grep take `path` as OPTIONAL, documented as "default:
		// current directory" — and the current directory is the GATEWAY's,
		// which in development is this repository. Rewriting only the paths
		// that are present left the omitted ones pointing at it.
		//
		// Observed 2026-08-30 on a live coder turn: the model called
		// glob("**/*_test.go") with no path and got 299 lines of memdoor's own
		// source — gateway/rag/, pkg/domain/, pkg/shared/ — none of which is in
		// its workdir. It then lost the task entirely and never edited the file
		// it had been asked about.
		//
		// So the workdir is INJECTED when the key is missing, for the tools
		// whose path is optional. write_file and friends require one, and a
		// call without it is malformed rather than unconfined.
		switch toolName {
		case "list_files", "grep", "glob", "jgrep":
			if _, has := in["path"]; !has {
				in["path"] = wd
				changed = true
			}
		}
		for _, key := range []string{"path", "file_path"} {
			p, ok := in[key].(string)
			if !ok {
				continue
			}
			// A path the model mangled is not a path. When a small model emits
			// a malformed tool call, its arguments can arrive as a fragment of
			// the tool SCHEMA — and filepath.Join will happily turn that into a
			// filename. Found 2026-08-30 in the coder workdir, dated Jul 27:
			//
			//   greetname.go and the full corrected content", "Fix the patch to
			//   correctly modify the function"], "default": "Fix the patch to ...
			//
			// A zero-byte file whose NAME is JSON from apply_patch's own schema.
			// It sat there for a month polluting every `ls` the coder ran, which
			// is how it was finally noticed.
			//
			// The test is deliberately narrow: a double quote or a newline in a
			// path is never a real source file and always a mangled argument.
			// Spaces are legal in filenames and are left alone.
			if bad := manglePathReason(p); bad != "" {
				in[key] = ""
				changed = true
				continue
			}
			if p == "" || !filepath.IsAbs(p) {
				in[key] = filepath.Join(wd, p)
				changed = true
			}
		}
	case "apply_patch":
		// The patch text carries its own (relative) file paths, so pass the workdir
		// as an internal cwd for the tool to resolve them under — keeps writes in the
		// sandbox without rewriting the patch body.
		in["cwd"] = wd
		changed = true
	case "locate":
		// locate walks the repo to rank candidate files; point it at the coder's
		// workdir (the repo being edited), NOT the gateway's own CWD — otherwise it
		// indexes memdoor's own source and returns nonsense the coder loops on.
		in["cwd"] = wd
		changed = true
	case "skill":
		// skill resolves <workdir>/.agents/skills as its project-local tier —
		// the dir the coder itself can write skills into (self-extension).
		in["cwd"] = wd
		changed = true
	case "verify":
		// verify runs build/test commands; without a dir it executes in the
		// GATEWAY's cwd (live: "stat stats.go: no such file" from /tmp deploy dir).
		// Default it to the coder workdir; an explicit absolute dir is respected.
		key := "dir"
		// A RELATIVE FOLDER IS RELATIVE TO THE SESSION, NEVER TO THE GATEWAY.
		//
		// Injecting the workdir only when the key was EMPTY left an explicit
		// relative folder to resolve against the gateway's own cwd — which is
		// "/" when the app is launched from Finder. Live 2026-09-16:
		// `out_dir:"ai_chip"` came back "mkdir ai_chip: read-only file
		// system", from a session whose folder was perfectly writable. The
		// model meant "a folder called ai_chip, here".
		d, _ := in[key].(string)
		switch {
		case d == "":
			in[key] = wd
			changed = true
		case strings.HasPrefix(d, "~/"):
			if home, err := os.UserHomeDir(); err == nil {
				in[key] = filepath.Join(home, d[2:])
				changed = true
			}
		case !filepath.IsAbs(d):
			in[key] = filepath.Join(wd, d)
			changed = true
		}
	}
	if !changed {
		return input
	}
	if out, err := json.Marshal(in); err == nil {
		return out
	}
	return input
}

// permissionModeFromContext returns the turn's interaction mode (default when unset).
func permissionModeFromContext(ctx context.Context) string {
	if m, _ := ctx.Value(sharedctx.PermissionModeKey).(string); m != "" {
		return m
	}
	return permissionModeDefault
}
