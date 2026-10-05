package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager keeps the connections to the servers a project uses: started on
// first need, kept while used, closed when idle, retried after a failure.
type Manager interface {
	// Tools connects the project's enabled, trusted servers (in parallel,
	// waiting at most the connect window) and returns the tools of those
	// that are up. A server still connecting joins on a later turn.
	Tools(ctx context.Context, projectDir string) []ServerTools
	// Call runs a tool on a connected server.
	Call(ctx context.Context, projectDir, server, tool string, args map[string]interface{}) (*ToolResult, error)
	// Resources lists the resources of the project's connected servers
	// that offer them (server "" for all of them).
	Resources(ctx context.Context, projectDir, server string) ([]ServerResource, error)
	// ReadResource reads one resource from a connected server.
	ReadResource(ctx context.Context, projectDir, server, uri string) ([]Resource, error)
	// Prompts lists the prompts of the project's connected servers.
	Prompts(ctx context.Context, projectDir string) []ServerPrompt
	// GetPrompt fills one prompt on a connected server.
	GetPrompt(ctx context.Context, projectDir, server, name string, args map[string]string) ([]PromptMessage, error)
	// Status is every configured server with its state, for the /mcp panel.
	Status(ctx context.Context, projectDir string) ([]Status, error)
	// Test connects one server now, whatever its last failure, and returns
	// its state (the add flow and /mcp test).
	Test(ctx context.Context, projectDir, name string) Status
	// Forget drops a server's connection so the next use reconnects (after a
	// config change, a sign-in or a sign-out).
	Forget(projectDir, name string)
	// Shutdown closes every connection.
	Shutdown()
}

// ServerTools is one connected server's tools, and what else it offers.
type ServerTools struct {
	Server string
	Tools  []Tool
	Info   ServerInfo
}

// ServerResource is a resource and the server it comes from.
type ServerResource struct {
	Server string
	ResourceInfo
}

// ServerPrompt is a prompt and the server it comes from.
type ServerPrompt struct {
	Server string
	Prompt
}

// State is where a server stands.
type State string

const (
	StateConnected   State = "connected"
	StateConnecting  State = "connecting"
	StateFailed      State = "failed"
	StateNeedsSignIn State = "needs-sign-in"
	StateOff         State = "off"
	StateNotTrusted  State = "not-trusted"
	StateIdle        State = "idle" // configured, not started yet
)

// Status is one server for the panel.
type Status struct {
	Name      string
	Scope     Scope
	Transport string // "stdio" or "http"
	Target    string // the command line or the URL
	State     State
	Err       string
	Tools     []Tool
	Info      ServerInfo
	// Saved says the server is in a file. An add that refused to keep one
	// reports false, so the caller says so instead of showing a dead row.
	Saved bool
}

// Options are the manager's dependencies and bounds.
type Options struct {
	Store  Store
	Logger Logger
	// Env reads the environment for ${VAR} expansion (os.LookupEnv).
	Env func(string) (string, bool)
	// TokenFor, when set, gives an HTTP server its bearer token (OAuth).
	TokenFor func(s Server, sp Spec) func(ctx context.Context) (string, error)
	// ConnectWindow is how long Tools waits for servers still connecting.
	ConnectWindow time.Duration
	// ConnectTimeout bounds one server's connect and tools/list.
	ConnectTimeout time.Duration
	// RetryAfter is how long a failed server is left alone.
	RetryAfter time.Duration
	// IdleAfter closes a connection unused this long.
	IdleAfter time.Duration
}

// NewManager is the Manager over opts.Store.
func NewManager(opts Options) Manager {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Env == nil {
		opts.Env = os.LookupEnv
	}
	if opts.ConnectWindow == 0 {
		opts.ConnectWindow = 8 * time.Second
	}
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = 60 * time.Second // npx's first download is slow
	}
	if opts.RetryAfter == 0 {
		opts.RetryAfter = time.Minute
	}
	if opts.IdleAfter == 0 {
		opts.IdleAfter = 30 * time.Minute
	}
	m := &manager{opts: opts, conns: map[string]*conn{}, stop: make(chan struct{})}
	go m.reapIdle()
	return m
}

