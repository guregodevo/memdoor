package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

// Logger is what this package logs through: *slog.Logger satisfies it, and
// so does the gateway's event logger.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

// Client is a connection to one MCP server: the handshake, its tools, and
// calls to them. NewStdio runs a local command and talks over its stdin and
// stdout; NewHTTP talks to a URL (Streamable HTTP). New picks by the spec.
type Client interface {
	// Start connects and runs the initialize handshake.
	Start(ctx context.Context) error
	// ServerInfo is what the server said about itself in the handshake.
	ServerInfo() ServerInfo
	// ListTools returns every tool the server offers, all pages.
	ListTools(ctx context.Context) ([]Tool, error)
	// CallTool runs one tool.
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
	// ListResources returns every resource the server lists, all pages.
	ListResources(ctx context.Context) ([]ResourceInfo, error)
	// ReadResource returns a resource's contents.
	ReadResource(ctx context.Context, uri string) ([]Resource, error)
	// ListPrompts returns every prompt the server offers, all pages.
	ListPrompts(ctx context.Context) ([]Prompt, error)
	// GetPrompt fills a prompt with arguments and returns its messages.
	GetPrompt(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error)
	// Stop ends the connection (and the process, for a local server).
	Stop() error
}

// ProtocolVersion is the MCP revision this client speaks first; the server
// answers with the one it will use.
const ProtocolVersion = "2025-06-18"

// DefaultRequestTimeout bounds a request whose context has no deadline.
const DefaultRequestTimeout = 30 * time.Second

// Spec is how to reach a server: a command (stdio) or a URL (HTTP).
type Spec struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string // added to the inherited environment
	Dir     string            // working directory of the command
	URL     string
	SSE     bool // the old HTTP+SSE transport (2024-11-05)
	Headers map[string]string
	// Token, when set, is asked for a bearer token before every HTTP
	// request (OAuth); it overrides an Authorization header.
	Token func(ctx context.Context) (string, error)
}

// ServerInfo is the server's own description from the handshake.
type ServerInfo struct {
	Name            string
	Version         string
	ProtocolVersion string
	Instructions    string
	// What the server offers besides tools.
	Resources bool
	Prompts   bool
}

// Tool is one tool a server offers.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Content           []Content       `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// Content represents MCP content (text, image, resource)
type Content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Data     string    `json:"data,omitempty"`     // Base64 for images
	MimeType string    `json:"mimeType,omitempty"` // For images
	Resource *Resource `json:"resource,omitempty"`
}

// ResourceInfo is a resource as a server lists it.
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Prompt is a prompt template a server offers.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument is one argument a prompt takes.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage is one message of a filled prompt.
type PromptMessage struct {
	Role    string  `json:"role"`
	Content Content `json:"content"`
}

// Resource is an embedded resource in a tool result, or a resource's
// contents.
type Resource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64
}

// RPCError is a JSON-RPC error the server answered with.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message) }

// AuthRequiredError is an HTTP server refusing the request for want of
// credentials (401, or 403 with a challenge). WWWAuthenticate is the
// server's challenge, which names where its OAuth metadata lives.
type AuthRequiredError struct {
	Status          int
	WWWAuthenticate string
}

func (e *AuthRequiredError) Error() string {
	return fmt.Sprintf("the server needs sign-in (HTTP %d)", e.Status)
}

// IsAuthRequired reports whether err is a server asking for credentials.
func IsAuthRequired(err error) bool {
	var a *AuthRequiredError
	return errors.As(err, &a)
}

// New is the client for spec: HTTP when it has a URL, stdio when it has a
// command.
func New(spec Spec, logger Logger) (Client, error) {
	switch {
	case spec.URL != "" && spec.Command != "":
		return nil, fmt.Errorf("server %q has both a command and a URL", spec.Name)
	case spec.URL != "" && spec.SSE:
		return NewSSE(spec, logger), nil
	case spec.URL != "":
		return NewHTTP(spec, logger), nil
	case spec.Command != "":
		return NewStdio(spec, logger), nil
	}
	return nil, fmt.Errorf("server %q has neither a command nor a URL", spec.Name)
}

