package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/client"
)

// memdoor run: the headless turn (Greg, 2026-10-11, after OpenRouter's
// "create a headless agent" cookbook: a prompt from a flag or stdin, text or
// an event stream out, an exit code the pipeline acts on). The same door the
// editor uses (gatewayRunner, acp.go), with a sink that prints instead of
// drawing: a CI job, a script, a queue worker gets the coder on this project
// with no window, and `memdoor resume <id>` opens the conversation later.
var (
	runDirFlag          string
	runConversationFlag string
	runJSONFlag         bool
	runYesFlag          bool
)

// Exit codes: the pipeline's contract.
const (
	runExitOK         = 0   // the turn ended; what it changed was checked, or nothing needed a check
	runExitError      = 1   // the gateway, the provider or the turn failed
	runExitUnverified = 2   // the turn ended with an unverified change or a failing check
	runExitAsked      = 3   // the agent asked a question nobody was there to answer (see --yes)
	runExitInterrupt  = 130 // ctrl-c or SIGTERM ended it; the conversation keeps what ran
)

var headlessRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "A headless turn: prompt in, answer out, an exit code for the pipeline",
	Long: `Run one coder turn on this project with no window.

The prompt comes from the arguments, or from stdin when there are none; with
both, what is piped in is the input the prompt is about (a log, a diff, a
list of files). The answer streams to stdout as text, everything else goes
to stderr, so the command composes:

  git diff | memdoor run "review this diff; one line per finding"
  memdoor run --json "…" | jq -r 'select(.event=="done") | .receipt'

Text out; --json streams NDJSON events instead
(text, tool_start, tool_done, question, done). The exit code is the turn's
outcome, read from its receipt:

  0  ended, and what it changed was checked (or nothing needed a check)
  1  the gateway, the provider or the turn failed
  2  ended unverified: a change with no check after it, or a failing check
  3  the agent asked a question in a pipe, nobody could answer, the default
     was taken; --yes answers the first option instead. At a terminal you
     answer the question there and the code is the receipt's.

  memdoor run "make the retry wait configurable and add a test"
  echo "summarize the failing tests" | memdoor run --json
  memdoor run --dir ../svc --yes "bump the Go version and run the tests"

The gateway starts if none is running. Each run is a conversation:
its id is printed at the end, and memdoor resume <id> opens it in a window.
--conversation <id> continues one instead of starting a new one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := promptFromArgsAndStdin(args, os.Stdin)
		if err != nil {
			return err
		}
		if err := firstRunReady(); err != nil {
			return err
		}
		dir := runDirFlag
		c := NewClient()
		if _, local := localGatewayPort(); !local {
			// The project is on the gateway's machine: --dir names it there,
			// else the same path as here when it has one, else its home.
			abs, derr := remoteProjectDir(c, dir)
			if derr != nil {
				return derr
			}
			dir = abs
		} else {
			if dir == "" {
				dir, _ = os.Getwd()
			}
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}
		}
		// A turn that opens with "@name " runs as that agent, as the window
		// routes it: a peer's agent (@box:coder) included.
		agent, text := ui.RouteByMention(acpAgentName, text)
		runner := &gatewayRunner{c: c, workspace: workspaceSlug, agent: agent, feeds: map[string]*client.Client{}}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		conv := runConversationFlag
		if conv != "" && len(conv) < 36 {
			// A prefix, as memdoor resume takes one (2026-10-24: the full id
			// was required here and the eight characters a run prints were
			// "no such conversation").
			rec, rerr := findSessionByID(loadSessionIndex(), conv)
			if rerr != nil {
				return rerr
			}
			conv = rec.ChannelID
		}
		if conv == "" {
			var err error
			if conv, err = runner.NewConversation(ctx, dir); err != nil {
				return err
			}
		}
		sink := &headlessSink{out: os.Stdout, errOut: os.Stderr, json: runJSONFlag, yes: runYesFlag}
		if stdinIsTerminal(os.Stdin) {
			// A person at the terminal answers the agent's questions there
			// (live 2026-10-11: an answer typed at a headless run went
			// nowhere, the default was taken, and the run exited 3).
			sink.answers = bufio.NewReader(os.Stdin)
		}
		err = runner.Run(ctx, conv, dir, text, sink)
		code := sink.finish(conv, err)
		if code != runExitOK {
			os.Exit(code)
		}
		return nil
	},
}

// headlessSink prints a turn as it happens and keeps what decides the exit
// code: the receipt line and whether a question went unanswered.
type headlessSink struct {
	out, errOut io.Writer
	json, yes   bool
	answers     *bufio.Reader // a terminal on stdin: questions are answered there

	mu      sync.Mutex
	tail    string // the last text, for the receipt
	receipt string
	asked   bool
	wrote   bool
}

func (s *headlessSink) event(kind string, fields map[string]any) {
	fields["event"] = kind
	b, _ := json.Marshal(fields)
	fmt.Fprintln(s.out, string(b))
}

func (s *headlessSink) Text(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tail += t
	if len(s.tail) > 4000 {
		s.tail = s.tail[len(s.tail)-4000:]
	}
	if r := receiptIn(s.tail); r != "" {
		s.receipt = r
	}
	if s.json {
		s.event("text", map[string]any{"text": t})
		return
	}
	fmt.Fprint(s.out, t)
	s.wrote = true
}

func (s *headlessSink) ToolStart(id, name string, input json.RawMessage) {
	if !s.json {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	in := string(input)
	if len(in) > 2000 {
		in = in[:2000] + "…"
	}
	s.event("tool_start", map[string]any{"id": id, "tool": name, "input": in})
}

func (s *headlessSink) ToolDone(id string, output string, failed bool) {
	if !s.json {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(output) > 2000 {
		output = output[:2000] + "…"
	}
	s.event("tool_done", map[string]any{"id": id, "output": output, "failed": failed})
}

// Ask: with --yes the first option (Yes, for an approval); at a terminal
// the person answers on stderr's prompt (a number or the text, Enter for
// the default); in a pipe with neither the question is printed, left to
// its default, and the run exits 3.
func (s *headlessSink) Ask(question string, options []string, approval string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	answerable := s.yes || s.answers != nil
	if s.json {
		s.event("question", map[string]any{"question": question, "options": options, "approval": approval, "answered": answerable})
	}
	if s.yes {
		if approval != "" {
			return "Yes"
		}
		if len(options) > 0 {
			return options[0]
		}
		return ""
	}
	fmt.Fprintf(s.errOut, "\n? %s\n", question)
	for i, o := range options {
		fmt.Fprintf(s.errOut, "  %d. %s\n", i+1, o)
	}
	if s.answers == nil {
		s.asked = true
		return ""
	}
	fmt.Fprint(s.errOut, "> ")
	line, _ := s.answers.ReadString('\n')
	return pickAnswer(strings.TrimSpace(line), options)
}

// pickAnswer: a number picks that option, text is the answer, "" the default.
func pickAnswer(line string, options []string) string {
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(options) {
		return options[n-1]
	}
	return line
}

func stdinIsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// finish prints the end of the run and returns its exit code.
func (s *headlessSink) finish(conversation string, err error) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := runExitCode(err, s.asked, s.receipt)
	if s.json {
		f := map[string]any{"conversation": conversation, "exit": code, "receipt": s.receipt}
		if err != nil {
			f["error"] = err.Error()
		}
		s.event("done", f)
		return code
	}
	if s.wrote {
		fmt.Fprintln(s.out)
	}
	switch {
	case code == runExitInterrupt:
		fmt.Fprintln(s.errOut, "interrupted")
	case err != nil:
		fmt.Fprintln(s.errOut, "error: "+err.Error())
		if strings.Contains(err.Error(), "no such conversation") {
			return code
		}
	}
	fmt.Fprintf(s.errOut, "conversation %s · %s\n", conversation, resumeCommand(conversation))
	return code
}

const (
	receiptChecked    = "✓ checked:"
	receiptUnverified = "⚠ unverified:"
	receiptNoChange   = "⚠ nothing changed"
	receiptFailing    = "⚠ failing after the change:"
)

// receiptHeads are the first words of a turn's receipt line
// (gateway/turn_done.go): checked, unverified, failing, or nothing changed.
var receiptHeads = []string{receiptChecked, receiptUnverified, receiptFailing, receiptNoChange}

// receiptIn is the last receipt line in the text, "" when none yet.
func receiptIn(text string) string {
	i := -1
	for _, h := range receiptHeads {
		if j := strings.LastIndex(text, h); j > i {
			i = j
		}
	}
	if i < 0 {
		return ""
	}
	line := text[i:]
	if k := strings.IndexByte(line, '\n'); k >= 0 {
		line = line[:k]
	}
	return strings.TrimSpace(line)
}

// runExitCode: an error is 1, an unanswered question 3, and the receipt's
// own verdict decides between 0 and 2. "✓ checked" is 0 whatever failed
// on the way there — a receipt lists the turn's history, and a turn that
// failed twice and then passed is a pass (live 2026-10-11: `go build` FAIL,
// `go test` FAIL, then `go mod init && go test` PASS read as exit 2).
// "⚠ unverified" and "⚠ failing after the change" are 2. "⚠ nothing
// changed" ran things without changing anything: its LAST run decides.
func runExitCode(err error, asked bool, receipt string) int {
	switch {
	case errors.Is(err, context.Canceled):
		return runExitInterrupt
	case err != nil:
		return runExitError
	case asked:
		return runExitAsked
	case strings.HasPrefix(receipt, receiptChecked):
		return runExitOK
	case strings.HasPrefix(receipt, receiptUnverified), strings.HasPrefix(receipt, receiptFailing):
		return runExitUnverified
	case strings.HasPrefix(receipt, receiptNoChange) && lastRunFailed(receipt):
		return runExitUnverified
	}
	return runExitOK
}

// lastRunFailed reads the receipt's " · "-separated entries and reports
// whether the last one with a verdict says FAIL.
func lastRunFailed(receipt string) bool {
	last := ""
	for _, part := range strings.Split(receipt, " · ") {
		part = strings.TrimSpace(part)
		if strings.HasSuffix(part, " PASS") || strings.HasSuffix(part, " FAIL") {
			last = part
		}
	}
	return strings.HasSuffix(last, " FAIL")
}

func init() {
	headlessRunCmd.Flags().StringVar(&runDirFlag, "dir", "", "the project directory (default: here)")
	headlessRunCmd.Flags().StringVar(&runConversationFlag, "conversation", "", "continue this conversation instead of starting one")
	headlessRunCmd.Flags().BoolVar(&runJSONFlag, "json", false, "NDJSON events on stdout instead of text")
	headlessRunCmd.Flags().BoolVar(&runYesFlag, "yes", false, "answer any question with its first option (an approval with Yes)")
	rootCmd.AddCommand(headlessRunCmd)
}

// promptFromArgsAndStdin is the pipe contract: arguments are the prompt;
// stdin alone is the prompt; both means stdin is the input the prompt is
// about. Stdin is read only when it is a pipe or a file: a terminal is
// never read, and neither is a socket a harness keeps open — `run "…"`
// hung on one, waiting for an EOF that never came (2026-10-11).
func promptFromArgsAndStdin(args []string, stdin *os.File) (string, error) {
	text := strings.TrimSpace(strings.Join(args, " "))
	piped := ""
	if stdinIsPipeOrFile(stdin) {
		b, _ := io.ReadAll(io.LimitReader(stdin, 1<<20))
		piped = strings.TrimSpace(string(b))
	}
	switch {
	case text == "" && piped == "":
		return "", errors.New("a prompt: memdoor run \"what to do\", or on stdin")
	case text == "":
		return piped, nil
	case piped == "":
		return text, nil
	}
	return text + "\n\nInput:\n" + piped, nil
}

func stdinIsPipeOrFile(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeNamedPipe != 0 || fi.Mode().IsRegular()
}
