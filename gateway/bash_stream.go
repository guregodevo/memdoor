package gateway

import (
	"context"
	"encoding/json"
	"memdoor/pkg/secrets"
	"memdoor/tools"
	"os/exec"
	"sync"
	"time"
)

// streamingExecutor runs a tool that produces output incrementally, calling emit
// with each chunk as it is produced and returning the final captured output.
// executeTool prefers this path over a tool's one-shot Function so the TUI can
// render a live tail. Any tool becomes streaming by registering here — no
// per-name branch in executeTool.
type streamingExecutor interface {
	Stream(ctx context.Context, input json.RawMessage, emit func(chunk string)) (output string, err error)
}

// streamerFor returns how a tool streams, or nil if it runs through the normal
// one-shot Function path. Add a case to make a new tool stream — executeTool
// needs no change. A switch (not a name== check scattered in executeTool) keeps
// the dispatch in one place and returns the streamingExecutor interface.
func streamerFor(toolName string) streamingExecutor {
	switch toolName {
	case "bash":
		return bashStreamer{}
	default:
		return nil
	}
}

// bashStreamer streams `bash -c <command>` and formats the result exactly as the
// non-streaming bash tool did — including the "Command FAILED — fix it" message
// the coder acts on, and a note when the time or output limit cut a long command short.
type bashStreamer struct{}

func (bashStreamer) Stream(ctx context.Context, input json.RawMessage, emit func(chunk string)) (string, error) {
	// THE SAME parser as the one-shot bash tool — not a second unmarshal. The
	// streamer's own bare decode discarded its error and ran an empty command
	// as `bash -c ""`: a silent success that taught the model nothing
	// (measured live 2026-08-31, bash {} from the coder). One parser means the
	// two paths absorb the same malformations and refuse with the same words.
	command, perr := tools.ParseBashCommand(input)
	if perr != nil {
		return "", perr
	}
	var in struct {
		Cwd string `json:"cwd"` // harness-injected workdir; see tools.BashInput
	}
	_ = json.Unmarshal(input, &in)
	out, truncated, runErr := runStreamingCommand(ctx, command, in.Cwd, emit)
	if runErr != nil {
		return tools.BashFailedResult(command, out, input, runErr), nil
	}
	if truncated {
		return out + "\n[stopped: the command was still running at the time or output limit, and it was killed with everything it started, background jobs too. " +
			"To run something longer, start it alone in a command that returns at once — nohup <cmd> > <log> 2>&1 & — and read the log in later commands]", nil
	}
	return out, nil
}

// Bounds on a streamed bash call so a non-terminating command (tail -f) can't
// hang the turn. Vars, not consts, so tests can shrink them.
//
// The time bound is the one-shot bash tool's (tools.BashTimeout,
// tools.BashTimeout, 300 s) when bashStreamMaxDuration is 0. It
// was 30 s of its own: that cut every longer build, test or wait the coder ran
// from the TUI, and killed with it the job the command had started in the
// background (live 2026-09-30: an 80-run sweep started with `nohup … &` and a
// `sleep` in one command died at 30 s).
var (
	bashStreamMaxDuration time.Duration // 0: tools.BashTimeout()
	bashStreamMaxBytes    = 256 * 1024
	bashStreamFlushEvery  = 100 * time.Millisecond
)

// runStreamingCommand runs `bash -c command`, calling emit with each new chunk
// of combined stdout/stderr as it is produced (coalesced to at most one call
// per bashStreamFlushEvery so a flood does not become a flood of events). It
// returns the full captured output and whether it was cut short by the time or
// size cap — in which case the process is killed and the turn continues.
//
// This is what lets the TUI render a live tail: emit feeds the streaming pane
// while the command runs; the returned string is the final tool result.
func runStreamingCommand(ctx context.Context, command, cwd string, emit func(chunk string)) (output string, truncated bool, runErr error) {
	limit := bashStreamMaxDuration
	if limit == 0 {
		limit = tools.BashTimeout()
	}
	cctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	var mu sync.Mutex
	var buf []byte     // everything captured (capped)
	var pending []byte // produced since the last flush
	capped := false

	// A POINTER, so Stdout and Stderr compare equal and os/exec hands the
	// child ONE pipe: output arrives in the order the command wrote it. A
	// func value is not comparable, so exec opened two pipes with a copying
	// goroutine each, and stderr could land before the stdout written
	// ahead of it — "bash: apply_patch: command not found" came back on the
	// same line as the "Output:" label, after "start" should have, and the
	// hint that reads bash's own message never fired (public CI, 2026-10-07).
	w := &streamWriter{write: func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		if capped {
			return len(p), nil // keep draining so the pipe doesn't block; ignore
		}
		room := bashStreamMaxBytes - len(buf)
		if room <= 0 {
			capped = true
			cancel() // size cap hit → kill the process
			return len(p), nil
		}
		if len(p) > room {
			p = p[:room]
			capped = true
			cancel()
		}
		buf = append(buf, p...)
		pending = append(pending, p...)
		return len(p), nil
	}}

	cmd := exec.CommandContext(cctx, "bash", "-c", command)
	// The workdir comes from the harness, not from a cd inside the command.
	cmd.Dir = cwd
	// The cap kills the whole process group, not only bash. Killing bash
	// alone left its child (sleep, find) holding the output pipe, and Run
	// waited for it: a 30 s cap held a coder turn 401 s on `sleep 401`, and
	// for over an hour on `find ~ -maxdepth 4 …` (2026-09-25). WaitDelay
	// bounds the wait for the pipe should anything escape the group.
	tools.OwnProcessGroup(cmd)
	cmd.Cancel = func() error { return tools.KillProcessGroup(cmd) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout = w
	cmd.Stderr = w

	// Flush pending output to emit on a fixed cadence while the command runs.
	done := make(chan struct{})
	var flushWG sync.WaitGroup
	flushWG.Add(1)
	go func() {
		defer flushWG.Done()
		t := time.NewTicker(bashStreamFlushEvery)
		defer t.Stop()
		flush := func() {
			mu.Lock()
			chunk := pending
			pending = nil
			mu.Unlock()
			if len(chunk) > 0 && emit != nil {
				// What the window shows live is redacted like the result
				// (agent_runtime_tools.go redactToolResult); a key split across
				// two flushes is the one case this misses.
				emit(secrets.RedactText(string(chunk)))
			}
		}
		for {
			select {
			case <-t.C:
				flush()
			case <-done:
				flush() // final flush
				return
			}
		}
	}()

	err := cmd.Run()
	close(done)
	flushWG.Wait()

	mu.Lock()
	out := string(buf)
	wasCapped := capped
	mu.Unlock()

	// A timeout (deadline exceeded) also counts as truncated even if the byte
	// cap was not hit — the command was still running when we cut it off.
	if wasCapped || cctx.Err() == context.DeadlineExceeded {
		truncated = true
	}
	// A cap we imposed is not a command failure — the caller reports truncation
	// separately, so don't surface the "signal: killed" as an exit error.
	if !truncated {
		runErr = err
	}
	return out, truncated, runErr
}

// streamWriter is the one io.Writer a streamed command's stdout and stderr
// share. It is a pointer type on purpose: see runStreamingCommand.
type streamWriter struct {
	write func([]byte) (int, error)
}

func (s *streamWriter) Write(p []byte) (int, error) { return s.write(p) }
