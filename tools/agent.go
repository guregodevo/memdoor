package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/invopop/jsonschema"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	"memdoor/pkg/sandbox"
)

// Tool Definition
type ToolDefinition struct {
	Name                string                                                       `json:"name"`
	Description         string                                                       `json:"description"`
	InputSchema         llm.ToolInputSchemaParam                                     `json:"input_schema"`
	Function            func(input json.RawMessage) (string, error)                  // Legacy function (no context)
	FunctionWithContext func(input json.RawMessage, ctx interface{}) (string, error) // Context-aware function (for sandbox)
}

// read_file tool
var ReadFileDefinition = ToolDefinition{
	Name: "read_file",
	Description: "Read ONE file's contents, at a path relative to your working directory. " +
		"If you are not certain the file exists, call glob or list_files FIRST — a guessed filename " +
		"is the most expensive mistake available here: the read fails, and the failure does not " +
		"become a success on the next attempt. Not for directories.",
	InputSchema:         ReadFileInputSchema,
	Function:            ReadFile,            // Legacy function (no sandbox enforcement)
	FunctionWithContext: ReadFileWithContext, // Sandbox-aware function
}

type ReadFileInput struct {
	Path   string `json:"path" jsonschema_description:"Path to ONE file that EXISTS, relative to your working directory (e.g. \"main.go\", \"cmd/app/run.go\"). If you have not seen this name in a directory listing or a glob result, do not guess it — list first. Not a directory, not a glob pattern, not a list."`
	All    bool   `json:"all,omitempty" jsonschema_description:"true: the whole file, however long."`
	Offset int    `json:"offset,omitempty" jsonschema_description:"First line to read, 1-based (default 1)."`
	Limit  int    `json:"limit,omitempty" jsonschema_description:"How many lines to read (default 300)."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	log := logs.New("Agent")
	log.Debug("Reading file", slog.String("path", readFileInput.Path))
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		log.WithError(err).Error("Failed to read file", slog.String("path", readFileInput.Path))
		// A small model on a fresh task GUESSES filenames it has no memory of ("main.go") and a
		// bare "no such file" strands it — it flails or empties the turn. List what IS
		// in the directory so its next read targets a real file (same absorption as
		// locate's no-match listing).
		if os.IsNotExist(err) {
			if files := listSourceFiles(filepath.Dir(readFileInput.Path)); len(files) > 0 {
				return "", fmt.Errorf("%s does not exist. The directory DOES contain these files: %s — read the one you need (do not guess names)", readFileInput.Path, strings.Join(files, ", "))
			}
		}
		return "", err
	}
	log.Debug("Successfully read file",
		slog.String("path", readFileInput.Path),
		slog.Int("bytes", len(content)))
	return readableContent(readFileInput.Path, content, injectedTurnTask(input), readFileInput.All, readFileInput.Offset, readFileInput.Limit), nil
}

// ReadFileWithContext reads a file with sandbox path validation
func ReadFileWithContext(input json.RawMessage, ctx interface{}) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Extract sandbox context
	sandboxCtx, ok := ctx.(sandbox.SandboxContext)
	if !ok {
		// No sandbox context available - fall back to unrestricted access
		return ReadFile(input)
	}

	log := logs.New("Agent")

	// Normalize path (convert relative to virtual path)
	virtualPath := sandboxCtx.NormalizePath(readFileInput.Path)

	// Resolve virtual path to real filesystem path
	realPath, err := sandboxCtx.ResolvePath(virtualPath)
	if err != nil {
		log.Warn("Sandbox security: Path access denied",
			slog.String("path", readFileInput.Path),
			slog.String("virtual_path", virtualPath),
			slog.String("agent_scope", string(sandboxCtx.AgentScope)),
			slog.String("agent_id", sandboxCtx.CurrentAgentID))
		return "", fmt.Errorf("❌ **Sandbox Security: Access Denied**\n\nPath: %s\nVirtual: %s\nAgent Scope: %s\n\nThis agent does not have permission to access this path.\n\nAllowed paths:\n%s",
			readFileInput.Path,
			virtualPath,
			sandboxCtx.AgentScope,
			sandboxCtx.GetAllowedPathsDescription())
	}

	log.Debug("Sandbox security: Path access granted",
		slog.String("path", readFileInput.Path),
		slog.String("virtual_path", virtualPath),
		slog.String("real_path", realPath),
		slog.String("agent_scope", string(sandboxCtx.AgentScope)))

	// Path validated - proceed with reading
	content, err := os.ReadFile(realPath)
	if err != nil {
		log.WithError(err).Error("Failed to read file", slog.String("real_path", realPath))
		return "", err
	}
	log.Debug("Successfully read file",
		slog.String("virtual_path", virtualPath),
		slog.String("real_path", realPath),
		slog.Int("bytes", len(content)))
	return readableContent(readFileInput.Path, content, injectedTurnTask(input), readFileInput.All, readFileInput.Offset, readFileInput.Limit), nil
}

// list_files tool
var ListFilesDefinition = ToolDefinition{
	Name:                "list_files",
	Description:         "List files and directories at a given path. If no path is provided, lists files in the current directory.",
	InputSchema:         ListFilesInputSchema,
	Function:            ListFiles,            // Legacy function (no sandbox enforcement)
	FunctionWithContext: ListFilesWithContext, // Sandbox-aware function
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	log := logs.New("Agent")
	log.Debug("Listing files in directory", slog.String("dir", dir))
	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip .devenv directory and its contents
		if info.IsDir() && (relPath == ".devenv" || strings.HasPrefix(relPath, ".devenv/")) {
			return filepath.SkipDir
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		log.WithError(err).Error("Failed to list files", slog.String("dir", dir))
		return "", err
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	log.Debug("Successfully listed files",
		slog.String("dir", dir),
		slog.Int("count", len(files)))
	return string(result), nil
}

// ListFilesWithContext lists files with sandbox path validation
func ListFilesWithContext(input json.RawMessage, ctx interface{}) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Extract sandbox context
	sandboxCtx, ok := ctx.(sandbox.SandboxContext)
	if !ok {
		// No sandbox context available - fall back to unrestricted access
		return ListFiles(input)
	}

	log := logs.New("Agent")

	// Default to current directory if no path provided
	dir := listFilesInput.Path
	if dir == "" {
		dir = "."
	}

	// Normalize path (convert relative to virtual path)
	virtualPath := sandboxCtx.NormalizePath(dir)

	// Resolve virtual path to real filesystem path
	realPath, err := sandboxCtx.ResolvePath(virtualPath)
	if err != nil {
		log.Warn("Sandbox security: Path access denied",
			slog.String("path", dir),
			slog.String("virtual_path", virtualPath),
			slog.String("agent_scope", string(sandboxCtx.AgentScope)),
			slog.String("agent_id", sandboxCtx.CurrentAgentID))
		return "", fmt.Errorf("❌ **Sandbox Security: Access Denied**\n\nPath: %s\nVirtual: %s\nAgent Scope: %s\n\nThis agent does not have permission to access this path.\n\nAllowed paths:\n%s",
			dir,
			virtualPath,
			sandboxCtx.AgentScope,
			sandboxCtx.GetAllowedPathsDescription())
	}

	log.Debug("Sandbox security: Path access granted for listing",
		slog.String("path", dir),
		slog.String("virtual_path", virtualPath),
		slog.String("real_path", realPath),
		slog.String("agent_scope", string(sandboxCtx.AgentScope)))

	// List files from sandboxed directory
	var files []string
	err = filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(realPath, path)
		if err != nil {
			return err
		}

		// Skip .devenv directory and its contents
		if info.IsDir() && (relPath == ".devenv" || strings.HasPrefix(relPath, ".devenv/")) {
			return filepath.SkipDir
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		log.WithError(err).Error("Failed to list files", slog.String("real_path", realPath))
		return "", err
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	log.Debug("Successfully listed sandboxed files",
		slog.String("virtual_path", virtualPath),
		slog.String("real_path", realPath),
		slog.Int("count", len(files)))

	return fmt.Sprintf("✅ **Files in %s**\n\nFound %d items:\n%s", virtualPath, len(files), string(result)), nil
}

// bash tool
var BashDefinition = ToolDefinition{
	Name: "bash",
	Description: "Run ONE shell command and return its output. You are ALREADY in the project " +
		"directory — never cd to it. Issue a single command, READ its output, then decide the next " +
		"one: a batch of commands emitted together cannot be steered by what the earlier ones " +
		"returned, which is how a turn spends itself without learning anything.",
	InputSchema:         BashInputSchema,
	Function:            Bash,            // Legacy function (no sandbox enforcement)
	FunctionWithContext: BashWithContext, // Sandbox-aware function
}

type BashInput struct {
	Command string `json:"command" jsonschema_description:"A SINGLE bash command to run in the working directory (e.g. \"go build ./...\", \"go test ./...\", \"ls\"). Chain with && only when the steps are inseparable. Do not cd to the project root — you are already there."`
	// Cwd is HARNESS-INJECTED (coder-workdir confinement), not model-facing.
	//
	// `json:"-"` keeps it out of the GENERATED SCHEMA — the tag it carried until
	// 2026-08-30 was `cwd,omitempty`, which put a bare `"cwd":{"type":"string"}`
	// with no description into every bash schema the model sees. The confinement
	// still held (the harness overwrites the key), but advertising an undocumented
	// parameter invites the model to set it, and noise in a schema is not free.
	//
	// It is recovered by a SECOND unmarshal at the call sites, the same way
	// apply_patch recovers its cwd and turn_reads.
	//
	// Before 2026-08-29 bash was the only confined tool without one, so the
	// gateway had to express "run here" by string-prefixing `cd '<wd>' && ` to
	// the model's command. That is a DEFAULT directory rather than a working
	// directory: the model's own cd runs after it and wins, and a model duly
	// emitted `cd '<wd>' && cd /Users/you/.memdoor/workdir/tetris && go run` —
	// a path it invented. Setting cmd.Dir says the same thing to the operating
	// system instead of to a shell that can be talked out of it.
	Cwd string `json:"-"`
}

var BashInputSchema = GenerateSchema[BashInput]()

// ParseBashCommand extracts the command, tolerating the ways a small model malforms
// the call. Chiefly: it nests the REAL command under a "parameters" (or "input")
// object and leaves the tool NAME ("bash") in the top-level "command" field —
// e.g. {"command":"bash","parameters":{"command":"go build ./..."}}. Left as-is,
// the tool would run the literal string "bash" (a no-op that silently satisfies the
// build-gate). Unwrap the nested command when the top-level one is empty or is just
// the tool name.
func ParseBashCommand(input json.RawMessage) (string, error) {
	var bi struct {
		Command    string           `json:"command"`
		Parameters *json.RawMessage `json:"parameters"`
		Input      *json.RawMessage `json:"input"`
		// Invented fields a small model splits the command across (live:
		// {"command":"go","path":"tetris.go"} looping on bare `go` usage/exit 2).
		Path string `json:"path"`
		File string `json:"file"`
		Args string `json:"args"`
	}
	if err := json.Unmarshal(input, &bi); err != nil {
		return "", err
	}
	cmd := strings.TrimSpace(bi.Command)
	// Absorb split-command malformations: a single-word command plus a stray
	// path/file/args field is one command the model took apart — reassemble it
	// verbatim, no language-specific guessing. If the reassembled command is
	// still wrong (e.g. `go x.go` needs `go run`), the failure output echoes
	// the exact command line, which is what lets the model correct itself.
	if cmd != "" && !strings.ContainsAny(cmd, " \t") {
		for _, extra := range []string{bi.Path, bi.File, bi.Args} {
			if extra = strings.TrimSpace(extra); extra != "" {
				cmd = cmd + " " + extra
				break
			}
		}
	}
	if cmd == "" || cmd == "bash" {
		for _, nested := range []*json.RawMessage{bi.Parameters, bi.Input} {
			if nested == nil {
				continue
			}
			var inner struct {
				Command string `json:"command"`
			}
			if json.Unmarshal(*nested, &inner) == nil && strings.TrimSpace(inner.Command) != "" {
				cmd = strings.TrimSpace(inner.Command)
				break
			}
		}
	}
	if cmd == "" {
		// DOUBLE-WRAPPED CALL: the model sent the tool-call ENVELOPE as the
		// arguments — {"name":"bash"} — so there is no command anywhere and
		// nothing to recover. Say WHICH mistake it made.
		//
		// "command is required" reads as "you forgot a field" and invites the
		// same shape again; measured 2026-08-30, this arrived as a bash frame
		// labelled `Bash(name: "bash")` that did nothing. The model needs to
		// know it wrapped the call twice, not that a field was missing.
		var env struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(input, &env) == nil && env.Name != "" {
			return "", fmt.Errorf("no command: you sent the tool-call envelope {\"name\":%q} as the ARGUMENTS. "+
				"Pass the arguments only, as {\"command\":\"...\"}", env.Name)
		}
		return "", fmt.Errorf("command is required")
	}
	return cmd, nil
}

// injectedCwd reads the harness-injected working directory out of a tool input.
// It is deliberately not part of any model-facing schema, so it cannot be
// recovered by unmarshalling the public struct.
func injectedCwd(input json.RawMessage) string {
	var conf struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(input, &conf)
	return conf.Cwd
}

func Bash(input json.RawMessage) (string, error) {
	command, err := ParseBashCommand(input)
	if err != nil {
		return "", err
	}
	bashInput := BashInput{Command: command, Cwd: injectedCwd(input)}

	log := logs.New("Agent")
	log.Debug("Executing bash command", slog.String("command", bashInput.Command))
	// runPlainBash, not exec directly: tool output is read by a model, so it
	// must be text and not terminal escapes. See bash_plain.go.
	output, err := runPlainBash(bashInput.Command, bashInput.Cwd)
	if err != nil {
		return bashFailedOutput(bashInput.Command, output, injectedTurnTask(input), err), nil
	}

	log.Debug("Bash command succeeded",
		slog.String("command", bashInput.Command),
		slog.Int("output_bytes", len(output)))
	return bashReadOutput(bashInput.Command, output, injectedTurnTask(input)), nil
}

// BashWithContext executes bash with sandbox awareness
// Note: bash commands can bypass file-based sandbox restrictions
// For strong isolation, use specific file tools (read_file, write_file) instead
func BashWithContext(input json.RawMessage, ctx interface{}) (string, error) {
	command, err := ParseBashCommand(input)
	if err != nil {
		return "", err
	}
	bashInput := BashInput{Command: command, Cwd: injectedCwd(input)}

	// Extract sandbox context
	sandboxCtx, ok := ctx.(sandbox.SandboxContext)
	if !ok {
		// No sandbox context available - fall back to unrestricted access
		return Bash(input)
	}

	log := logs.New("Agent")

	// Warn if using bash with restricted scope
	// Note: We allow bash but log the security implications
	if sandboxCtx.AgentScope != sandbox.ScopeWorkspace {
		log.Warn("Sandbox security: bash command executed with restricted scope",
			slog.String("command", bashInput.Command),
			slog.String("agent_scope", string(sandboxCtx.AgentScope)),
			slog.String("agent_id", sandboxCtx.CurrentAgentID))
	}

	log.Debug("Executing bash command", slog.String("command", bashInput.Command))
	// runPlainBash, not exec directly: tool output is read by a model, so it
	// must be text and not terminal escapes. See bash_plain.go.
	output, err := runPlainBash(bashInput.Command, bashInput.Cwd)
	if err != nil {
		return bashFailedOutput(bashInput.Command, output, injectedTurnTask(input), err), nil
	}

	log.Debug("Bash command succeeded",
		slog.String("command", bashInput.Command),
		slog.Int("output_bytes", len(output)))
	return bashReadOutput(bashInput.Command, output, injectedTurnTask(input)), nil
}

// Helper function to generate JSON schema from Go struct
func GenerateSchema[T any]() llm.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	// Type:"object" is the JSON-schema contract for tool input — an
	// OpenAI-compatible server rejects the request with "Unrecognized
	// schema" when the top-level type is missing or empty. The providers'
	// converter coerces it, but the remote executor (pkg/remote/translator.go)
	// passes InputSchema through as-is. Setting it at the source fixes both.
	return llm.ToolInputSchemaParam{
		Type:       "object",
		Properties: schema.Properties,
		Required:   schema.Required,
	}
}

// bashSuccessOutput renders a successful command's output for the model. An
// EMPTY result must say so explicitly — go build/go vet succeed silently, and
// a model reads an empty tool result as "the build did not complete" (live: after
// a fully green build it asked the user for the error message).
// bashReadKind names what a bash command does when it only READS: "file"
// (cat, head, tail, sed -n …), "matches" (grep, rg …), or "" when any part of
// it does something else. A chain (a; b && c) and a pipeline count when every
// command in them reads; a redirect writes, so it never does. The chain is the
// common case: live 2026-09-28, 51 of a turn's 52 bash reads were
// "sed -n …; grep …", and the note this replaced skipped every one of them.
func bashReadKind(command string) string {
	// Silencing or merging stderr writes nothing: `2>/dev/null` is how a read
	// says "a missing path is fine".
	command = strings.NewReplacer("2>/dev/null", "", "2> /dev/null", "", "2>&1", "").Replace(command)
	segs, writes := shellSegments(command)
	if writes {
		return ""
	}
	kind := ""
	for _, seg := range segs {
		f := strings.Fields(seg)
		// Shell control words are not commands: `for f in a b`, `do head $f`,
		// `done`. Strip the keyword and judge what actually runs (live
		// 2026-10-04: a read-only for-loop over head/wc counted as a CHECK
		// and the receipt printed it with a PASS).
		for len(f) > 0 {
			switch f[0] {
			case "do", "then", "else", "elif", "if", "while", "until":
				f = f[1:]
				continue
			}
			break
		}
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "cd", "ls", "wc", "sort", "uniq", "cut", "nl", "tr", "true", "echo",
			"for", "done", "fi", "esac", "printf", "basename", "dirname":
		case "cat", "head", "tail", "less":
			if kind == "" {
				kind = "file"
			}
		case "sed", "awk":
			if len(f) < 2 || !(f[1] == "-n" || strings.HasPrefix(f[1], "'NR")) {
				return ""
			}
			if kind == "" {
				kind = "file"
			}
		case "grep", "rg", "egrep", "find":
			kind = "matches"
		default:
			return ""
		}
	}
	if kind == "" && strings.TrimSpace(command) != "" {
		// Only ls, echo, cd, wc …: it reads the tree, not a file's text.
		// Live 2026-10-04: `ls -a; echo ---; ls .agents 2>/dev/null` probed for
		// a folder that was not there and came back "Command FAILED — fix IT".
		kind = "listing"
	}
	return kind
}

// IsBashRead reports whether every command in a bash line only reads: its
// exit status is an answer (no match, no such path), never a check that failed.
func IsBashRead(command string) bool { return bashReadKind(command) != "" }

// BashFailedResult is what a command that exited non-zero returns, for every
// path that runs bash (the one-shot tool here, the gateway's streamer): one
// formatter, so a read's exit status reads as an answer on both.
func BashFailedResult(command, output string, input json.RawMessage, err error) string {
	return bashFailedOutput(command, output, injectedTurnTask(input), err)
}

// bashFailedOutput is what a command that exited non-zero returns. A
// read-only command's exit status is an ANSWER — grep finding no match, ls or
// cat naming a path that does not exist — so it comes back as the output with
// its status, not as a broken command to fix. Live 2026-09-28:
// `grep … --include="*.tsx" …; echo ---; ls web/src/pages` ran exactly as
// written, ls found no such folder, and the tool told the model to "fix IT
// (wrong subcommand, missing flag, wrong path)". A timeout, or a command that
// does anything else, is a failure as before.
func bashFailedOutput(command, output, task string, err error) string {
	if bashReadKind(command) != "" && strings.HasPrefix(err.Error(), "exit status") {
		body := strings.TrimSpace(output)
		if len(output) >= bashReadJudgeMin {
			body = bashReadOutput(command, output, task)
		}
		if body == "" {
			body = "(no output)"
		}
		return body + fmt.Sprintf("\n\n(%s. In a command that only reads this is a result, not a broken command: "+
			"grep found no match, or a path it names (ls, cat, head) does not exist.)", err.Error())
	}
	logs.New("Agent").WithError(err).Error("Bash command failed", slog.String("command", command))
	return fmt.Sprintf(CommandFailedPrefix+"%s): `%s`\nOutput: %s\nThe command line above is exactly what ran — fix IT (wrong subcommand, missing flag, wrong path) and retry; do not repeat it unchanged.", err.Error(), command, output)
}

// CommandFailedPrefix opens the result of a command that exited non-zero. The
// failure travels as OUTPUT, not as a tool error: a build that fails is work in
// progress, not a broken tool, and counting it as one would retire bash in the
// middle of a fix loop. So this prefix is how everything downstream — the
// screen's marker among them — knows what it is looking at.
const CommandFailedPrefix = "Command FAILED ("

// shellSegments splits a command line at ; & | and newlines that are not
// quoted — `grep "a\|b" x` is one command, not two — and reports whether it
// redirects or substitutes (> < outside quotes, ` or $ outside single quotes),
// which a read never needs.
func shellSegments(command string) (segs []string, writes bool) {
	var cur strings.Builder
	var quote rune
	for _, r := range command {
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case quote == '"':
			if r == '"' {
				quote = 0
			} else if r == '`' {
				writes = true
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '>' || r == '<' || r == '`':
			writes = true
		case r == ';' || r == '&' || r == '|' || r == '\n':
			segs = append(segs, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	// A bare $var expanded as an argument cannot become a new command (word
	// splitting never re-parses separators) — but $( runs one. `head -5 $f`
	// in a read-only loop was disqualified by the bare variable alone
	// (live 2026-10-04, the receipt printed the loop as a PASS check).
	if strings.Contains(command, "$(") {
		writes = true
	}
	return append(segs, cur.String()), writes
}

// bashReadJudgeMin is where a bash read starts to be judged: about 500
// tokens. The turn above read 120 KB through bash in reads of 1-4 KB each;
// a threshold at the note's 4 KB would have judged a quarter of it.
const bashReadJudgeMin = 2 << 10

// bashReadOutput is what a successful bash command returns to the model. A
// read-only command past bashReadJudgeMin is judged against the turn's task
// like jread — the model keeps its habit of reading through bash, the window
// gets only the sections that matter. Unjudged (no task, no decision model,
// judging kept nearly all of it), the output is whole, with one line naming
// the tool that reads for the task.
func bashReadOutput(command, output, task string) string {
	kind := bashReadKind(command)
	if kind == "" || len(output) < bashReadJudgeMin {
		return bashSuccessOutput([]byte(output))
	}
	if strings.TrimSpace(task) != "" && decisionService() != nil {
		label := "`" + commandLabel(command, 120) + "`"
		if out, err := judgedSections("bash", label, strings.TrimRight(output, "\n"), task, bashReadJudgeKeep); err == nil && !strings.HasPrefix(out, "UNJUDGED") {
			return fmt.Sprintf("%s printed %s; judged against this turn's task — rerun a narrower range for exact text:\n", label, humanKB(len(output))) + out
		}
	}
	return bashSuccessOutput([]byte(output)) + bashReadNote(kind, len(output))
}

const bashReadJudgeKeep = 8

// bashReadNote is the one line an UNJUDGED read through bash carries,
// naming the tool that reads for the task.
func bashReadNote(kind string, outBytes int) string {
	if kind == "listing" {
		return ""
	}
	if kind == "matches" {
		return fmt.Sprintf("\n\n(%s of matches through bash, all of them. jgrep keeps only the matches that matter, ranked.)", humanKB(outBytes))
	}
	return fmt.Sprintf("\n\n(%s read through bash, all of it. read_file or jread returns only the sections that matter for the task.)", humanKB(outBytes))
}

func commandLabel(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func humanKB(n int) string { return fmt.Sprintf("%.1f KB", float64(n)/1024) }

func bashSuccessOutput(output []byte) string {
	s := strings.TrimSpace(string(output))
	if s == "" {
		return "(command succeeded with exit code 0 — no output)"
	}
	return s
}

// read_file returns text. A binary file came back as 276 KB of mojibake
// (2026-09-21: a .png read as text), all of it resent on
// every later call; it is now named and refused with the tool that reads it.
// Text past readFileMax returns its head and the way to the rest.
const readFileMax = 128 << 10

// A text file past readFileJudgeMin, read in a turn whose task is known and
// with a decision model answering, comes back as the sections that matter for
// the task (jread's judge) unless all is set. The month's read_file output
// was 22.8 MB over 318 calls, most of it yt-dlp .source.json dumps (up to
// 9.5 MB) read for a title, a licence and a duration.
// readFileJudgeMin stays at 32 KB. Dropping it to 10 KB was tried and
// reverted on 2026-09-27: in an 8-run A/B the judged read of a mid-size file
// showed no saving the noise could not explain, and the one edit turn that
// thrashed (495 s, 15 calls) was on that build. A file the turn is about to
// patch wants to arrive whole; judging pays on the files this threshold
// already catches. What stayed from that attempt is in jev_tools.go: the keep
// cap scaled to the file, and the whole file returned when judging would keep
// nearly all of it.
const (
	readFileJudgeMin  = 32 << 10
	readFileJudgeKeep = 12
)

// injectedTurnTask reads the harness-injected turn task (the request the turn
// is working on) out of a tool input. Like cwd, it is not in any schema.
func injectedTurnTask(input json.RawMessage) string {
	var v struct {
		T string `json:"turn_task"`
	}
	_ = json.Unmarshal(input, &v)
	return v.T
}

// readFileLines is a read's default page (omp's read.defaultLimit): whole
// files were 41% of the tool output entering coder conversations, and reads
// over 300 lines 77% of that (60 transcripts, 2026-09-30).
const readFileLines = 300

func readableContent(path string, content []byte, task string, all bool, offset, limit int) string {
	sample := content
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	if bytes.IndexByte(sample, 0) >= 0 || !utf8.Valid(trimPartialRune(sample)) {
		return fmt.Sprintf("%s is a binary file (%d bytes) — not returned as text; use a tool made for that file type.", path, len(content))
	}
	paged := offset > 0 || limit > 0
	if !all && !paged && len(content) > readFileJudgeMin && strings.TrimSpace(task) != "" && decisionService() != nil {
		text := string(content)
		// Indented JSON is not the file: its line numbers could not be edited.
		if strings.EqualFold(filepath.Ext(path), ".json") && !HashlineOn() {
			var buf bytes.Buffer
			if json.Indent(&buf, content, "", " ") == nil {
				text = buf.String()
			}
		}
		if out, err := judgedSections("read_file", path, text, task, readFileJudgeKeep); err == nil && !strings.HasPrefix(out, "UNJUDGED") {
			return fmt.Sprintf("%s is %d KB; judged against this turn's task (read_file with all: true returns the whole file):\n", path, len(content)>>10) + out
		}
	}
	if HashlineOn() {
		return hashlineRead(path, string(content), readFileMax)
	}
	if !all {
		if page, ok := readPage(path, string(content), offset, limit); ok {
			return page
		}
	}
	if len(content) > readFileMax {
		return string(content[:readFileMax]) + fmt.Sprintf("\n…[truncated at %d KB of %d KB: for the parts that matter use jread with a task, or bash sed -n 'FROM,TOp' for a line range]", readFileMax>>10, len(content)>>10)
	}
	return string(content)
}

// readPage is lines offset..offset+limit-1 of text (1-based; defaults 1 and
// readFileLines) and a note of what is left, or ok=false when the whole file
// fits one page — then the read is the file itself, byte for byte, as before
// paging. A page also stops at readFileMax bytes, on a whole line; a single
// line longer than that is cut on a rune boundary and says so.
func readPage(path, text string, offset, limit int) (string, bool) {
	if offset < 1 {
		offset = 1
	}
	if limit < 1 {
		limit = readFileLines
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	if offset == 1 && limit >= total && len(text) <= readFileMax {
		return "", false
	}
	if offset > total {
		return fmt.Sprintf("%s has %d lines; line %d is past the end. read_file with offset: 1 reads from the start.", path, total, offset), true
	}
	end := min(offset-1+limit, total)
	var b strings.Builder
	shown := offset - 1
	for i := offset - 1; i < end; i++ {
		if b.Len()+len(lines[i])+1 > readFileMax {
			if i == offset-1 { // one line longer than a page
				cut := string(trimPartialRune([]byte(lines[i][:readFileMax])))
				b.WriteString(cut)
				fmt.Fprintf(&b, "\n\n[line %d of %s is %d KB; the first %d KB are shown. bash with cut or head -c reads the rest]", i+1, path, len(lines[i])>>10, len(cut)>>10)
				if i+1 < total {
					fmt.Fprintf(&b, "\n[%d more lines (%d in all). read_file with offset: %d to continue]", total-i-1, total, i+2)
				}
				return b.String(), true
			}
			break
		}
		if i > offset-1 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
		shown = i + 1
	}
	if shown < total {
		fmt.Fprintf(&b, "\n\n[%d more lines in %s (%d in all). read_file with offset: %d to continue]", total-shown, path, total, shown+1)
	} else {
		fmt.Fprintf(&b, "\n\n[lines %d-%d of %s, the end]", offset, shown, path)
	}
	return b.String(), true
}

// trimPartialRune drops a UTF-8 sequence cut at the end of a sample.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < 3 && len(b) > 0; i++ {
		if r, _ := utf8.DecodeLastRune(b); r != utf8.RuneError {
			return b
		}
		b = b[:len(b)-1]
	}
	return b
}
