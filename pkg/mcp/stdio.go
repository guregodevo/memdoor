package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// NewStdio is a client that runs spec.Command and speaks MCP over its stdin
// and stdout, one JSON message per line.
func NewStdio(spec Spec, logger Logger) Client {
	c := newClient(nil, spec.Name, logger)
	c.t = &stdioTransport{spec: spec, logger: c.logger, pending: newPending()}
	return c
}

// stderrKeep is how many of the server's last stderr lines an error quotes.
const stderrKeep = 12

type stdioTransport struct {
	spec   Spec
	logger Logger

	cmd   *exec.Cmd
	stdin io.WriteCloser
	wmu   sync.Mutex // one message on stdin at a time

	mu      sync.Mutex // guards stderr
	pending *pending
	stderr  []string
	exited  chan struct{}
	// stderrDone closes when the stderr reader has drained the pipe: a dying
	// server's last words arrive there, and an error composed at stdout EOF
	// must not run ahead of them.
	stderrDone chan struct{}
}

func (t *stdioTransport) open(ctx context.Context) error {
	if t.cmd != nil {
		return fmt.Errorf("MCP client already started")
	}
	// Not CommandContext: the server outlives the request that started it.
	t.cmd = exec.Command(t.spec.Command, t.spec.Args...)
	t.cmd.Dir = t.spec.Dir
	// The server runs on its own Node, not the parent's: a NODE_OPTIONS
	// inherited from a developer's shell named a preload file that had
	// been cleaned up overnight, and every browser start died with
	// "Cannot find module" (2026-09-14 14:45).
	t.cmd.Env = withEnv(withoutNodeOptions(os.Environ()), t.spec.Env)
	// Its own process group, so it is killed WITH its children (an MCP
	// server that launched Chrome leaves the browser running otherwise).
	setProcessGroup(t.cmd)

	var err error
	if t.stdin, err = t.cmd.StdinPipe(); err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	if err := t.cmd.Start(); err != nil {
		if errorsIsNotFound(err) {
			return fmt.Errorf("command not found: %s", t.spec.Command)
		}
		return fmt.Errorf("failed to start MCP server: %w", err)
	}
	t.exited = make(chan struct{})
	t.stderrDone = make(chan struct{})
	go t.readStderr(stderr)
	go t.readStdout(stdout)
	t.logger.Info("MCP server started", slog.String("server", t.spec.Name), slog.Int("pid", t.cmd.Process.Pid))
	return nil
}

// readStdout is the only reader: each line is a response for a waiting
// request, a request from the server (answered), or a notification.
// Anything that is not JSON (a server logging on stdout) is skipped.
func (t *stdioTransport) readStdout(r io.Reader) {
	sc := bufio.NewScanner(r)
	// A page snapshot in one result can be several MB.
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var m message
		if json.Unmarshal(line, &m) != nil {
			t.logger.Debug("MCP server stdout (not JSON)", slog.String("server", t.spec.Name), slog.String("line", truncate(string(line), 200)))
			continue
		}
		t.pending.route(m, func(b []byte) { _ = t.write(b) })
	}
	err := sc.Err()
	if err == nil {
		err = io.EOF
	}
	// THE REASON COMES ON STDERR, AND STDOUT CLOSES FIRST. A server that
	// crashes writes "database is locked" to stderr and exits; both pipes
	// close, in whatever order the scheduler reads them. On Linux CI stdout's
	// EOF won the race every time and the error quoted nothing — "the server
	// exited (EOF)" — while macOS happened to read stderr first. Wait for the
	// stderr reader to drain, briefly, before composing what the person sees.
	select {
	case <-t.stderrDone:
	case <-time.After(2 * time.Second):
	}
	t.mu.Lock()
	tail := t.stderrTail()
	t.mu.Unlock()
	t.pending.fail(fmt.Errorf("the server exited (%v)%s", err, tail))
	close(t.exited)
}

func (t *stdioTransport) readStderr(r io.Reader) {
	defer close(t.stderrDone)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		t.logger.Warn("MCP server stderr", slog.String("server", t.spec.Name), slog.String("line", line))
		t.mu.Lock()
		t.stderr = append(t.stderr, line)
		if len(t.stderr) > stderrKeep {
			t.stderr = t.stderr[len(t.stderr)-stderrKeep:]
		}
		t.mu.Unlock()
	}
}

// stderrTail is the server's last words, for an error. Caller holds mu.
//
// Only lines that say something: a launcher that prints a banner and exits
// filled the add box with box-drawing characters and left the reason out of
// sight (live 2026-10-01, server-everything given an argument it does not
// take). Framing, rules and empty lines go; the last few sentences stay.
func (t *stdioTransport) stderrTail() string {
	var said []string
	for _, l := range t.stderr {
		if l = strings.TrimSpace(strings.Trim(l, "|-_=*# 	")); l == "" {
			continue
		}
		if !strings.ContainsFunc(l, func(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }) {
			continue // a rule or a frame, nothing to read
		}
		said = append(said, l)
	}
	if len(said) == 0 {
		return ""
	}
	if len(said) > 3 {
		said = said[len(said)-3:]
	}
	return ": " + truncate(strings.Join(said, " | "), 300)
}

func (t *stdioTransport) write(b []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	_, err := t.stdin.Write(append(b, '\n'))
	return err
}

func (t *stdioTransport) call(ctx context.Context, id int64, method string, params interface{}) (json.RawMessage, error) {
	b, err := encodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}
	if t.cmd == nil {
		return nil, fmt.Errorf("MCP client not started")
	}
	return t.pending.call(ctx, id, method, t.spec.Name, func() error {
		if err := t.write(b); err != nil {
			return fmt.Errorf("failed to write request: %w", err)
		}
		return nil
	})
}

func (t *stdioTransport) notify(_ context.Context, method string, params interface{}) error {
	b, err := encodeNotification(method, params)
	if err != nil {
		return err
	}
	return t.write(b)
}

func (t *stdioTransport) negotiated(string) {}

// close ends the server: stdin closed, two seconds to exit, then the whole
// process group is killed (chrome-devtools-mcp does not always exit on a
// closed stdin, and its Chrome would outlive it).
func (t *stdioTransport) close() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	if t.stdin != nil {
		t.stdin.Close()
	}
	done := make(chan error, 1)
	go func() { done <- t.cmd.Wait() }()
	select {
	case err := <-done:
		t.cmd = nil
		return err
	case <-time.After(2 * time.Second):
		pid := t.cmd.Process.Pid
		if err := killProcessGroup(t.cmd, pid); err != nil {
			_ = t.cmd.Process.Kill()
		}
		<-done
		t.cmd = nil
		t.logger.Info("MCP server stopped (forced)", slog.String("server", t.spec.Name))
		return nil
	}
}

// withEnv is env with extra set over it, in a stable order.
func withEnv(env []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return env
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env)+len(extra))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if _, over := extra[k]; !over {
			out = append(out, kv)
		}
	}
	for _, k := range keys {
		out = append(out, k+"="+extra[k])
	}
	return out
}

// withoutNodeOptions drops the variables that make Node load something
// before the server: the server's own dependencies are all it needs.
func withoutNodeOptions(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "NODE_OPTIONS=") || strings.HasPrefix(kv, "NODE_EXTRA_CA_CERTS=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
