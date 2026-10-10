package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// MEMDOOR IN THE EDITOR (Greg, 2026-10-10: "i want to be able to use the
// agents with VS code"). The Agent Client Protocol is what Zed and JetBrains
// host natively and what VS Code hosts through an ACP client extension: a
// JSON-RPC agent over stdio that the editor spawns. `memdoor acp` is that
// agent, as thin as the remote-control page: an editor session is a
// conversation on the local gateway, a prompt is a turn in the editor's
// workspace folder, and the turn's text and tool calls stream back as the
// editor's own message chunks and tool frames. The coder, its tools, the
// decision model and the providers are the gateway's, unchanged.
//
// What drives a turn is behind turnRunner, so the editor half is tested
// against a fake and the gateway half against the gateway.

// turnRunner is what the agent needs of the gateway: a conversation per
// editor session, a turn that streams into a sink until it ends, and an
// interrupt.
type turnRunner interface {
	// NewConversation opens a conversation for a session rooted at cwd.
	NewConversation(ctx context.Context, cwd string) (string, error)
	// Run sends one turn and blocks until it ends, feeding the sink.
	Run(ctx context.Context, conversation, cwd, text string, sink turnSink) error
	// Interrupt stops the running turn of a conversation.
	Interrupt(ctx context.Context, conversation string) error
}

// turnSink is what a turn reports while it runs, and the one thing it may
// ask: a question or an approval, answered by the person in the editor.
type turnSink interface {
	Text(s string)
	ToolStart(id, name string, input json.RawMessage)
	ToolDone(id string, output string, failed bool)
	// Ask puts a question (or, when approval names a tool, an approval) to
	// the person; the answer is one of the options, "" when they declined.
	Ask(question string, options []string, approval string) string
}

type acpSession struct {
	conversation string
	cwd          string
	cancel       context.CancelFunc
}

// acpAgent is the ACP agent over a turnRunner.
type acpAgent struct {
	runner   turnRunner
	conn     *acp.AgentSideConnection
	mu       sync.Mutex
	sessions map[acp.SessionId]*acpSession
}

var _ acp.Agent = (*acpAgent)(nil)

func newACPAgent(r turnRunner) *acpAgent {
	return &acpAgent{runner: r, sessions: map[acp.SessionId]*acpSession{}}
}

func (a *acpAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *acpAgent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{LoadSession: false},
		AgentInfo:         &acp.Implementation{Name: "memdoor", Title: acp.Ptr("Memdoor"), Version: Version},
	}, nil
}

func (a *acpAgent) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *acpAgent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	conv, err := a.runner.NewConversation(ctx, params.Cwd)
	if err != nil {
		return acp.NewSessionResponse{}, err
	}
	id := acp.SessionId("memdoor-" + randomHex(8))
	a.mu.Lock()
	a.sessions[id] = &acpSession{conversation: conv, cwd: params.Cwd}
	a.mu.Unlock()
	return acp.NewSessionResponse{SessionId: id}, nil
}

