package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"memdoor/pkg/decision"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"memdoor/gateway/a2a"
	"memdoor/gateway/broadcast"
	"memdoor/gateway/channels"
	"memdoor/gateway/config"
	"memdoor/gateway/cron"
	"memdoor/gateway/email"
	"memdoor/gateway/flow"
	"memdoor/gateway/health"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/migrations"
	"memdoor/gateway/providers"
	"memdoor/gateway/queue"
	"memdoor/gateway/ratelimit"
	gatewayrepo "memdoor/gateway/repository"
	"memdoor/gateway/streaming"
	"memdoor/gateway/subagents"
	"memdoor/gateway/telemetry"
	"memdoor/pkg/auth"
	"memdoor/pkg/authorization"
	"memdoor/pkg/domain"
	"memdoor/pkg/filestore"
	pkgprompts "memdoor/pkg/prompts"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
	_ "memdoor/pkg/sqlitedriver"

	"github.com/gorilla/websocket"
	mario "github.com/guregodevo/mario/workflow"
)

// AgentEvent type alias for cleaner code
// Pattern: OpenClaw's event streaming
type AgentEvent = infra.AgentEvent

// Server represents the WebSocket gateway server
// SystemAnnouncementPoster defines the interface for posting system announcements
// Pattern: Duck typing - message.Service implements this without importing gateway
type SystemAnnouncementPoster interface {
	PostSystemAnnouncement(ctx context.Context, channelID string, authorID interface{}, text string, parentMessageID int64) error
}

type Server struct {
	cfg               *config.Config               // Configuration system
	repoFactory       repository.RepositoryFactory // Repository factory for database abstraction
	log               *logs.EventLogger            // Structured logger
	port              int
	sessions          *SessionManager
	agent             *AgentRuntime                  // AI agent runtime
	decisions         decision.Service               // Workspace decision model (decisions.go); nil until wired
	acceptance        *resultAcceptance              // Is a spawned agent's result the task done? (result_acceptance.go); nil = off
	clientFactory     *providers.ClientFactory       // LLM provider factory
	runTracker        *RunTracker                    // Async run tracking (OpenClaw pattern)
	queueManager      *queue.QueueManager            // Lane-based queue system
	broadcaster       *broadcast.SubscriptionManager // Event broadcasting
	channelRouter     *channels.Router               // Multi-channel routing
	cronScheduler     *cron.Scheduler                // Cron scheduler
	workflowOnce      sync.Once                      // workflow_runs.go: the runs this gateway knows
	workflowRunsState *workflowRuns
	runReposMu        sync.Mutex                          // workflow_runs.go: guards runRepos
	runRepos          map[string]mario.WorkflowRepository // workflow_runs.go: the run table per project (persistent)
	hostedState       *hostedState                        // hosted_state.go: a Pro account's runs live on memdoor.ai; nil = local only (tests)
	cronLastAnswer    sync.Map                            // cron job id → its last answer, so an unchanged one wakes nobody
	subagentRegistry  *subagents.SubagentRegistry         // Subagent lifecycle tracking
	flowRegistry      *flow.Registry                      // Task Flow driver (managed multi-step sequences)
	// resolveAgentConfig returns a target agent's palette + prompt (resolved at
	// spawn time). Shared by the sessions_spawn wiring and the flow dispatcher.
	resolveAgentConfig func(agentID string) (tools []string, systemPrompt string)
	messageService     SystemAnnouncementPoster    // Message service for posting announcements (duck-typed)
	a2aHandler         *a2a.A2AHandler             // A2A message handler
	a2aMessageQueue    chan *shared.A2AMessage     // A2A message queue
	a2aPolicy          *authorization.A2APolicy    // A2A policy
	a2aResolver        *a2a.SessionResolver        // A2A resolver
	healthCache        *health.Cache               // Health snapshot cache
	rateLimiter        *ratelimit.PerClientLimiter // Per-client rate limiting (10 req/min)
	authAdapter        *AuthGatewayAdapter         // Authentication gateway adapter (uses pkg/auth)
	authzService       *authorization.Service      // Authorization service (permissions & ACL)
	emailService       *email.Service              // Email service for transactional emails
	baseURL            string                      // Base URL for email links
	csrfManager        *CSRFManager                // CSRF token manager for web UI protection
	fileService        *filestore.Service          // File sharing service (shared with ChatServer)
	remoteRelay        *remoteRelayHub             // Remote control relay (remote_relay.go)
	upgrader           websocket.Upgrader
	clients            map[string]*Client
	clientsMu          sync.RWMutex
	runCancels         map[string]context.CancelFunc // sessionKey -> cancel for its running turn (Esc interrupt)
	runCancelsMu       sync.Mutex
	verbose            bool
}