// NewClient is a stdio client for command and args (the browser tool's
// constructor).
func NewClient(command string, args []string, logger Logger) Client {
	return NewStdio(Spec{Name: command, Command: command, Args: args}, logger)
}

// transport carries JSON-RPC messages to one server. call sends a request
// and returns its result; notify sends a notification.
type transport interface {
	open(ctx context.Context) error
	call(ctx context.Context, id int64, method string, params interface{}) (json.RawMessage, error)
	notify(ctx context.Context, method string, params interface{}) error
	negotiated(protocolVersion string)
	close() error
}

// client is the protocol over a transport.
type client struct {
	t      transport
	name   string
	logger Logger
	nextID atomic.Int64
	info   ServerInfo
}

func newClient(t transport, name string, logger Logger) *client {
	if logger == nil {
		logger = slog.Default()
	}
	return &client{t: t, name: name, logger: logger}
}

func (c *client) request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultRequestTimeout)
		defer cancel()
	}
	return c.t.call(ctx, c.nextID.Add(1), method, params)
}

func (c *client) Start(ctx context.Context) error {
	raw, err := c.initialize(ctx)
	// The spec's rule for servers from before Streamable HTTP: a POST they
	// refuse with 404 or 405 means the old HTTP+SSE transport.
	var he *HTTPError
	if ht, isHTTP := c.t.(*httpTransport); isHTTP && errors.As(err, &he) && (he.Status == 404 || he.Status == 405) {
		c.logger.Info("MCP server speaks the old SSE transport", "server", c.name)
		c.t = &sseTransport{spec: ht.spec, logger: c.logger, pending: newPending(), http: ht.http}
		raw, err = c.initialize(ctx)
	}
	if err != nil {
		return err
	}
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Instructions string `json:"instructions"`
		Capabilities struct {
			Resources json.RawMessage `json:"resources"`
			Prompts   json.RawMessage `json:"prompts"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		_ = c.t.close()
		return fmt.Errorf("it answered the handshake with something unreadable: %w", err)
	}
	c.info = ServerInfo{Name: res.ServerInfo.Name, Version: res.ServerInfo.Version,
		ProtocolVersion: res.ProtocolVersion, Instructions: res.Instructions,
		Resources: len(res.Capabilities.Resources) > 0, Prompts: len(res.Capabilities.Prompts) > 0}
	c.t.negotiated(res.ProtocolVersion)
	if err := c.t.notify(ctx, "notifications/initialized", nil); err != nil {
		_ = c.t.close()
		return err
	}
	c.logger.Info("MCP server connected", slog.String("server", c.name),
		slog.String("server_name", c.info.Name), slog.String("protocol", c.info.ProtocolVersion))
	return nil
}

// initialize opens the transport and sends the handshake's request.
func (c *client) initialize(ctx context.Context) (json.RawMessage, error) {
	if err := c.t.open(ctx); err != nil {
		return nil, err
	}
	raw, err := c.request(ctx, "initialize", map[string]interface{}{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "memdoor", "version": "1"},
	})
	if err != nil {
		_ = c.t.close()
		// A command that runs but speaks something else refuses the handshake
		// (`cat` echoes it back as "method not found: initialize"). Say that in
		// words first: the person pasted the wrong thing, not a broken server.
		// No name in these: every surface that shows an error names the
		// server itself, and two names read as a stutter ("everything:
		// everything: the server exited").
		var rpc *RPCError
		if errors.As(err, &rpc) {
			return nil, fmt.Errorf("it did not answer the MCP handshake — is it an MCP server? (%s)", rpc.Message)
		}
		return nil, err
	}
	return raw, nil
}

func (c *client) ServerInfo() ServerInfo { return c.info }

func (c *client) ListTools(ctx context.Context) ([]Tool, error) {
	var out []Tool
	err := c.paged(ctx, "tools/list", "tools", func(r json.RawMessage) error {
		var page []Tool
		if len(r) == 0 {
			return nil
		}
		err := json.Unmarshal(r, &page)
		out = append(out, page...)
		return err
	})
	return out, err
}

func (c *client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error) {
	if arguments == nil {
		arguments = map[string]interface{}{}
	}
	raw, err := c.request(ctx, "tools/call", map[string]interface{}{"name": name, "arguments": arguments})
	if err != nil {
		return nil, err
	}
	var res ToolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("tools/call %s: unreadable answer: %w", name, err)
	}
	return &res, nil
}

// paged runs a list method through every page, decoding each page's items
// from field into out.
func (c *client) paged(ctx context.Context, method, field string, each func(json.RawMessage) error) error {
	cursor := ""
	for page := 0; page < 100; page++ {
		var params interface{}
		if cursor != "" {
			params = map[string]string{"cursor": cursor}
		}
		raw, err := c.request(ctx, method, params)
		if err != nil {
			return err
		}
		var res map[string]json.RawMessage
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("%s %s: unreadable answer: %w", method, c.name, err)
		}
		if err := each(res[field]); err != nil {
			return fmt.Errorf("%s %s: %w", method, c.name, err)
		}
		var next string
		_ = json.Unmarshal(res["nextCursor"], &next)
		if next == "" {
			return nil
		}
		cursor = next
	}
	return nil
}

func (c *client) ListResources(ctx context.Context) ([]ResourceInfo, error) {
	var out []ResourceInfo
	err := c.paged(ctx, "resources/list", "resources", func(r json.RawMessage) error {
		var page []ResourceInfo
		if len(r) == 0 {
			return nil
		}
		err := json.Unmarshal(r, &page)
		out = append(out, page...)
		return err
	})
	return out, err
}

func (c *client) ReadResource(ctx context.Context, uri string) ([]Resource, error) {
	raw, err := c.request(ctx, "resources/read", map[string]string{"uri": uri})
	if err != nil {
		return nil, err
	}
	var res struct {
		Contents []Resource `json:"contents"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("resources/read %s: unreadable answer: %w", uri, err)
	}
	return res.Contents, nil
}