func (a *acpAgent) Prompt(_ context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	a.mu.Lock()
	s, ok := a.sessions[params.SessionId]
	a.mu.Unlock()
	if !ok {
		return acp.PromptResponse{}, fmt.Errorf("no session %s: session/new first", params.SessionId)
	}
	text := promptText(params.Prompt)
	if strings.TrimSpace(text) == "" {
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
		}
		a.mu.Unlock()
	}()
	sink := &acpSink{agent: a, ctx: ctx, session: params.SessionId}
	err := a.runner.Run(ctx, s.conversation, s.cwd, text, sink)
	if ctx.Err() != nil {
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}
	if err != nil {
		// The turn's failure in the editor, in words, then the turn ends.
		sink.Text("⚠ " + err.Error())
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *acpAgent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	a.mu.Lock()
	s, ok := a.sessions[params.SessionId]
	var cancel context.CancelFunc
	if ok {
		cancel = s.cancel
	}
	a.mu.Unlock()
	if !ok {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	return a.runner.Interrupt(ctx, s.conversation)
}

// The rest of the interface: nothing to load, list, resume or configure yet.

func (a *acpAgent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionLoad)
}
func (a *acpAgent) ListSessions(ctx context.Context, params acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionList)
}
func (a *acpAgent) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionResume)
}
func (a *acpAgent) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	a.mu.Lock()
	delete(a.sessions, params.SessionId)
	a.mu.Unlock()
	return acp.CloseSessionResponse{}, nil
}
func (a *acpAgent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}
func (a *acpAgent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}
func (a *acpAgent) Logout(ctx context.Context, params acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, acp.NewMethodNotFound(acp.AgentMethodLogout)
}
func (a *acpAgent) UnstableDidChangeDocument(context.Context, acp.UnstableDidChangeDocumentNotification) error {
	return nil
}
func (a *acpAgent) UnstableDidCloseDocument(context.Context, acp.UnstableDidCloseDocumentNotification) error {
	return nil
}
func (a *acpAgent) UnstableDidFocusDocument(context.Context, acp.UnstableDidFocusDocumentNotification) error {
	return nil
}
func (a *acpAgent) UnstableDidOpenDocument(context.Context, acp.UnstableDidOpenDocumentNotification) error {
	return nil
}
func (a *acpAgent) UnstableDidSaveDocument(context.Context, acp.UnstableDidSaveDocumentNotification) error {
	return nil
}
func (a *acpAgent) UnstableAcceptNes(context.Context, acp.UnstableAcceptNesNotification) error {
	return nil
}
func (a *acpAgent) UnstableCloseNes(context.Context, acp.UnstableCloseNesRequest) (acp.UnstableCloseNesResponse, error) {
	return acp.UnstableCloseNesResponse{}, acp.NewMethodNotFound(acp.AgentMethodNesClose)
}
func (a *acpAgent) UnstableRejectNes(context.Context, acp.UnstableRejectNesNotification) error {
	return nil
}
func (a *acpAgent) UnstableStartNes(context.Context, acp.UnstableStartNesRequest) (acp.UnstableStartNesResponse, error) {
	return acp.UnstableStartNesResponse{}, acp.NewMethodNotFound(acp.AgentMethodNesStart)
}
func (a *acpAgent) UnstableSuggestNes(context.Context, acp.UnstableSuggestNesRequest) (acp.UnstableSuggestNesResponse, error) {
	return acp.UnstableSuggestNesResponse{}, acp.NewMethodNotFound(acp.AgentMethodNesSuggest)
}
func (a *acpAgent) UnstableDisableProvider(context.Context, acp.UnstableDisableProviderRequest) (acp.UnstableDisableProviderResponse, error) {
	return acp.UnstableDisableProviderResponse{}, acp.NewMethodNotFound(acp.AgentMethodProvidersDisable)
}

// acpSink turns a turn's events into the session's updates.
type acpSink struct {
	agent   *acpAgent
	ctx     context.Context
	session acp.SessionId
}

func (s *acpSink) send(u acp.SessionUpdate) {
	if s.agent.conn == nil {
		return
	}
	_ = s.agent.conn.SessionUpdate(s.ctx, acp.SessionNotification{SessionId: s.session, Update: u})
}

func (s *acpSink) Text(t string) {
	if t != "" {
		s.send(acp.UpdateAgentMessageText(t))
	}
}

func (s *acpSink) ToolStart(id, name string, input json.RawMessage) {
	var raw any
	_ = json.Unmarshal(input, &raw)
	opts := []acp.ToolCallStartOpt{acp.WithStartKind(toolKindOf(name)), acp.WithStartStatus(acp.ToolCallStatusInProgress), acp.WithStartRawInput(raw)}
	if p := toolPath(input); p != "" {
		opts = append(opts, acp.WithStartLocations([]acp.ToolCallLocation{{Path: p}}))
	}
	s.send(acp.StartToolCall(acp.ToolCallId(id), toolTitle(name, input), opts...))
}