// registerRunCancel records the cancel func for a session's running turn so a
// later "cancel" message can interrupt it. Runs are sequential per session, so a
// plain by-key map is enough.
func (s *Server) registerRunCancel(sessionKey string, cancel context.CancelFunc) {
	s.runCancelsMu.Lock()
	if s.runCancels == nil {
		s.runCancels = make(map[string]context.CancelFunc)
	}
	s.runCancels[sessionKey] = cancel
	s.runCancelsMu.Unlock()
}

// clearRunCancel removes a session's cancel func once its turn finishes.
func (s *Server) clearRunCancel(sessionKey string) {
	s.runCancelsMu.Lock()
	delete(s.runCancels, sessionKey)
	s.runCancelsMu.Unlock()
}

// cancelRun interrupts a session's running turn, if any. Returns whether one was
// active.
func (s *Server) cancelRun(sessionKey string) bool {
	s.runCancelsMu.Lock()
	cancel := s.runCancels[sessionKey]
	s.runCancelsMu.Unlock()
	if cancel != nil {
		cancel()
		return true
	}
	return false
}

// SetMessageService wires the message service for posting system announcements
// Pattern: Dependency injection via setter to avoid circular imports
func (s *Server) SetMessageService(svc SystemAnnouncementPoster) {
	s.messageService = svc
}

// Client represents a connected WebSocket client
// Implements broadcast.Subscriber interface
// Thread-safe: All methods can be called concurrently
type Client struct {
	ID      string
	Conn    *websocket.Conn
	Session *Session
	// Scope is the workspace this connection is scoped to, from the ?workspace=
	// handshake param. Empty falls back to the install's single workspace.
	Scope string
	// Actor is who this connection acts as: the user id behind a validated
	// token, wsActorLocal for the machine's own unproxied connection, or ""
	// for an anonymous listener that may receive broadcast events and nothing
	// else. See wsActor.
	Actor   string
	send    chan []byte  // Buffered channel for outgoing messages
	writeMu sync.Mutex   // Protects concurrent writes to WebSocket connection
	closed  bool         // Indicates if client is closed
	mu      sync.RWMutex // Protects closed flag and send channel operations
	// dropped counts events discarded because the send buffer was full — for a
	// streaming turn, each one is a missing token in what the client renders.
	// Atomic; read via DroppedEvents.
	dropped atomic.Int64
}

// Send implements broadcast.Subscriber interface
// Thread-safe: Can be called by multiple goroutines concurrently (broadcasters)
// Uses RLock to allow concurrent sends from multiple broadcasters
// IMPORTANT: Lock is held during channel send to prevent send-on-closed-channel race
func (c *Client) Send(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	// Keep lock held during send - prevents close() from running between check and send
	select {
	case c.send <- data:
		return nil
	default:
		// The drop stays — blocking the broadcaster on one slow client would
		// stall every other client — but it is counted and said out loud. A
		// dropped event during a streaming turn is a missing token in the
		// rendered text, invisible unless someone stares at the transcript.
		n := c.dropped.Add(1)
		if n == 1 || n%100 == 0 { // first drop, then every 100th: visible, not a flood
			logs.New("WebSocket").Warn("send buffer full — event dropped",
				slog.String("client_id", c.ID),
				slog.Int64("dropped_total", n))
		}
		return fmt.Errorf("client send channel full (%d events dropped so far)", n)
	}
}

