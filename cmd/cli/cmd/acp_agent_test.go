package cmd

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

// fakeRunner is a turn that streams a sentence, runs one tool, asks one
// approval, and ends; or blocks until interrupted.
type fakeRunner struct {
	mu          sync.Mutex
	cwd         string
	prompts     []string
	interrupted int
	block       bool
	answers     []string
}

func (f *fakeRunner) NewConversation(ctx context.Context, cwd string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cwd = cwd
	return "conv-1", nil
}

func (f *fakeRunner) Run(ctx context.Context, conversation, cwd, text string, sink turnSink) error {
	f.mu.Lock()
	f.prompts = append(f.prompts, text)
	block := f.block
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	sink.Text("On it. ")
	sink.ToolStart("tu-1", "bash", json.RawMessage(`{"command":"go test ./..."}`))
	sink.ToolDone("tu-1", "ok  \tcalc\t0.1s", false)
	a := sink.Ask("bash: rm -rf build — allow?", []string{"Yes", "Yes, and don't ask again for this tool this session", "No"}, "bash")
	f.mu.Lock()
	f.answers = append(f.answers, a)
	f.mu.Unlock()
	sink.Text("Done.")
	return nil
}

func (f *fakeRunner) Interrupt(ctx context.Context, conversation string) error {
	f.mu.Lock()
	f.interrupted++
	f.mu.Unlock()
	return nil
}

// fakeEditor is the client side: it keeps every update and approves with
// the second option.
type fakeEditor struct {
	mu      sync.Mutex
	updates []acp.SessionNotification
	asked   []acp.RequestPermissionRequest
}

func (e *fakeEditor) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
	e.mu.Lock()
	e.updates = append(e.updates, n)
	e.mu.Unlock()
	return nil
}
func (e *fakeEditor) RequestPermission(ctx context.Context, r acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	e.mu.Lock()
	e.asked = append(e.asked, r)
	e.mu.Unlock()
	return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: r.Options[1].OptionId}}}, nil
}
func (e *fakeEditor) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}
func (e *fakeEditor) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}
func (e *fakeEditor) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}
func (e *fakeEditor) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}
func (e *fakeEditor) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}
func (e *fakeEditor) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}
func (e *fakeEditor) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}

// wire connects an agent and an editor in-process over two pipes.
func wire(t *testing.T, runner turnRunner) (*acp.ClientSideConnection, *fakeEditor) {
	t.Helper()
	agentIn, editorOut := io.Pipe()
	editorIn, agentOut := io.Pipe()
	agent := newACPAgent(runner)
	asc := acp.NewAgentSideConnection(agent, agentOut, agentIn)
	agent.SetAgentConnection(asc)
	editor := &fakeEditor{}
	csc := acp.NewClientSideConnection(editor, editorOut, editorIn)
	t.Cleanup(func() { _ = agentOut.Close(); _ = editorOut.Close() })
	return csc, editor
}

