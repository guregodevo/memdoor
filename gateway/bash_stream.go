package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"memdoor/pkg/secrets"
	"memdoor/pkg/shell"
	"memdoor/tools"
	"os"
	"os/exec"
	"path/filepath"
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
	limit := tools.ParseBashTimeout(input)
	r := runStreamingCommand(ctx, command, in.Cwd, limit, emit)
	switch {
	case r.runErr != nil:
		return tools.BashFailedResult(command, r.out, input, r.runErr), nil
	case r.running:
		return r.out + fmt.Sprintf("\n[still running after %s, not stopped: process group %d. Its output goes on in %s, "+
			"and its last line says when it ended. Read the log (tail %s) to follow it, or stop it with kill -- -%d "+
			"when it is not worth more time. It is stopped when this turn ends.]", limit.Round(time.Second), r.pid, r.log, r.log, r.pid), nil
	case r.capped:
		return r.out + "\n[stopped: the command printed more than the output limit, and it was killed with everything it started, background jobs too. " +
			"Narrow its output (grep, head, tail), or send it to a file — <cmd> > <log> 2>&1 — and read the log]", nil
	}
	return r.out, nil
}

// Bounds on a streamed bash call so a non-terminating command (tail -f) can't
// hang the turn. Vars, not consts, so tests can shrink them.
//
// The time bound is the call's own (tools.ParseBashTimeout: its timeout when
// the config offers one, else tools.BashTimeout, 300 s); bashStreamMaxDuration,
// when set, lowers it for a test. It
// was 30 s of its own: that cut every longer build, test or wait the coder ran
// from the TUI, and killed with it the job the command had started in the
// background (live 2026-09-30: an 80-run sweep started with `nohup … &` and a
// `sleep` in one command died at 30 s).
var (
	bashStreamMaxDuration time.Duration // 0: the call's own bound
	bashStreamMaxBytes    = 256 * 1024
	bashStreamFlushEvery  = 100 * time.Millisecond
)

// streamRun is how a streamed command ended: its output, and either its
// exit (runErr), a cut (capped: the byte cap stopped it), or still running
// at the call's time limit (running, its pid and the log its output goes
// on to).
type streamRun struct {
	out     string
	capped  bool
	running bool
	pid     int
	log     string
	runErr  error
}

// bashLeftLogMax bounds the log a command left running writes to.
var bashLeftLogMax int64 = 64 << 20