// DroppedEvents reports how many events this client has lost to a full send
// buffer since it connected.
func (c *Client) DroppedEvents() int64 { return c.dropped.Load() }

// GetID implements broadcast.Subscriber interface
func (c *Client) GetID() string {
	return c.ID
}

// close safely closes the client's send channel
// Thread-safe: Idempotent - safe to call multiple times
// Uses exclusive Lock to prevent any sends during close
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

// Message represents a WebSocket message
type Message struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// NewServer creates a new gateway server
func NewServer(port int, apiKey string, verbose bool) (*Server, error) {
	// Initialize event-based logging with SQLite storage FIRST
	logsDir := shared.MemdoorHome("logs")

	// Install the telemetry storage decorator BEFORE the logger
	// boots so events flow through the wrapper from the very first
	// write. telemetry.Install reads MEMDOOR_TELEMETRY_ENABLED and
	// related env vars — when disabled, the wrapper is identity and
	// there's no per-event cost. See gateway/telemetry for the full
	// rationale.
	logs.StorageWrapper = func(inner logs.Storage) logs.Storage {
		stack := telemetry.Install(inner, telemetry.GatewayVersion, nil)
		return stack.Storage
	}

	if err := logs.InitGlobalLogger(logsDir, verbose); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	configLog := logs.New("Config")

	// Load config FIRST (before creating AgentRuntime)
	var (
		cfg   *config.Config
		agent *AgentRuntime
	)
	configPath, configErr := config.ResolveConfigPath()
	if configErr != nil {
		configLog.Warn("Failed to resolve config path",
			slog.String("error", configErr.Error()))
		configPath = ""
	}

	cfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		configLog.Warn("Failed to load config, using defaults",
			slog.String("error", err.Error()))
		cfg = config.DefaultConfig()
	} else {
		configLog.Info("Config loaded",
			slog.String("path", configPath),
			slog.String("config_path", configPath))
	}

	// Auto-create database if it doesn't exist (enables web-based onboarding)
	dbLog := logs.New("Config")
	if cfg.Database != nil && cfg.Database.GetDatabaseType() == "sqlite" {
		dbPath := cfg.Database.GetSQLitePath()
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbLog.Info("Database does not exist, will be created on first start",
				slog.String("path", dbPath))
		} else {
			dbLog.Info("Database file verified",
				slog.String("path", dbPath))
		}
	}

	// Create repository factory based on database configuration
	// Pattern: Factory pattern for database abstraction (SQLite now, PostgreSQL later)
	// This allows switching databases by just changing config - no code changes needed
	repoFactory, err := gatewayrepo.NewRepositoryFactory(cfg.Database)
	if err != nil {
		dbLog.Warn("Failed to create repository factory, using defaults",
			slog.String("error", err.Error()))
		// Create default SQLite factory if config is missing
		dataDir := shared.MemdoorHome("data")
		defaultDBPath := filepath.Join(dataDir, "memdoor.db")
		repoFactory, err = gatewayrepo.NewRepositoryFactory(&config.DatabaseConfig{
			Type: "sqlite",
			SQLite: &config.SQLiteDatabaseConfig{
				Path: defaultDBPath,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create default repository factory: %w", err)
		}
		dbLog.Info("Using default SQLite database",
			slog.String("path", defaultDBPath))
	} else {
		dbType := cfg.Database.GetDatabaseType()
		dbLog.Info("Database initialized",
			slog.String("type", dbType))
	}

	// Get default agent from config
	defaultAgent := cfg.GetDefaultAgent()
	if defaultAgent == nil {
		return nil, fmt.Errorf("no default agent found in configuration")
	}

	clientFactory := providers.NewClientFactory()
	agent, err = NewAgentRuntimeWithFactory(clientFactory, apiKey, cfg, defaultAgent, repoFactory.AgentMemories(), verbose, "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Initialize channel router for multi-channel support
	channelRouter := channels.NewRouter(verbose)

	// Create server struct first (needed for executor closure)
	server := &Server{
		cfg:           cfg,              // Store config
		repoFactory:   repoFactory,      // Repository factory for database abstraction
		log:           logs.New("HTTP"), // Structured logger
		port:          port,
		sessions:      NewSessionManager(),
		agent:         agent,
		clientFactory: clientFactory,
		runTracker:    nil,                                       // Will be set below after queueManager
		queueManager:  nil,                                       // Will be set below
		broadcaster:   broadcast.NewSubscriptionManager(verbose), // Event broadcasting
		channelRouter: channelRouter,                             // Multi-channel routing
		healthCache:   health.NewCache(),                         // Health snapshot cache
		csrfManager:   NewCSRFManager(),                          // CSRF protection for web UI
		upgrader: websocket.Upgrader{
			ReadBufferSize:    1024 * 1024, // 1MB read buffer (matches client)
			WriteBufferSize:   1024 * 1024, // 1MB write buffer (matches client)
			EnableCompression: true,        // Enable per-message compression (RFC 7692) - controlled via EnableWriteCompression()
			CheckOrigin:       checkWebSocketOrigin(port),
		},
		clients: make(map[string]*Client),
		verbose: verbose,
	}

	// Create queue manager with proper executor (only once, in stable state)
	// Pattern: Create object in stable state from the start
	server.queueManager = queue.NewQueueManager(server.executeAgentJob, verbose)

	// Create run tracker with queue manager reference for A2A routing
	server.runTracker = NewRunTracker(verbose, server.queueManager)

	queueLog := logs.New("Queue")
	queueLog.Debug("QueueManager initialized (not started yet)")

	// Connect agent event emitter to broadcaster for streaming events
	// This allows agent events to be streamed to WebSocket clients
	streaming.ConnectInfraEventEmitter(agent.EventEmitter(), server.broadcaster, verbose)

	streamingLog := logs.New("Streaming")
	streamingLog.Debug("Agent events connected to broadcaster")

	// Initialize SubagentRegistry for subagent collaboration (SQLite-backed)
	subagentsLog := logs.New("Subagents")
	{
		registry := subagents.NewSubagentRegistry(server.repoFactory.SubagentRuns(), verbose)
		server.subagentRegistry = registry
		subagentsLog.Debug("Registry initialized (SQLite-backed)")

		agent.EventEmitter().OnEvent(func(event infra.AgentEvent) {
			registry.OnLifecycleEvent(&event)
		})
		subagentsLog.Debug("Registry connected to agent lifecycle events")

		// Resolver used AT SPAWN TIME to build a subagent WITH its palette + prompt.
		// Returns empty tools if the agent can't be resolved — the spawn then fails
		// fast rather than launching a blank default agent.
		resolveAgentConfig := func(agentID string) ([]string, string) {
			if server.repoFactory == nil || agentID == "" {
				return nil, ""
			}
			buddy, err := server.repoFactory.Buddies().GetByName(context.Background(), agentID)
			if err != nil || buddy == nil {
				return nil, ""
			}
			personality := ""
			if buddy.Personality != nil {
				personality = *buddy.Personality
			}
			sysPrompt := ""
			if buddy.SystemPrompt != nil {
				sysPrompt = *buddy.SystemPrompt
			}
			// Build the SAME structured prompt the direct-mention path uses
			// (BuildAgentExtraPrompt: personality + agent instructions), so a spawned
			// agent's build is IDENTICAL to the same agent run directly — no thinner,
			// differently-shaped prompt on the spawn path.
			prompt := pkgprompts.BuildAgentExtraPrompt(personality, sysPrompt, "", agentID, "")
			return buddy.Tools, prompt
		}
		// Shared by the flow dispatcher (server_flow.go) so a flow step builds the
		// coder WITH its palette+prompt, exactly like a sessions_spawn dispatch.
		server.resolveAgentConfig = resolveAgentConfig

		if err := server.agent.WireSessionsSpawnTool(registry, server.queueManager, NewSessionManagerAdapter(server.sessions), resolveAgentConfig); err != nil {
			subagentsLog.Warn("Failed to wire sessions_spawn tool",
				slog.String("error", err.Error()))
		} else {
			subagentsLog.Debug("sessions_spawn tool wired to agent runtime")
		}

		// Task Flow driver: deterministic managed multi-step sequences. The server
		// implements flow.Dispatcher (DispatchStep) and tools.FlowStarter (StartFlow).
		flowsDir := shared.MemdoorHome("flows")
		server.flowRegistry = flow.NewRegistry(flowsDir, server, server)
		if err := server.agent.WireTaskFlowTool(server); err != nil {
			subagentsLog.Warn("Failed to wire task_flow tool", slog.String("error", err.Error()))
		} else {
			subagentsLog.Debug("task_flow tool wired to agent runtime")
		}
	}

	// Initialize A2A messaging system for agent-to-agent communication
	a2aLog := logs.New("A2A")
	handler, messageQueue, policy, resolver, err := server.InitializeA2A()
	if err != nil {
		a2aLog.Warn("Failed to initialize A2A system",
			slog.String("error", err.Error()))
	} else {
		server.a2aHandler = handler
		server.a2aMessageQueue = messageQueue
		server.a2aPolicy = policy
		server.a2aResolver = resolver

		// Wire sessions_send tool into agent runtime
		// NOTE: For now, we wire it for the default "main" agent
		// In a multi-agent setup, each agent would get its own sessions_send instance
		agentID := "main"
		if cfg.Agent != nil && cfg.Agent.ID != "" {
			agentID = cfg.Agent.ID
		}

		if err := server.agent.WireSessionsSendTool(policy, resolver, messageQueue, agentID); err != nil {
			a2aLog.Warn("Failed to wire sessions_send tool",
				slog.String("error", err.Error()))
		} else {
			a2aLog.Debug("sessions_send tool wired to agent runtime")
		}

		// Wire A2A policy into sessions_spawn tool for cross-agent authorization
		server.agent.SetSpawnA2AChecker(policy)
	}

	// Wire workspace-settings reader into the runtime so admin-configured tool
	// guards (setting `tool_guards`) apply to every agent tool call.
	if db, ok := repoFactory.DB().(*sql.DB); ok {
		server.agent.SetWorkspaceSettingReader(newWorkspaceSettingReader(db))
	}

	// Initialize channels from configuration with interface-based adapters
	// Pattern: Factory pattern with abstract ChannelAdapter interface
	channelsLog := logs.New("Channels")
	if err := channels.InitializeChannels(cfg, channelRouter, verbose); err != nil {
		channelsLog.Warn("Failed to initialize channels",
			slog.String("error", err.Error()))
	}

	// Register channel message handler for multi-channel support
	// This allows incoming messages from channels (WhatsApp, Telegram, etc.) to be routed to agents
	server.registerChannelMessageHandler()

	channelsLog.Debug("Channel message handler registered")

	// Initialize cron scheduler for scheduled jobs (SQLite-backed, Raft-replicated)
	cronLog := logs.New("Cron")
	// The agent's workflow tool is the server's own run/stop/approve, with or
	// without cron.
	if server.agent != nil {
		server.agent.workflows = server
	}
	if cfg.Cron != nil && cfg.Cron.Enabled {
		// The single-tenant workspace, the one `memdoor cron list` reads: a job
		// the agent schedules must show there, or it is a job nobody can see.
		cronJobStore := cron.NewDBJobStore(server.repoFactory.CronJobs(), domain.DefaultWorkspaceID)
		cronHistoryRepo := server.repoFactory.CronHistory()

		cronScheduler, err := cron.NewScheduler(cfg, server.executeCronJob, cronJobStore, cronHistoryRepo, verbose)
		if err != nil {
			cronLog.Warn("Failed to initialize scheduler",
				slog.String("error", err.Error()))
		} else {
			server.cronScheduler = cronScheduler
			// The agent's own cron tool runs on the same scheduler the person's
			// jobs do, so a poll it schedules is a job you can see and stop.
			if server.agent != nil {
				server.agent.scheduler = cronScheduler
			}
			cronLog.Debug("Scheduler initialized (SQLite-backed)")
		}
	} else {
		cronLog.Debug("Scheduler disabled in config")
	}

	// Initialize authentication service (using pkg/auth with DDD architecture)
	authLog := logs.New("Auth")
	if db, ok := repoFactory.DB().(*sql.DB); ok && db != nil {
		// Run database migrations
		migrationLog := slog.Default().With(slog.String("component", "Migrations"))
		migrationRunner := migrations.NewRunner(db, migrationLog)
		if err := migrationRunner.Run(); err != nil {
			authLog.Warn("Failed to run migrations", slog.String("error", err.Error()))
		} else {
			authLog.Debug("Database migrations completed")
		}

		// Initialize email service (infrastructure layer)
		emailLog := slog.Default().With(slog.String("component", "Email"))
		emailService, err := email.NewService(emailLog)
		if err != nil {
			authLog.Warn("Failed to initialize email service", slog.String("error", err.Error()))
		} else {
			authLog.Debug("Email service initialized")
		}

		// Get base URL from config or environment
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:18789"
		}
		server.emailService = emailService
		server.baseURL = baseURL

		// Use workspace slug for auth service, fallback to default if not set
		wsForAuth := domain.DefaultWorkspaceID

		// Create auth domain service with dependency injection
		// Pattern: Interface-based DDD architecture
		var authRepo auth.Repository = auth.NewSQLiteRepository(db)
		authEmailAdapter := NewEmailServiceAdapter(emailService)
		tokenGen := auth.NewTokenGenerator()
		authService := auth.NewService(authRepo, authEmailAdapter, tokenGen, logs.New("Auth"), baseURL, wsForAuth)

		// Create gateway adapter for HTTP handlers
		server.authAdapter = NewAuthGatewayAdapter(authService)
		authLog.Info("Authentication service initialized (DDD architecture)",
			slog.String("base_url", baseURL),
			slog.String("pattern", "interface-based"))
	} else {
		authLog.Warn("Could not initialize authentication service - database not available or not SQLite")
	}

	// Initialize authorization service (permissions & ACL)
	authzLog := logs.New("Config")
	membershipRepo := repoFactory.ChannelMemberships()
	channelRepo := repoFactory.Channels()
	buddyRepo := repoFactory.Buddies()
	// authRepo is created above in auth service initialization
	var authRepoForAuthz auth.Repository
	if db, ok := repoFactory.DB().(*sql.DB); ok && db != nil {
		authRepoForAuthz = auth.NewSQLiteRepository(db)
	}
	server.authzService = authorization.NewService(membershipRepo, channelRepo, buddyRepo, authRepoForAuthz)
	authzLog.Info("Authorization service initialized with role-based access control")

	// Initialize rate limiter (10 requests per minute per client)
	// Pattern: Per-client token bucket to prevent API abuse
	server.rateLimiter = ratelimit.NewPerClientLimiter(10, 1*time.Minute)

	return server, nil
}

// GetChannelRouter returns the channel router for external access
func (s *Server) GetChannelRouter() *channels.Router {
	return s.channelRouter
}

// checkWebSocketOrigin returns a function that validates the Origin header
// against localhost and the server's own host. Allows connections with no
// Origin header (non-browser clients like CLI tools).
func checkWebSocketOrigin(serverPort int) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Non-browser clients (CLI, TUI) don't send Origin
		}

		// Parse the origin to extract the host
		origin = strings.TrimSpace(origin)
		host := origin

		// Strip scheme if present
		if idx := strings.Index(host, "://"); idx != -1 {
			host = host[idx+3:]
		}
		// Strip path if present
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}

		// Extract hostname without port (handles IPv6 brackets correctly)
		hostname := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}
		// Strip brackets from bare IPv6 like [::1] without port
		hostname = strings.TrimPrefix(strings.TrimSuffix(hostname, "]"), "[")

		// Allow localhost variants
		switch hostname {
		case "localhost", "127.0.0.1", "::1":
			return true
		}

		// Allow the server's own host
		requestHost := r.Host
		if requestHost == host {
			return true
		}

		slog.Warn("WebSocket connection rejected: origin not allowed",
			slog.String("origin", origin),
			slog.String("request_host", requestHost))
		return false
	}
}
