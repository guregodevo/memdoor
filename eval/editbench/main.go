// edit-bench runner: HASHLINE.md §7. One process drives the runs of one
// model × edit_format end to end against OpenRouter, with the coder's own
// read_file and apply_patch (the memdoor/tools package, not a copy), then
// gates each on `go test ./...`.
//
//	go run ./eval/editbench -model <openrouter/model> -format patch|hashline
//
// The caller (a shell loop) runs the 2 formats × 2 models × 20 tasks = 80
// runs and collects eval/editbench/results/*.json.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"memdoor/gateway/logs"
)

type runResult struct {
	Model         string  `json:"model"`
	Format        string  `json:"format"`
	Task          string  `json:"task"`
	Pass          bool    `json:"pass"`
	OutputTokens  int     `json:"output_tokens"`
	InputTokens   int     `json:"input_tokens"`
	PatchCalls    int     `json:"patch_calls"`
	PatchFailures int     `json:"patch_failures"`
	TurnSeconds   float64 `json:"turn_seconds"`
	LastReply     string  `json:"last_reply,omitempty"` // the model's final text, for a run that stopped
	Error         string  `json:"error,omitempty"`
}

var systemPrompt = `You are editing a small Go module. Make "go test ./..." pass.
Use the read_file tool to see a file and apply_patch to edit it. Work alone;
when the tests pass, reply DONE and stop calling tools.`

func main() {
	model := flag.String("model", "", "OpenRouter model id")
	format := flag.String("format", "patch", "edit_format: patch | hashline")
	taskIdx := flag.Int("task", -1, "task index (default: all)")
	flag.Parse()
	if *model == "" {
		fmt.Fprintln(os.Stderr, "-model required")
		os.Exit(2)
	}
	if os.Getenv("OPEN_ROUTER_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "OPEN_ROUTER_API_KEY is not set")
		os.Exit(2)
	}
	// The tools package logs through the gateway's logger; the runs log to a
	// temp dir, not to the gateway's store.
	if err := logs.InitGlobalLogger(filepath.Join(os.TempDir(), "editbench-logs"), false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	all := tasks()
	idx := []int{}
	for i := range all {
		if *taskIdx < 0 || i == *taskIdx {
			idx = append(idx, i)
		}
	}
	for _, i := range idx {
		// Resume: skip a task whose result file already exists. Results land
		// in eval/editbench/results of the repo (rootDir: the dir holding the
		// module's go.mod), wherever the runner is launched from.
		resultsDir := filepath.Join(rootDir(), "eval", "editbench", "results")
		msan := sanitize(*model)
		name := fmt.Sprintf("%s__%s__%s.json", msan, *format, all[i].name)
		if _, err := os.Stat(filepath.Join(resultsDir, name)); err == nil {
			fmt.Printf("%-40s skipped (result exists)\n", all[i].name)
			continue
		}
		res := runOne(*model, *format, all[i])
		b, _ := json.Marshal(res)
		_ = os.MkdirAll(resultsDir, 0o755)
		_ = os.WriteFile(filepath.Join(resultsDir, name), b, 0o644)
		fmt.Printf("%-40s pass=%v out=%d patch_calls=%d fails=%d %.1fs %s\n",
			all[i].name, res.Pass, res.OutputTokens, res.PatchCalls, res.PatchFailures, res.TurnSeconds, res.Error)
	}
}

// errUnknownTool answers a call to a tool the run does not offer.
var errUnknownTool = errors.New("unknown tool")

// rootDir walks up from the CWD to the dir holding go.mod (the repo root).
func rootDir() string {
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '/' || r == ':' || r == ' ' {
			r = '_'
		}
		out = append(out, r)
	}
	return string(out)
}

// ---- tools the model sees --------------------------------------------------

// tools moved to tools.go

// ---- the harness -----------------------------------------------------------

const maxTurns = 12

func runOne(model, format string, tk task) runResult {
	res := runResult{Model: model, Format: format, Task: tk.name}
	start := time.Now()

	tmp, err := os.MkdirTemp("", "editbench")
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer os.RemoveAll(tmp)
	// The real path (macOS temp dirs sit behind /var -> /private/var): the
	// same string is the process's working directory and apply_patch's cwd.
	dir, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	gomod := "module bench\n\ngo 1.21\n"
	for path, content := range tk.files {
		full := filepath.Join(dir, path)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			res.Error = err.Error()
			return res
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	// Confirm the task is really broken before spending tokens.
	if pass, _ := goTest(dir); pass {
		res.Error = "fixture already passes"
		return res
	}

	turns := []map[string]any{{"role": "system", "content": systemPrompt + "\n\nWorking directory: " + dir}}
	var patchCalls, patchFails int
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		res.Error = err.Error()
		return res
	}
	defer os.Chdir(prev)

	for turn := 0; turn < maxTurns; turn++ {
		msgs, usage, err := chat(model, turns, toolsFor(format))
		if err != nil {
			res.Error = err.Error()
			break
		}
		res.InputTokens += usage.Input
		res.OutputTokens += usage.Output
		turns = append(turns, msgs...)

		toolCalls := extractToolCalls(lastAssistant(msgs))
		if len(toolCalls) == 0 {
			if c, ok := lastAssistant(msgs)["content"].(string); ok {
				res.LastReply = c
			}
			break
		}
		for _, tc := range toolCalls {
			out, err := runTool(dir, tc.Name, tc.Arguments)
			if tc.Name == "apply_patch" {
				patchCalls++
				if err != nil {
					patchFails++
				}
			}
			if err != nil {
				out = "error: " + err.Error()
			}
			turns = append(turns, map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": out})
		}
		if pass, _ := goTest(dir); pass {
			res.Pass = true
			break
		}
	}
	res.PatchCalls = patchCalls
	res.PatchFailures = patchFails
	res.TurnSeconds = time.Since(start).Seconds()
	if !res.Pass && res.Error == "" {
		res.Error = "no pass within maxTurns"
	}
	return res
}

func goTest(dir string) (bool, string) {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return err == nil, out.String()
}