// runStreamingCommand runs `bash -c command`, calling emit with each new chunk
// of combined stdout/stderr as it is produced (coalesced to at most one call
// per bashStreamFlushEvery so a flood does not become a flood of events).
//
// A command still running at limit is not killed: some take longer, and
// that is the model's call, not the harness's ("no hard timeout", 2026-11-25:
// a karpathy coder's exhaustive search ran into bash's 300 s kill). The call
// returns its output so far; the command goes on in its process group, its
// output appended to a log, until it exits or ctx ends (the turn, or Esc),
// which stops it and everything it started. The byte cap still stops a
// command whose output floods (yes, tail -f).
//
// This is what lets the TUI render a live tail: emit feeds the streaming pane
// while the command runs; the returned output is the final tool result.
func runStreamingCommand(ctx context.Context, command, cwd string, limit time.Duration, emit func(chunk string)) streamRun {
	if bashStreamMaxDuration != 0 {
		limit = min(limit, bashStreamMaxDuration)
	}
	cctx, cancel := context.WithCancel(ctx)
	left := false
	defer func() {
		if !left {
			cancel()
		}
	}()

	var mu sync.Mutex
	var buf []byte     // everything captured (capped)
	var pending []byte // produced since the last flush
	capped := false
	var logf *os.File // set once the call returned with the command running
	var logged int64

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
		if logf != nil {
			if logged < bashLeftLogMax {
				n, _ := logf.Write(p[:min(int64(len(p)), bashLeftLogMax-logged)])
				logged += int64(n)
			}
			return len(p), nil
		}
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

	// The shell is the machine's (pkg/shell): bash where there is one,
	// PowerShell on a Windows without it.
	cmd := shell.Command(cctx, command)
	// The workdir comes from the harness, not from a cd inside the command.
	cmd.Dir = cwd
	// A stop kills the whole process group, not only bash. Killing bash
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

	if err := cmd.Start(); err != nil {
		close(done)
		flushWG.Wait()
		return streamRun{runErr: err}
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	var err error
	select {
	case err = <-exited:
	case <-time.After(limit):
		if r, ok := leaveRunning(cmd, &mu, &buf, &logf, done, &flushWG); ok {
			left = true
			go func() {
				var werr error
				select {
				case werr = <-exited:
				case <-turnEnded(ctx):
					cancel() // the turn is over: so is what it left running
					werr = <-exited
				}
				mu.Lock()
				if logf != nil {
					fmt.Fprintf(logf, "\n[memdoor: the command ended: %s]\n", exitWords(werr))
					logf.Close()
				}
				mu.Unlock()
				cancel()
			}()
			return r
		}
		cancel() // no log to leave it with: stopped, as before
		<-exited
		close(done)
		flushWG.Wait()
		mu.Lock()
		defer mu.Unlock()
		return streamRun{out: tools.CommandText(buf), runErr: fmt.Errorf("still running after %s, and stopped: no log could be made for it to go on in", limit)}
	}
	// The command exited; what Run waited WaitDelay for was a backgrounded
	// child still holding the output pipe (`(…) & echo bg`). That is the
	// shape of running something in the background, not a failure: the
	// exit status decides (live: "Command FAILED (exec: WaitDelay expired
	// before I/O complete)" on an exit-0 launch, twice in one turn, 2026-10-10).
	err = ignoreWaitDelayAfterExit(cmd, err)
	close(done)
	flushWG.Wait()

	mu.Lock()
	r := streamRun{out: tools.CommandText(buf), capped: capped || cctx.Err() != nil && ctx.Err() == nil}
	mu.Unlock()
	// A cap we imposed is not a command failure — the caller reports it
	// separately, so don't surface the "signal: killed" as an exit error.
	if !r.capped {
		r.runErr = err
	}
	return r
}

type turnEndKey struct{}

// withTurnEnd marks ctx with a channel end closes when the turn returns: a
// command a bash call left running is stopped then (runStreamingCommand).
func withTurnEnd(ctx context.Context) (context.Context, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return context.WithValue(ctx, turnEndKey{}, ch), func() { once.Do(func() { close(ch) }) }
}

// turnEnded is ctx's turn-end channel; nil (never ready) outside a turn.
func turnEnded(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(turnEndKey{}).(chan struct{})
	return ch
}

// leaveRunning moves a command still running at the call's limit to a log
// of its own: what it printed so far is the call's output and the log's
// start. False when no log can be made.
func leaveRunning(cmd *exec.Cmd, mu *sync.Mutex, buf *[]byte, logf **os.File, done chan struct{}, flushWG *sync.WaitGroup) (streamRun, bool) {
	f, err := os.Create(filepath.Join(os.TempDir(), fmt.Sprintf("memdoor-bash-%d.log", cmd.Process.Pid)))
	if err != nil {
		return streamRun{}, false
	}
	close(done)
	flushWG.Wait()
	mu.Lock()
	defer mu.Unlock()
	_, _ = f.Write(*buf)
	*logf = f
	return streamRun{out: tools.CommandText(*buf), running: true, pid: cmd.Process.Pid, log: f.Name()}, true
}

// exitWords says how a command left running ended, for the end of its log.
func exitWords(err error) string {
	var ee *exec.ExitError
	switch {
	case err == nil:
		return "exit 0"
	case errors.As(err, &ee) && ee.Exited():
		return fmt.Sprintf("exit %d", ee.ExitCode())
	default:
		return err.Error()
	}
}

// ignoreWaitDelayAfterExit returns nil for exec.ErrWaitDelay when the process
// itself exited 0: its output up to the delay was captured, and the pipe was
// held by a child it left running on purpose.
func ignoreWaitDelayAfterExit(cmd *exec.Cmd, err error) error {
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return nil
	}
	return err
}

// streamWriter is the one io.Writer a streamed command's stdout and stderr
// share. It is a pointer type on purpose: see runStreamingCommand.
type streamWriter struct {
	write func([]byte) (int, error)
}

func (s *streamWriter) Write(p []byte) (int, error) { return s.write(p) }