// An editor session is a conversation in its folder; a prompt streams the
// turn's text and tool frames with the editor's kinds and ends as end_turn;
// an approval reaches the editor as a permission request and its choice
// goes back as the option's text.
func TestAnEditorPromptStreamsTheTurnAsItsOwnFrames(t *testing.T) {
	runner := &fakeRunner{}
	csc, editor := wire(t, runner)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	init, err := csc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil || init.ProtocolVersion != acp.ProtocolVersionNumber || init.AgentInfo == nil || init.AgentInfo.Name != "memdoor" {
		t.Fatalf("initialize: %+v %v", init, err)
	}
	sess, err := csc.NewSession(ctx, acp.NewSessionRequest{Cwd: "/tmp/proj", McpServers: []acp.McpServer{}})
	if err != nil || sess.SessionId == "" || runner.cwd != "/tmp/proj" {
		t.Fatalf("session/new: %+v %v cwd=%q", sess, err, runner.cwd)
	}
	resp, err := csc.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("add a Sub function")}})
	if err != nil || resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("prompt: %+v %v", resp, err)
	}
	if len(runner.prompts) != 1 || runner.prompts[0] != "add a Sub function" {
		t.Fatalf("the prompt's text reaches the turn: %v", runner.prompts)
	}
	editor.mu.Lock()
	defer editor.mu.Unlock()
	var texts []string
	var started, done *acp.SessionUpdate
	for i := range editor.updates {
		u := editor.updates[i].Update
		switch {
		case u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil:
			texts = append(texts, u.AgentMessageChunk.Content.Text.Text)
		case u.ToolCall != nil:
			started = &editor.updates[i].Update
		case u.ToolCallUpdate != nil:
			done = &editor.updates[i].Update
		}
	}
	if strings.Join(texts, "") != "On it. Done." {
		t.Fatalf("the turn's text, in order: %q", texts)
	}
	if started == nil || started.ToolCall.Kind != acp.ToolKindExecute || string(started.ToolCall.ToolCallId) != "tu-1" || !strings.Contains(started.ToolCall.Title, "go test") {
		t.Fatalf("a bash call is an execute frame titled by its command: %+v", started)
	}
	if done == nil || done.ToolCallUpdate.Status == nil || *done.ToolCallUpdate.Status != acp.ToolCallStatusCompleted || string(done.ToolCallUpdate.ToolCallId) != "tu-1" {
		t.Fatalf("the call completes under the same id: %+v", done)
	}
	if len(editor.asked) != 1 || len(editor.asked[0].Options) != 3 || editor.asked[0].Options[1].Kind != acp.PermissionOptionKindAllowAlways || editor.asked[0].Options[2].Kind != acp.PermissionOptionKindRejectOnce {
		t.Fatalf("an approval is a permission request with the gateway's three options: %+v", editor.asked)
	}
	if len(runner.answers) != 1 || !strings.HasPrefix(runner.answers[0], "Yes, and") {
		t.Fatalf("the editor's choice goes back as the option's exact text: %v", runner.answers)
	}
}

// Cancel from the editor interrupts the turn and the prompt ends as cancelled.
func TestAnEditorCancelInterruptsTheTurn(t *testing.T) {
	runner := &fakeRunner{block: true}
	csc, _ := wire(t, runner)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := csc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	sess, err := csc.NewSession(ctx, acp.NewSessionRequest{Cwd: "/tmp/proj", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatal(err)
	}
	type out struct {
		resp acp.PromptResponse
		err  error
	}
	res := make(chan out, 1)
	go func() {
		r, e := csc.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("wait")}})
		res <- out{r, e}
	}()
	for i := 0; i < 50; i++ {
		runner.mu.Lock()
		n := len(runner.prompts)
		runner.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := csc.Cancel(ctx, acp.CancelNotification{SessionId: sess.SessionId}); err != nil {
		t.Fatal(err)
	}
	r := <-res
	if r.err != nil || r.resp.StopReason != acp.StopReasonCancelled {
		t.Fatalf("a cancelled prompt ends as cancelled: %+v %v", r.resp, r.err)
	}
	if runner.interrupted != 1 {
		t.Fatalf("the turn was interrupted on the gateway: %d", runner.interrupted)
	}
}

// A patch frame is named and placed by the first file it touches.
func TestAPatchFrameNamesItsFile(t *testing.T) {
	in := json.RawMessage(`{"patch":"*** Begin Patch\n*** Update File: calc.go\n@@\n+func Sub() {}\n*** End Patch"}`)
	if got := toolTitle("apply_patch", in); got != "apply_patch: calc.go" {
		t.Fatalf("title: %q", got)
	}
	if got := toolPath(in); got != "calc.go" {
		t.Fatalf("path: %q", got)
	}
	if got := toolTitle("bash", json.RawMessage(`{"command":"go test ./...\necho done"}`)); got != "bash: go test ./...…" {
		t.Fatalf("a command's first line: %q", got)
	}
	if toolKindOf("apply_patch") != acp.ToolKindEdit || toolKindOf("jread") != acp.ToolKindRead || toolKindOf("bash") != acp.ToolKindExecute {
		t.Fatal("tool kinds")
	}
}