func (s *acpSink) ToolDone(id string, output string, failed bool) {
	status := acp.ToolCallStatusCompleted
	if failed {
		status = acp.ToolCallStatusFailed
	}
	opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(status)}
	if output != "" {
		opts = append(opts, acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock(clipTail(output, 4000)))}))
	}
	s.send(acp.UpdateToolCall(acp.ToolCallId(id), opts...))
}

// Ask is the gateway's question stream as the editor's permission request:
// an approval (MEMDOOR_APPROVE=changes) or an ask_user_question picker, each
// option a choice; the editor's cancel is "".
func (s *acpSink) Ask(question string, options []string, approval string) string {
	if s.agent.conn == nil || len(options) == 0 {
		return ""
	}
	kind := acp.ToolKindOther
	if approval != "" {
		kind = toolKindOf(approval)
	}
	var opts []acp.PermissionOption
	for i, o := range options {
		k := acp.PermissionOptionKindAllowOnce
		lower := strings.ToLower(o)
		switch {
		case approval != "" && strings.HasPrefix(lower, "yes, and"):
			k = acp.PermissionOptionKindAllowAlways
		case approval != "" && lower == "no":
			k = acp.PermissionOptionKindRejectOnce
		}
		opts = append(opts, acp.PermissionOption{Kind: k, Name: o, OptionId: acp.PermissionOptionId(fmt.Sprint(i))})
	}
	title := question
	resp, err := s.agent.conn.RequestPermission(s.ctx, acp.RequestPermissionRequest{
		SessionId: s.session,
		ToolCall:  acp.ToolCallUpdate{ToolCallId: acp.ToolCallId("ask-" + randomHex(4)), Title: acp.Ptr(title), Kind: acp.Ptr(kind), Status: acp.Ptr(acp.ToolCallStatusPending)},
		Options:   opts,
	})
	if err != nil || resp.Outcome.Selected == nil {
		return ""
	}
	var i int
	if _, err := fmt.Sscanf(string(resp.Outcome.Selected.OptionId), "%d", &i); err != nil || i < 0 || i >= len(options) {
		return ""
	}
	return options[i]
}

// toolKindOf is the editor's category for a Memdoor tool.
func toolKindOf(name string) acp.ToolKind {
	switch name {
	case "read_file", "jread", "jgrep", "grep", "glob", "locate", "list_files", "ls", "jlogs":
		return acp.ToolKindRead
	case "write_file", "edit_file", "apply_patch", "search_replace":
		return acp.ToolKindEdit
	case "bash", "verify":
		return acp.ToolKindExecute
	case "web_search", "web_fetch":
		return acp.ToolKindFetch
	case "ask_user_question":
		return acp.ToolKindOther
	}
	return acp.ToolKindOther
}

// toolTitle is the frame's title: the tool and the one argument that names
// what it touches; a patch is named by the first file it touches.
func toolTitle(name string, input json.RawMessage) string {
	if p := toolPath(input); p != "" {
		return name + ": " + firstLineOf(p, 80)
	}
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, k := range []string{"command", "pattern", "query", "question"} {
		if v, ok := in[k].(string); ok && v != "" {
			return name + ": " + firstLineOf(v, 80)
		}
	}
	return name
}

var patchFileRe = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+)$`)

// toolPath is the file a tool touches: its path argument, or the first file
// of an apply_patch (the editor shows the frame at that location).
func toolPath(input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, k := range []string{"path", "file_path"} {
		if v, ok := in[k].(string); ok && v != "" {
			return v
		}
	}
	for _, k := range []string{"patch", "input"} {
		if v, ok := in[k].(string); ok {
			if m := patchFileRe.FindStringSubmatch(v); m != nil {
				return strings.TrimSpace(m[1])
			}
		}
	}
	return ""
}

// promptText joins the prompt's text blocks; an editor may send several.
func promptText(blocks []acp.ContentBlock) string {
	var b strings.Builder
	for _, c := range blocks {
		if c.Text != nil {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text.Text)
		}
	}
	return b.String()
}

func firstLineOf(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + "…"
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func clipTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