type manager struct {
	opts  Options
	mu    sync.Mutex
	conns map[string]*conn // projectDir \x00 name
	stop  chan struct{}
	once  sync.Once
}

// conn is one server's connection for one project.
type conn struct {
	mu       sync.Mutex
	key      string // the spec it was made from: a changed config reconnects
	client   Client
	tools    []Tool
	state    State
	err      string
	failedAt time.Time
	lastUsed time.Time
	ready    chan struct{} // closed when the current attempt ends
}

func connKey(projectDir, name string) string { return projectDir + "\x00" + name }

func specKey(sp Spec) string {
	b, _ := json.Marshal(struct {
		C   string
		A   []string
		E   map[string]string
		D   string
		U   string
		SSE bool
		H   map[string]string
		Tok bool
	}{sp.Command, sp.Args, sp.Env, sp.Dir, sp.URL, sp.SSE, sp.Headers, sp.Token != nil})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// plan is a server and what to do about it this turn.
type plan struct {
	srv   Server
	spec  Spec
	state State // set when it will not be started
	err   string
}

func (m *manager) plans(projectDir string) ([]plan, error) {
	servers, err := m.opts.Store.Servers(projectDir)
	if err != nil {
		return nil, err
	}
	trusted, terr := m.opts.Store.Trusted(projectDir)
	out := make([]plan, 0, len(servers))
	for _, s := range servers {
		p := plan{srv: s}
		sp, serr := s.Spec(projectDir, m.opts.Env)
		p.spec = sp
		switch {
		case s.Entry.Disabled:
			p.state = StateOff
		case s.Scope == ScopeProject && (terr != nil || !trusted):
			p.state = StateNotTrusted
		case serr != nil:
			p.state, p.err = StateFailed, serr.Error()
		}
		if p.state == "" && m.opts.TokenFor != nil && sp.URL != "" {
			p.spec.Token = m.opts.TokenFor(s, sp)
		}
		out = append(out, p)
	}
	return out, nil
}

// ensure starts a connection attempt for p unless one is up, running, or
// failed recently; it returns the conn.
func (m *manager) ensure(projectDir string, p plan, force bool) *conn {
	key := connKey(projectDir, p.srv.Name)
	sk := specKey(p.spec)
	m.mu.Lock()
	c := m.conns[key]
	if c == nil || c.key != sk {
		if c != nil {
			go c.close()
		}
		c = &conn{key: sk, state: StateIdle}
		m.conns[key] = c
	}
	m.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.state == StateConnected || c.state == StateConnecting:
		return c
	case (c.state == StateFailed || c.state == StateNeedsSignIn) && !force && time.Since(c.failedAt) < m.opts.RetryAfter:
		return c
	}
	c.state, c.err = StateConnecting, ""
	c.ready = make(chan struct{})
	go m.connect(c, p)
	return c
}