func (c *client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var out []Prompt
	err := c.paged(ctx, "prompts/list", "prompts", func(r json.RawMessage) error {
		var page []Prompt
		if len(r) == 0 {
			return nil
		}
		err := json.Unmarshal(r, &page)
		out = append(out, page...)
		return err
	})
	return out, err
}

func (c *client) GetPrompt(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error) {
	if arguments == nil {
		arguments = map[string]string{}
	}
	raw, err := c.request(ctx, "prompts/get", map[string]interface{}{"name": name, "arguments": arguments})
	if err != nil {
		return nil, err
	}
	var res struct {
		Messages []PromptMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("prompts/get %s: unreadable answer: %w", name, err)
	}
	return res.Messages, nil
}

func (c *client) Stop() error { return c.t.close() }

// message is one JSON-RPC message, either direction.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func encodeRequest(id int64, method string, params interface{}) ([]byte, error) {
	m := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		m["params"] = params
	}
	return json.Marshal(m)
}

func encodeNotification(method string, params interface{}) ([]byte, error) {
	m := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if params != nil {
		m["params"] = params
	}
	return json.Marshal(m)
}

// replyTo answers a request the server sent: ping succeeds, anything else
// is a method this client does not offer.
func replyTo(m message) []byte {
	var out map[string]interface{}
	if m.Method == "ping" {
		out = map[string]interface{}{"jsonrpc": "2.0", "id": m.ID, "result": map[string]interface{}{}}
	} else {
		out = map[string]interface{}{"jsonrpc": "2.0", "id": m.ID,
			"error": map[string]interface{}{"code": -32601, "message": "method not found: " + m.Method}}
	}
	b, _ := json.Marshal(out)
	return b
}

// idOf reads a response id as a number (this client only sends numbers).
func idOf(raw json.RawMessage) (int64, bool) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var v int64
		if _, err := fmt.Sscan(strings.TrimSpace(s), &v); err == nil {
			return v, true
		}
	}
	return 0, false
}