func (m *manager) connect(c *conn, p plan) {
	ctx, cancel := context.WithTimeout(context.Background(), m.opts.ConnectTimeout)
	defer cancel()
	cl, err := New(p.spec, m.opts.Logger)
	var tools []Tool
	if err == nil {
		err = cl.Start(ctx)
	}
	if err == nil {
		tools, err = cl.ListTools(ctx)
		if err != nil {
			_ = cl.Stop()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	defer close(c.ready)
	if err != nil {
		c.state, c.err, c.failedAt = StateFailed, err.Error(), time.Now()
		if IsAuthRequired(err) {
			c.state, c.err = StateNeedsSignIn, "sign in with /mcp login "+p.srv.Name
		}
		m.opts.Logger.Warn("MCP server not connected", slog.String("server", p.srv.Name), slog.String("error", c.err))
		return
	}
	c.client, c.tools, c.state, c.lastUsed = cl, tools, StateConnected, time.Now()
	m.opts.Logger.Info(fmt.Sprintf("MCP server %s ready: %d tools", p.srv.Name, len(tools)), slog.String("server", p.srv.Name))
}

func (c *conn) close() {
	c.mu.Lock()
	cl := c.client
	c.client, c.tools, c.state = nil, nil, StateIdle
	c.mu.Unlock()
	if cl != nil {
		_ = cl.Stop()
	}
}

func (m *manager) Tools(ctx context.Context, projectDir string) []ServerTools {
	plans, err := m.plans(projectDir)
	if err != nil {
		m.opts.Logger.Warn("MCP config unreadable", slog.String("error", err.Error()))
		return nil
	}
	type started struct {
		name string
		c    *conn
	}
	var all []started
	for _, p := range plans {
		if p.state != "" {
			continue
		}
		all = append(all, started{p.srv.Name, m.ensure(projectDir, p, false)})
	}
	deadline := time.NewTimer(m.opts.ConnectWindow)
	defer deadline.Stop()
	var out []ServerTools
	for _, s := range all {
		s.c.mu.Lock()
		ready := s.c.ready
		state := s.c.state
		s.c.mu.Unlock()
		if state == StateConnecting && ready != nil {
			select {
			case <-ready:
			case <-deadline.C:
				deadline.Reset(0)
			case <-ctx.Done():
				return out
			}
		}
		s.c.mu.Lock()
		if s.c.state == StateConnected {
			out = append(out, ServerTools{Server: s.name, Tools: s.c.tools, Info: s.c.client.ServerInfo()})
			s.c.lastUsed = time.Now()
		}
		s.c.mu.Unlock()
	}
	return out
}

func (m *manager) Call(ctx context.Context, projectDir, server, tool string, args map[string]interface{}) (*ToolResult, error) {
	m.mu.Lock()
	c := m.conns[connKey(projectDir, server)]
	m.mu.Unlock()
	if c == nil {
		return nil, fmt.Errorf("MCP server %q is not connected", server)
	}
	c.mu.Lock()
	cl, state, why := c.client, c.state, c.err
	c.lastUsed = time.Now()
	c.mu.Unlock()
	if state != StateConnected || cl == nil {
		return nil, fmt.Errorf("MCP server %q is %s: %s", server, state, why)
	}
	res, err := cl.CallTool(ctx, tool, args)
	if err != nil {
		var rpc *RPCError
		if !errors.As(err, &rpc) { // the connection itself failed: reconnect next turn
			c.mu.Lock()
			c.state, c.err, c.failedAt = StateFailed, err.Error(), time.Time{}
			c.mu.Unlock()
			go func() { _ = cl.Stop() }()
		}
	}
	return res, err
}

// connected is the client of a connected server, or why there is none.
func (m *manager) connected(projectDir, server string) (Client, error) {
	m.mu.Lock()
	c := m.conns[connKey(projectDir, server)]
	m.mu.Unlock()
	if c == nil {
		return nil, fmt.Errorf("MCP server %q is not connected", server)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != StateConnected || c.client == nil {
		return nil, fmt.Errorf("MCP server %q is %s: %s", server, c.state, c.err)
	}
	c.lastUsed = time.Now()
	return c.client, nil
}

// connectedClients is every connected server of the project, by name.
func (m *manager) connectedClients(projectDir string) map[string]Client {
	prefix := projectDir + "\x00"
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]Client{}
	for k, c := range m.conns {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		c.mu.Lock()
		if c.state == StateConnected && c.client != nil {
			out[strings.TrimPrefix(k, prefix)] = c.client
		}
		c.mu.Unlock()
	}
	return out
}

func (m *manager) Resources(ctx context.Context, projectDir, server string) ([]ServerResource, error) {
	var out []ServerResource
	var errs []string
	for name, cl := range m.connectedClients(projectDir) {
		if (server != "" && name != server) || !cl.ServerInfo().Resources {
			continue
		}
		list, err := cl.ListResources(ctx)
		if err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		for _, r := range list {
			out = append(out, ServerResource{Server: name, ResourceInfo: r})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Server < out[j].Server })
	if len(out) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

func (m *manager) ReadResource(ctx context.Context, projectDir, server, uri string) ([]Resource, error) {
	cl, err := m.connected(projectDir, server)
	if err != nil {
		return nil, err
	}
	return cl.ReadResource(ctx, uri)
}

func (m *manager) Prompts(ctx context.Context, projectDir string) []ServerPrompt {
	var out []ServerPrompt
	for name, cl := range m.connectedClients(projectDir) {
		if !cl.ServerInfo().Prompts {
			continue
		}
		list, err := cl.ListPrompts(ctx)
		if err != nil {
			continue
		}
		for _, p := range list {
			out = append(out, ServerPrompt{Server: name, Prompt: p})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Server != out[j].Server {
			return out[i].Server < out[j].Server
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (m *manager) GetPrompt(ctx context.Context, projectDir, server, name string, args map[string]string) ([]PromptMessage, error) {
	cl, err := m.connected(projectDir, server)
	if err != nil {
		return nil, err
	}
	return cl.GetPrompt(ctx, name, args)
}

func (m *manager) Status(_ context.Context, projectDir string) ([]Status, error) {
	plans, err := m.plans(projectDir)
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(plans))
	for _, p := range plans {
		out = append(out, m.statusOf(projectDir, p))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// statusOf is p's status: the plan's verdict when it will not be started,
// else its connection's state.
func (m *manager) statusOf(projectDir string, p plan) Status {
	// Saved: this one was read from a file, so it is in one. Only an add that
	// refused to keep a server reports false.
	st := Status{Name: p.srv.Name, Scope: p.srv.Scope, State: p.state, Err: p.err, Saved: true}
	if p.srv.Entry.Remote() {
		st.Transport, st.Target = "http", p.srv.Entry.URL
	} else {
		st.Transport, st.Target = "stdio", strings.TrimSpace(p.srv.Entry.Command+" "+strings.Join(p.srv.Entry.Args, " "))
	}
	if st.State != "" {
		return st
	}
	st.State = StateIdle
	m.mu.Lock()
	c := m.conns[connKey(projectDir, p.srv.Name)]
	m.mu.Unlock()
	if c != nil && c.key == specKey(p.spec) {
		c.mu.Lock()
		st.State, st.Err, st.Tools = c.state, c.err, c.tools
		if c.client != nil {
			st.Info = c.client.ServerInfo()
		}
		c.mu.Unlock()
	}
	return st
}

func (m *manager) Test(ctx context.Context, projectDir, name string) Status {
	plans, err := m.plans(projectDir)
	if err != nil {
		return Status{Name: name, State: StateFailed, Err: err.Error()}
	}
	for _, p := range plans {
		if p.srv.Name != name {
			continue
		}
		if p.state == StateNotTrusted || p.state == StateOff {
			p.state = "" // testing is the person asking for it
		}
		if p.state == "" {
			c := m.ensure(projectDir, p, true)
			c.mu.Lock()
			ready := c.ready
			c.mu.Unlock()
			if ready != nil {
				select {
				case <-ready:
				case <-ctx.Done():
				}
			}
		}
		return m.statusOf(projectDir, p)
	}
	return Status{Name: name, State: StateFailed, Err: fmt.Sprintf("no server named %q", name)}
}

func (m *manager) Forget(projectDir, name string) {
	m.mu.Lock()
	c := m.conns[connKey(projectDir, name)]
	delete(m.conns, connKey(projectDir, name))
	m.mu.Unlock()
	if c != nil {
		go c.close()
	}
}

func (m *manager) reapIdle() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
		}
		m.mu.Lock()
		var idle []*conn
		for _, c := range m.conns {
			c.mu.Lock()
			if c.state == StateConnected && time.Since(c.lastUsed) > m.opts.IdleAfter {
				idle = append(idle, c)
			}
			c.mu.Unlock()
		}
		m.mu.Unlock()
		for _, c := range idle {
			c.close()
		}
	}
}

func (m *manager) Shutdown() {
	m.once.Do(func() { close(m.stop) })
	m.mu.Lock()
	conns := m.conns
	m.conns = map[string]*conn{}
	m.mu.Unlock()
	for _, c := range conns {
		c.close()
	}
}
