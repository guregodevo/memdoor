package gateway

import (
	"context"
	"database/sql"
	"fmt"
	iofs "io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/gateway/routing"
	"memdoor/gateway/websocket"
	"memdoor/pkg/auth"
	"memdoor/pkg/authorization"
	"memdoor/pkg/channel"
	"memdoor/pkg/filestore"
	"memdoor/pkg/message"
	"memdoor/pkg/remote"
	"memdoor/pkg/shared"
	"memdoor/tools"
	webembed "memdoor/web"
)

// Server lifecycle management
// Pattern: OpenClaw graceful shutdown with phased cleanup

// authRepositoryAdapter adapts auth.Repository to message.AuthRepository (duck-typing pattern)
// This avoids circular dependencies while allowing message service to look up users by email
type authRepositoryAdapter struct {
	repo auth.Repository
}

// userIDWrapper wraps auth.User to satisfy message.UserWithID interface
type userIDWrapper struct {
	id       string
	username string
}

func (u *userIDWrapper) GetID() string {
	return u.id
}

func (u *userIDWrapper) GetUsername() string {
	return u.username
}

// GetUserByEmail implements message.AuthRepository
func (a *authRepositoryAdapter) GetUserByEmail(ctx context.Context, email string) (message.UserWithID, string, error) {
	user, passwordHash, err := a.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", nil
	}
	return &userIDWrapper{id: user.ID, username: user.Username}, passwordHash, nil
}

// GetUserByUsername implements message.AuthRepository
func (a *authRepositoryAdapter) GetUserByUsername(ctx context.Context, username string) (message.UserWithID, error) {
	user, err := a.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &userIDWrapper{id: user.ID, username: user.Username}, nil
}

// corsOrigins are the only pages that may call this gateway from a browser:
// the web app's dev server and memdoor.ai. No environment variable widens it,
// and there is no "*" (2026-10-04: any website could script a local gateway
// when CORS_ORIGIN was unset). The gateway's own pages are same-origin and
// need no entry.
var corsOrigins = "http://localhost:5173,http://127.0.0.1:5173,https://memdoor.ai"

// addCORS wraps an HTTP handler with CORS headers for corsOrigins.
func addCORS(next http.Handler) http.Handler {
	allowedOrigin := corsOrigins

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		for _, allowed := range strings.Split(allowedOrigin, ",") {
			if strings.TrimSpace(allowed) == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// addSecurityHeaders wraps an HTTP handler with security headers
func addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		// Baseline CSP for the React SPA. style-src allows 'unsafe-inline'
		// because highlight.js / markdown render inline styles; script-src
		// stays 'self' (no 'unsafe-inline') to block inline-script injection.
		// The built index.html's only inline <script> is application/ld+json
		// (data, not executable, so CSP doesn't gate it). If executable
		// inline scripts are ever added, they MUST use per-request nonces —
		// do NOT add 'unsafe-inline' to script-src.
		// 'wasm-unsafe-eval' lets WebAssembly compile and nothing else (JS eval
		// stays refused): the landing demo's terminal player is WebAssembly,
		// and without it the page threw a CompileError and showed nothing
		// (2026-09-28).
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}

// Start starts the gateway server
func Start(ctx context.Context, host string, port int, apiKey string, verbose bool) error {
	server, err := NewServer(port, apiKey, verbose)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Set gateway base URL for agent tools API calls
	tools.SetGatewayBaseURL(host, port)

	// Create ChatServer for Slack-like UI with DDD architecture
	buddyRepo := server.repoFactory.Buddies()
	channelRepo := server.repoFactory.Channels()
	membershipRepo := server.repoFactory.ChannelMemberships()
	messageRepo := server.repoFactory.Messages()
	reactionRepo := server.repoFactory.Reactions()
	agentSecretRepo := server.repoFactory.AgentSecrets()

	// Wire get_secret tool into agent runtime
	server.agent.WireSecretTool(agentSecretRepo)

	// Wire DB to auth adapter for invite-only registration
	if server.authAdapter != nil {
		if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
			server.authAdapter.SetDB(db)
		}
	}

	// Ensure the default agents exist in the buddies table. Each seed uses
	// INSERT OR IGNORE so this is safe to run on every boot — new
	// installs get them via the existing handleSetup path; existing
	// installs that predate v0.3.0 backfill the missing agents the
	// next time the gateway starts. The Server's seed* functions are
	// the single source of truth for the canonical agent definitions.
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		server.ensureDefaultAgents(ctx, db)
	}

	// Create auth repository for authorization service (user role checks)
	var authRepoForAuthz auth.Repository
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		authRepoForAuthz = auth.NewSQLiteRepository(db)
	}

	// Create authorization service (enables channel ACL + user role checks)
	authorizationService := authorization.NewService(membershipRepo, channelRepo, buddyRepo, authRepoForAuthz)

	// Create channel service with authorization
	channelService := channel.NewChannelService(membershipRepo, authorizationService)

	// Wire up the DDD architecture:
	// ChatServer → MessageService → Published Interface → AgentExecutorAdapter → AgentRuntime
	agentExecutorAdapter := NewAgentExecutorAdapter(server.agent, server.sessions) // a /route pin lives on the registered session
	// Make message-path runs Esc-cancellable: the ws "cancel" handler calls
	// s.cancelRun(sessionKey), which finds the cancel registered here.
	agentExecutorAdapter.SetCancelHooks(server.registerRunCancel, server.clearRunCancel)

	// Create remote executor for OpenAI-compatible agents
	remoteExecutor := remote.NewRemoteExecutor()

	// Create adapter for message service (duck-typing pattern)
	// Adapts auth.Repository to message.AuthRepository interface
	authRepoAdapter := &authRepositoryAdapter{repo: authRepoForAuthz}

	messageService := message.NewService(logs.New("Message"), messageRepo, membershipRepo, channelRepo, buddyRepo, authRepoAdapter, agentExecutorAdapter, remoteExecutor)
	// Workspace slug is resolved per-request from ExecutionContext
	authorization.SetLogger(logs.New("HTTP"))

	chatServer := NewChatServer(
		buddyRepo,
		channelRepo,
		reactionRepo,
		server.repoFactory.CronJobs(),
		server.repoFactory.CronHistory(),
		agentSecretRepo,
		channelService,
		messageService,
		server.authzService,
		server.agent,
		server.queueManager,
		server.repoFactory,
	)
	chatHub := chatServer.GetHub()
	go chatHub.Run()

	// Wire up file sharing service (SQLite-backed for Raft replication)
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		fileStore := filestore.NewSQLiteFileStore(db)
		fileSvc := filestore.NewService(
			server.repoFactory.Files(),
			fileStore,
			filestore.DefaultConfig(),
		)
		chatServer.SetFileService(fileSvc)
		chatServer.SetTokenVerifier(server.authAdapter)
		server.fileService = fileSvc
		getChatLogger().Info("File sharing service initialized", slog.String("storage", "sqlite"))
	}

	// Wire up MessageService broadcaster to ChatServer WebSocket hub
	// This allows agent responses to be broadcast in real-time
	// Let jobs created at runtime reach the running scheduler. Without this a
	// cron job is stored, listed by `cron list`, and never fires — the
	// scheduler registers jobs once at start-up (2026-08-24).
	server.wireCronHooks(chatServer)

	messageService.SetBroadcaster(chatServer.CreateBroadcaster())

	// Wire up MessageService execution broadcaster to ChatServer WebSocket hub
	// This allows agent execution events (started, failed, completed) to be broadcast in real-time
	messageService.SetExecutionBroadcaster(chatServer.CreateExecutionBroadcaster())

	// Wire up link preview enricher for persisting enriched names
	// Pattern: Dependency Inversion - domain calls back to gateway for presentation concerns before saving
	messageService.SetLinkPreviewEnricher(chatServer.CreateLinkPreviewEnricher())

	// Wire agent secret counter for auto-injecting get_secret tool
	messageService.SetAgentSecretCounter(agentSecretRepo)

	// Wire remote tool executor for remote agent tool calls
	remoteToolAdapter := NewRemoteToolAdapter(server.agent.GetTools())
	messageService.SetRemoteToolExecutor(remoteToolAdapter)
	logs.New("Agent").Info("Remote tool executor wired - remote agents can now call tools")

	// Wire workspace language resolver (reads from workspace_settings table)
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		messageService.SetLanguageResolver(&workspaceLanguageResolver{db: db})
		// Decision model: service, /api/decisions/evaluate, decision_evaluate
		// tool, and the opt-in message gate (decisions.go).
		server.wireDecisions(db, messageService)
	}

	// Wire message service for posting system announcements (subagent completions)
	// Pattern: Duck typing - messageService implements SystemAnnouncementPoster interface without importing gateway
	server.SetMessageService(messageService)
	// Turn ledger: every dispatch recorded before it can die silently;
	// idempotent on (channel, agent, trigger); the janitor announces losses.
	messageService.SetTurnLedger(newTurnLedger(server.repoFactory.AgentTurns()))

	// Wire up agent todo events to chat WebSocket hub
	// This allows todo updates from agents to be broadcast to chat UI in real-time
	// Pattern: Bridge infra.EventEmitter (agent runtime) → websocket.Hub (chat clients)
	todoLogger := logs.New("TodoBroadcast")
	server.agent.EventEmitter().OnEvent(func(event infra.AgentEvent) {
		// Filter for tool events with type "todo"
		if event.Stream == infra.EventStreamTool {
			if eventType, ok := event.Data["type"].(string); ok && eventType == "todo" {
				// Extract todos from event data
				if todos, ok := event.Data["todos"]; ok {
					// Parse session ID to extract channel and agent information
					session := shared.ParseSessionID(event.SessionID)

					// Get channel ID from session
					channelID := session.GetChannelID()

					// If channel ID not in session ID (e.g., subagent), check session metadata
					if channelID == "" && session.IsSubagent() {
						// Try to get parent channel ID from session metadata
						if sess, err := server.sessions.GetSession(event.SessionID); err == nil {
							if parentChannelID, ok := sess.GetMetadataValue("parent_channel_id"); ok {
								if channelIDStr, ok := parentChannelID.(string); ok && channelIDStr != "" {
									channelID = channelIDStr
									todoLogger.Debug("Retrieved parent channel ID from session metadata",
										slog.String("subagent_session", event.SessionID),
										slog.String("parent_channel", channelID))
								}
							}
						}
					}

					if channelID == "" {
						if session.IsSubagent() {
							todoLogger.Warn("Subagent todo event - no parent channel ID in metadata",
								slog.String("sessionID", event.SessionID),
								slog.String("subagentRun", session.SubagentRun))
						} else {
							todoLogger.Warn("Could not extract channel ID from session",
								slog.String("sessionID", event.SessionID))
						}
						return
					}

					// Broadcast todo update to chat WebSocket clients
					chatHub.BroadcastToChannel(channelID, websocket.WSMessage{
						Type: websocket.ExecutionTodoUpdated,
						Data: mustMarshal(map[string]interface{}{
							"execution_id": event.RunID,
							"agent_id":     extractAgentFromSession(event.SessionID),
							"channel_id":   channelID,
							"todos":        todos,
							"timestamp":    event.Timestamp,
						}),
					})
				}
			}
		}
	})

	// Create authentication middleware
	// This extracts the user from the Authorization header and adds it to the request context
	// Use adapter to bridge gateway.AuthGatewayAdapter to authorization.AuthService interface
	authMiddleware := authorization.NewAuthMiddleware(server.authAdapter)

	// Wire the workspace slug resolver (authz reads an actor's workspace by slug)
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		authMiddleware.SetSlugResolver(&dbSlugResolver{db: db})
	}

	// HTTP endpoints - Gateway RPC
	http.HandleFunc("/ws", server.handleWebSocket)
	http.HandleFunc("/health", server.handleHealth)
	// Remote control (step 2): the encrypted relay. The TUI attaches as the
	// authenticated terminal on /api/relay; the page at /r/<id> is the SPA
	// (the "/" catch-all below) and opens /api/relay/browser (remote_relay.go).
	remoteHub := NewRemoteRelayHub()
	server.remoteRelay = remoteHub
	accountAuth := newRelayAuth(server.authAdapter, brokerBaseURL())
	server.hostedState = newHostedState(brokerBaseURL(), hostedStateURL(), accountToken)
	http.Handle("/api/relay", remoteHub.HandleRelayWS(accountAuth))
	http.HandleFunc("/api/relay/browser", remoteHub.HandleRelayBrowser)
	// /share: sealed transcripts behind https://memdoor.ai/s/<id>#k=<key>
	// (share_store.go). Kept beside the database, never under the deploy dir.
	if dataDir := shared.MemdoorHome("data"); dataDir != "" {
		if shares, err := newShareStore(filepath.Join(dataDir, "shares"), accountAuth); err == nil {
			http.HandleFunc("/api/share", shares.Handle)
			http.HandleFunc("/api/share/", shares.Handle)
		} else {
			logs.New("Share").Warn("share store unavailable: " + err.Error())
		}
	}
	// Mux specificity beats the "/" SPA catch-all, and workspaceMiddleware
	// passes the path through (the dot fails its slug regex).
	// These session/agent/queue endpoints were registered with bare
	// http.HandleFunc — no auth at all — so an anonymous caller could read
	// any session's transcript, wipe history, or enqueue agent runs. The
	// auth middleware only POPULATES the actor (it lets anonymous requests
	// pass through); these wrappers ENFORCE it. Sensitive introspection is
	// admin-only; the agent/message control paths require any authenticated
	// actor (the in-process loopback "system:internal" actor counts as admin,
	// so agent tools and the CLI keep working).
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return addCORS(authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requireAdmin(w, r, server.authzService) {
				return
			}
			h(w, r)
		})))
	}
	authOnly := func(h http.HandlerFunc) http.Handler {
		return addCORS(authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := requireAuth(w, r); !ok {
				return
			}
			h(w, r)
		})))
	}
	http.Handle("/sessions", adminOnly(server.handleSessions))
	http.Handle("/sessions/detail", adminOnly(server.handleSessionDetail))
	http.Handle("/sessions/history", adminOnly(server.handleSessionHistory))
	http.Handle("/sessions/clear", adminOnly(server.handleSessionClear))   // Clear session conversation
	http.Handle("/sessions/rewind", adminOnly(server.handleSessionRewind)) // Undo the last N turns of a session
	http.Handle("/queue", adminOnly(server.handleQueue))
	http.Handle("/api/message", authOnly(server.handleAPIMessage))
	http.Handle("/api/agent", authOnly(server.handleAgentExecution))
	// Profiling endpoints (pprof) are intentionally NOT wired here.
	// Importing net/http/pprof has an init() side effect that registers
	// /debug/pprof/ on http.DefaultServeMux — which nginx fronts — so even
	// a guarded http.Handle couldn't undo it (the init runs first, and a
	// runtime env gate left the import in place, leaking heap dumps with
	// in-memory session tokens + a CPU-profile DoS to anonymous callers).
	// To profile, build with `-tags pprof` (see gateway/pprof_enabled.go);
	// that binary is for local/load-test use, never the public prod build.
	registerPprof()

	// HTTP endpoints - Chat UI (Slack clone) - most specific routes first!
	// Apply authentication middleware to protected endpoints

	// Public endpoints (no authentication required)

	// WebSocket endpoint with token authentication (prevents anonymous access and user impersonation)
	http.Handle("/chat/ws", websocket.HandleWebSocketWithAuth(chatHub, server.authAdapter, checkWebSocketOrigin(server.port)))
	http.Handle("/api/channels/members", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleRoomMembers))))
	http.Handle("/api/channels/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleRoom))))
	http.Handle("/api/channels", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleRooms))))
	http.Handle("/api/messages/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleMessagesWithID))))                     // Handles /api/messages/:id/read and /api/messages/:id/reactions
	http.Handle("/api/messages/unread-mentions", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleUnreadMentionCounts)))) // Get unread @mention counts per channel
	http.Handle("/api/messages/threads", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleListUnreadMentions))))          // Get all unread @mention messages (for Threads view)
	http.Handle("/api/messages", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleMessages))))
	http.Handle("/api/agents/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleAgentsSubpath)))) // /api/agents/{name}/secrets
	http.Handle("/api/agents", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleAgents))))
	http.Handle("/api/files/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleFileByID))))          // File download (supports ?token= for img tags)
	http.Handle("/api/files", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleFileUpload))))         // File upload (multipart)
	http.Handle("/api/cron-history", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleCronHistory)))) // Cron execution history (GET)
	http.Handle("/api/cron-stats", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleCronStats))))     // Cron job statistics (GET)
	http.Handle("/api/cron/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleCronJob))))            // Individual cron job (GET/DELETE)
	http.Handle("/api/cron", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleCronJobs))))            // Cron job list/create (GET/POST)

	// System secrets endpoints (protected - requires authentication)
	http.Handle("/api/secrets/", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleSecrets)))) // Individual secret (GET/DELETE)
	http.Handle("/api/secrets", addCORS(authMiddleware.Handler(http.HandlerFunc(chatServer.handleSecrets))))  // Secret list/create (GET/POST)
	http.Handle("/api/memory/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleMemory))))       // Individual memory (GET/DELETE)
	http.Handle("/api/memory", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleMemory))))        // Memory list/search/store (GET/POST/PUT)

	// Log query endpoints (protected - requires authentication)
	http.Handle("/api/logs/query", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsQuery)))) // Log event query (GET)
	http.Handle("/api/logs/prune", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsPrune)))) // Prune old log events (POST)
	// Automatic log retention: prune events older than 30 days
	// (default 30d; "0"/"off" disables) so events.db can't grow unbounded.
	logs.StartRetention(context.Background(), logs.LogRetention())
	http.Handle("/api/logs/errors", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsErrors))))   // Log errors (GET)
	http.Handle("/api/logs/stats", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsStats))))     // Log statistics (GET)
	http.Handle("/api/logs/trace", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsTrace))))     // Log causal trace (GET)
	http.Handle("/api/logs/session", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLogsSession)))) // Log session timeline (GET)

	// Admin endpoints (protected - requires admin role)
	http.Handle("/api/admin/users", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleListUsers)))) // List all users (admin only)
	http.Handle("/api/mcp/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleMCP))))              // the person's MCP servers: panel, add, sign-in (mcp_handlers.go)
	http.Handle("/api/mcp", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleMCP))))
	http.Handle("/api/workflow/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleWorkflow)))) // DAG runs on mario: run, status, stop, approve (workflow_runs.go)
	http.Handle("/api/workflow", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleWorkflow))))
	http.Handle("/api/models", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleModels))))
	http.Handle("/api/providers", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleProviders))))
	http.Handle("/api/providers/connect", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleProvidersConnect))))
	http.Handle("/api/models/providers", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleModelProviders)))) // the catalogue /model-search reads (models_handler.go)
	http.Handle("/api/models/check", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleModelCheck))))
	http.Handle("/api/route", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleRoute))))                          // which rung answers a conversation, and a pin (route_handler.go)
	http.Handle("/api/sessions/fresh", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleSessionFresh))))          // /fresh and /clear: wipe an agent's memory of a conversation (session_fresh.go)
	http.Handle("/api/sessions/compact", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleSessionCompact))))      // /compact: summarize the conversation now (session_compact.go)
	http.Handle("/api/sessions/handoff", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleSessionHandoff))))      // /handoff: summarize, then start over from it (session_compact.go)
	http.Handle("/api/decisions/evaluate", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleDecisionsEvaluate)))) // Workspace decision model (OpenClaw shape): ok answers or unavailable+reason
	http.Handle("/api/llm/burst", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleLLMBurst))))                   // GET: the brain status and which model answers (footer, gate, account status)
	http.Handle("/api/admin/users/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleUpdateUserRole))))          // Update user role (admin only)

	// Workspace endpoints (protected - requires authentication)
	http.Handle("/api/workspace/users", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleListWorkspaceUsers))))   // List workspace users for mention autocomplete
	http.Handle("/api/workspace/settings", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleWorkspaceSettings)))) // PUT workspace settings (admin only)
	http.Handle("/api/workspace/settings/public", addCORS(http.HandlerFunc(server.handleGetWorkspaceSettingsPublic)))         // GET workspace settings (public, for language detection)
	http.Handle("/api/workspaces/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleDeleteWorkspace))))          // DELETE /api/workspaces/{slug} — admin cascade delete

	// User endpoints (protected - requires authentication)
	http.Handle("/api/users/avatar", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleAvatarUpload))))                  // Upload profile picture
	http.Handle("/api/users/available-for-dm", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleAvailableUsersForDM)))) // List users available for DM
	http.Handle("/api/users/", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleGetUserByID))))                         // Get user by ID

	// Account SSH public keys — the identity the git-over-ssh transport reads

	// Authentication endpoints (using pkg/auth with DDD architecture)
	if server.authAdapter != nil {
		http.Handle("/api/auth/register", addCORS(http.HandlerFunc(server.authAdapter.HandleRegister)))
		http.Handle("/api/auth/login", addCORS(http.HandlerFunc(server.authAdapter.HandleLogin)))
		http.Handle("/api/auth/whoami", addCORS(http.HandlerFunc(server.authAdapter.HandleWhoami)))
		http.Handle("/api/auth/logout", addCORS(http.HandlerFunc(server.authAdapter.HandleLogout)))
		http.Handle("/api/auth/verify-email", addCORS(http.HandlerFunc(server.authAdapter.HandleVerifyEmail)))               // Email verification
		http.Handle("/api/auth/resend-verification", addCORS(http.HandlerFunc(server.authAdapter.HandleResendVerification))) // Resend verification
		http.Handle("/api/auth/forgot-password", addCORS(http.HandlerFunc(server.authAdapter.HandleForgotPassword)))         // Password reset request
		http.Handle("/api/auth/reset-password", addCORS(http.HandlerFunc(server.authAdapter.HandleResetPassword)))           // Password reset
		http.Handle("/api/auth/change-password", addCORS(http.HandlerFunc(server.authAdapter.HandleChangePassword)))         // Self-service password change (authenticated via Bearer token)
	}
	http.Handle("/api/invite/validate", addCORS(http.HandlerFunc(server.handleValidateInvite))) // Validate invite token (public)
	// The landing page's seat request: public POST, rate-limited, stored in
	// seat_requests and mailed to the inbox; the admin list sits behind auth.
	// invite + email run behind auth middleware so the "internal service"
	// actor is set ONLY for loopback X-Internal-Service calls — the handlers
	// must not trust the raw header (a remote attacker could forge it).
	http.Handle("/api/invite", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleSendInvite))))    // Send invite email
	http.Handle("/api/email/send", addCORS(authMiddleware.Handler(http.HandlerFunc(server.handleSendEmail)))) // Send email (agent tool)
	http.Handle("/api/demo-request", addCORS(http.HandlerFunc(server.handleDemoRequest)))                     // Demo request form (public, no auth)
	http.Handle("/api/setup/status", addCORS(http.HandlerFunc(server.handleSetupStatus)))                     // Workspace setup status (public)
	http.Handle("/api/setup", addCORS(http.HandlerFunc(server.handleSetup)))                                  // First-time workspace setup (public, one-time)
	http.Handle("/api/telemetry", addCORS(http.HandlerFunc(server.handleTelemetry)))                          // Receives event batches from remote memdoor installs (public; bearer-token spam deterrent)
	http.Handle("/api/auth/csrf", addCORS(http.HandlerFunc(server.handleGetCSRFToken)))                       // CSRF token endpoint (still using old handler)
	http.Handle("/api/auth/cli", addCORS(http.HandlerFunc(server.handleCLIAuth)))                             // CLI authentication endpoint (still using old handler)

	// Web UI is served from the embedded React bundle (//go:embed
	// web/dist via memdoor/web). The binary is self-contained: no
	// web/dist/ on disk is required at runtime. The build pipeline
	// (`make web-build`) regenerates web/dist/ from web/src/ via
	// `vite build` before `go build` captures it.
	distFS, distErr := iofs.Sub(webembed.Assets, "dist")
	if distErr != nil {
		return fmt.Errorf("embedded web/dist not found: %w", distErr)
	}
	indexHTML, indexErr := iofs.ReadFile(distFS, "index.html")
	if indexErr != nil {
		return fmt.Errorf("embedded web/dist/index.html missing — was `vite build` run before `go build`? (%w)", indexErr)
	}
	serveIndex := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(indexHTML)
	}
	staticFS := newGzipStatic(distFS, http.FileServer(http.FS(distFS)))

	// SPA routes - serve the React app's index.html for client-side routing
	for _, route := range []string{"/login", "/verify-email", "/forgot-password", "/reset-password", "/pricing", "/extension", "/extension/install", "/extension/connect"} {
		http.HandleFunc(route, serveIndex)
	}

	// Docs: serve SPA for navigation routes, static files for .md fetches.
	// The React DocsPage component fetches /docs/README.md via fetch() — those
	// fall through to the embedded static file server. But /docs, /docs/,
	// /docs/creators/getting-started need the SPA to render the React component.
	http.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".md") || strings.HasSuffix(r.URL.Path, ".json") {
			staticFS.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r)
	})
	http.HandleFunc("/docs", serveIndex)

	// Catch-all with SPA fallback. Tries the embedded FS first; if no
	// file matches and the request path has no file extension (so it
	// looks like an SPA route, not an asset fetch), serves index.html
	// so React Router can render it client-side. Without this, deep
	// links and refreshes on the app's own routes 404 from the
	// FileServer because no such file exists in the embedded bundle.
	//
	// The explicit /login and /docs handlers above are still served by
	// their own HandleFunc registrations (they take precedence in the
	// ServeMux); this catch-all only fires for everything else.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			serveIndex(w, r)
			return
		}
		if _, err := iofs.Stat(distFS, clean); err == nil {
			staticFS.ServeHTTP(w, r)
			return
		}
		// No file at that path. If it has a file extension it's a real
		// asset request (e.g. /assets/missing.css) and should 404. If
		// not, treat it as an SPA route and serve index.html — with a 404
		// status when it is not a route the app has: every URL of the old
		// wiki (/cyberlaw/…, /localllm/…) answered 200 with the home page
		// and Google kept three pages of them as copies of it (2026-10-06).
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		if to, ok := legacyRedirects["/"+strings.TrimSuffix(clean, "/")]; ok {
			http.Redirect(w, r, to, http.StatusMovedPermanently)
			return
		}
		if !isSPARoute("/" + clean) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(indexHTML)
			return
		}
		serveIndex(w, r)
	})

	addr := fmt.Sprintf("%s:%d", host, port)

	// Initialize structured logging
	log := logs.New("HTTP")
	queueLog := logs.New("Queue")
	channelsLog := logs.New("Channels")
	cronLog := logs.New("Cron")
	a2aLog := logs.New("A2A")

	log.Info("Starting gateway server",
		slog.Int("port", port),
		slog.String("ws_endpoint", fmt.Sprintf("ws://localhost%s/ws", addr)),
		slog.String("health_endpoint", fmt.Sprintf("http://localhost%s/health", addr)))

	// Start queue manager background workers
	// Pattern: Start background services only AFTER NewServer returns successfully
	server.queueManager.Start()
	queueLog.Info("QueueManager started")

	// Start all registered channel adapters
	if err := server.channelRouter.StartAll(ctx); err != nil {
		channelsLog.Warn("Some channels failed to start",
			slog.String("error", err.Error()))
	} else {
		channelsLog.Info("All channel adapters started")
	}

	// Start cron scheduler for periodic job execution
	if server.cronScheduler != nil {
		if err := server.cronScheduler.Start(); err != nil {
			cronLog.Warn("Failed to start scheduler",
				slog.String("error", err.Error()))
		} else {
			cronLog.Info("Scheduler started",
				slog.Int("job_count", server.cronScheduler.GetJobCount()))
		}
	}

	// Start A2A handler for agent-to-agent messaging
	if server.a2aHandler != nil {
		go server.a2aHandler.Start(ctx)
		a2aLog.Info("A2A message handler started")
	}

	// Start file cleanup goroutine (hourly)
	if chatServer.fileService != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			fileLog := logs.New("FileCleanup")
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					deleted, err := chatServer.fileService.CleanupExpired(context.Background())
					if err != nil {
						fileLog.Warn("File cleanup failed", slog.String("error", err.Error()))
					} else if deleted > 0 {
						fileLog.Info("File cleanup completed", slog.Int("deleted", deleted))
					}
				}
			}
		}()
	}

	// Start HTTP server with security headers on all responses.
	// Wrap with workspace middleware: /{workspace}/api/... → /api/... with
	// workspace set in ExecutionContext. Also handles /{workspace}/chat/ws.
	slugExists := func(string) bool { return false }
	if db, ok := server.repoFactory.DB().(*sql.DB); ok && db != nil {
		slugExists = func(slug string) bool {
			var one int
			return db.QueryRow(`SELECT 1 FROM workspaces WHERE slug = ?`, slug).Scan(&one) == nil
		}
	}
	handler := workspaceMiddleware(addSecurityHeaders(http.DefaultServeMux), slugExists)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 180 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Seed embedded skills to ~/.memdoor/skills before anything can call the
	// skill tool — cheap (hash compares), and makes disk the source of truth.
	SeedEmbeddedSkills()

	// Deploy watchdog: progress announcements, auto-registration, and the
	// idle auto-stop that makes the /model picker's "auto-stops" true.
	server.selectModelEngine()
	server.startTurnJanitor()

	// Handle graceful shutdown. main waits on this WaitGroup after
	// ListenAndServe returns, otherwise there's a race: srv.Shutdown() (called
	// inside this goroutine) unblocks ListenAndServe in the main goroutine,
	// which would let main return — and the process exit — BEFORE this goroutine
	// runs the post-drain steps (Phase 6: flushing logs, Phase 7: closing the
	// database). The WaitGroup makes main block until those finish.
	var shutdownWG sync.WaitGroup
	shutdownWG.Add(1)
	go func() {
		defer shutdownWG.Done()
		<-ctx.Done()
		startTime := time.Now()
		shutdownLog := logs.New("Shutdown")

		shutdownLog.Info("🛑 Graceful shutdown initiated")

		// Phase 1: Stop accepting new work
		shutdownLog.Debug("Phase 1: Stopping new work...")

		// Stop cron scheduler (no more scheduled jobs)
		if server.cronScheduler != nil {
			server.cronScheduler.Stop()
			shutdownLog.Debug("✓ Cron scheduler stopped")
		}

		// Stop channel adapters (no more incoming messages)
		if err := server.channelRouter.StopAll(); err != nil {
			shutdownLog.Warn("⚠ Failed to stop some channels",
				slog.String("error", err.Error()))
		} else {
			shutdownLog.Debug("✓ Channel adapters stopped")
		}

		// Phase 2: Notify clients
		shutdownLog.Debug("Phase 2: Notifying clients...")
		server.notifyClientsShutdown()

		// Phase 3: Drain queues with timeout
		shutdownLog.Debug("Phase 3: Draining queues...")
		drainTimeout := 30 * time.Second
		if !server.drainQueuesWithTimeout(drainTimeout) {
			shutdownLog.Warn("⚠ Queue drain timed out",
				slog.Duration("timeout", drainTimeout))
		} else {
			shutdownLog.Debug("✓ All queues drained")
		}

		// Phase 4: Stop background services
		shutdownLog.Debug("Phase 4: Stopping services...")

		// Stop A2A handler
		if server.a2aHandler != nil {
			server.a2aHandler.Stop()
			shutdownLog.Debug("✓ A2A handler stopped")
		}

		// Stop queue manager
		server.queueManager.Stop()
		shutdownLog.Debug("✓ Queue manager stopped")

		// Close agent runtime (closes session persistence)
		if server.agent != nil {
			if err := server.agent.Close(); err != nil {
				shutdownLog.Warn("⚠ Failed to close agent runtime",
					slog.String("error", err.Error()))
			} else {
				shutdownLog.Debug("✓ Agent runtime closed")
			}
		}

		// Flush subagent registry
		if server.subagentRegistry != nil {
			if err := server.subagentRegistry.Persist(); err != nil {
				shutdownLog.Warn("⚠ Failed to persist subagent registry",
					slog.String("error", err.Error()))
			} else {
				shutdownLog.Debug("✓ Subagent registry persisted")
			}
		}

		// Phase 5: Shutdown HTTP server
		shutdownLog.Debug("Phase 5: Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			shutdownLog.Error("⚠ HTTP server shutdown error",
				slog.String("error", err.Error()))
		} else {
			shutdownLog.Debug("✓ HTTP server stopped")
		}

		// Phase 5a: stop the remote-control relay (its sweeper goroutine and
		// every session's sockets), so a clean shutdown leaves no stray writes.
		if server.remoteRelay != nil {
			server.remoteRelay.Shutdown()
			shutdownLog.Debug("✓ Remote relay stopped")
		}

		elapsed := time.Since(startTime)
		shutdownLog.Info("✅ Graceful shutdown complete",
			slog.Duration("elapsed", elapsed))

		// Phase 6: Flush and close logger
		shutdownLog.Debug("Phase 6: Flushing logs...")
		if err := logs.CloseGlobalLogger(); err != nil {
			// Log to stderr since logger is closing
			fmt.Fprintf(os.Stderr, "⚠ Logger shutdown error: %v\n", err)
		}

		// Phase 7: Close the database. LAST, because the logger writes through
		// it — closing the handle in phase 5 would cut off the very flush that
		// phase 6 exists to perform.
		//
		// Until 2026-08-30 this call did not exist: six phases of careful
		// teardown and the connection was simply abandoned at exit. SQLite
		// commits per statement so no committed row was ever lost, but the WAL
		// was never checkpointed, so every restart left one to be replayed and
		// the file grew without bound. The book survives; closing it properly
		// is what makes that a guarantee rather than a property of the
		// journalling mode.
		if server.repoFactory != nil {
			if err := server.repoFactory.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ Database close error: %v\n", err)
			}
		}
	}()

	serveErr := srv.ListenAndServe()
	// Block until the graceful-shutdown goroutine has finished its post-drain
	// work (log flush, database close). On a clean SIGTERM, ListenAndServe
	// returns http.ErrServerClosed the moment srv.Shutdown() runs; without this
	// Wait, the process could exit mid-teardown.
	shutdownWG.Wait()
	return serveErr
}

// notifyClientsShutdown sends shutdown notification to all WebSocket clients
// Pattern: Graceful client notification before shutdown
func (s *Server) notifyClientsShutdown() {
	shutdownLog := logs.New("Shutdown")

	s.clientsMu.RLock()
	clients := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		if client != nil {
			clients = append(clients, client)
		}
	}
	s.clientsMu.RUnlock()

	if len(clients) == 0 {
		shutdownLog.Debug("✓ No clients to notify")
		return
	}

	// Send shutdown message to all clients
	shutdownMsg := Message{
		Type: "shutdown",
		Data: map[string]interface{}{
			"message": "Gateway is shutting down",
			"reason":  "Server shutdown requested",
		},
	}

	failedCount := 0
	for _, client := range clients {
		if err := client.sendMessage(shutdownMsg); err != nil {
			failedCount++
			shutdownLog.Warn("Failed to notify client",
				slog.String("client_id", client.ID),
				slog.String("error", err.Error()))
		}
	}

	shutdownLog.Info("✓ Notified clients",
		slog.Int("total", len(clients)),
		slog.Int("failed", failedCount))

	// Give clients a moment to receive the message
	time.Sleep(100 * time.Millisecond)
}

// drainQueuesWithTimeout waits for all queues to drain with timeout
// Returns true if queues drained successfully, false if timed out
func (s *Server) drainQueuesWithTimeout(timeout time.Duration) bool {
	shutdownLog := logs.New("Shutdown")
	deadline := time.Now().Add(timeout)
	checkInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		depth := s.queueManager.GetQueueDepth()

		// Check if all queues are empty
		globalDepths := depth["global"].(map[queue.Lane]map[string]interface{})
		allEmpty := true

		for lane, info := range globalDepths {
			queuedCount := info["queued"].(int)
			activeCount := info["active"].(int)

			if queuedCount > 0 || activeCount > 0 {
				allEmpty = false
				shutdownLog.Debug("Waiting for lane to drain",
					slog.String("lane", string(lane)),
					slog.Int("queued", queuedCount),
					slog.Int("active", activeCount))
			}
		}

		if allEmpty {
			return true
		}

		time.Sleep(checkInterval)
	}

	// Timeout - log final state
	depth := s.queueManager.GetQueueDepth()
	globalDepths := depth["global"].(map[queue.Lane]map[string]interface{})
	for lane, info := range globalDepths {
		queuedCount := info["queued"].(int)
		activeCount := info["active"].(int)
		if queuedCount > 0 || activeCount > 0 {
			shutdownLog.Warn("Lane still has pending jobs",
				slog.String("lane", string(lane)),
				slog.Int("queued", queuedCount),
				slog.Int("active", activeCount))
		}
	}

	return false
}

// extractAgentFromSession extracts the agent ID from a session ID
// Supports agent session key format: "agent:<agent-id>:<session-type>"
// Falls back to "main" for legacy or unrecognized formats
func extractAgentFromSession(sessionID string) string {
	return routing.ResolveSessionAgentID(sessionID)
}

// dbSlugResolver resolves a workspace UUID → its slug in the workspaces table.
type dbSlugResolver struct {
	db *sql.DB
}

func (r *dbSlugResolver) ResolveSlug(workspaceID string) string {
	// Try exact match first
	var slug string
	err := r.db.QueryRow(`SELECT slug FROM workspaces WHERE id = ?`, workspaceID).Scan(&slug)
	if err == nil && slug != "" {
		return slug
	}
	// Fallback ONLY for a genuine single-tenant install: when there's exactly
	// one workspace, resolve to it. With multiple workspaces, falling back to
	// "the first" would hand the caller an ARBITRARY tenant's slug — which authz
	// then trusts as their binding — so return "" and let authz fail closed
	// instead of authorizing the wrong workspace.
	var count int
	if cerr := r.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count); cerr == nil && count == 1 {
		if serr := r.db.QueryRow(`SELECT slug FROM workspaces LIMIT 1`).Scan(&slug); serr == nil && slug != "" {
			return slug
		}
	}
	return ""
}

// knownPrefixes are URL path prefixes that are NOT workspace slugs.
// If the first path segment matches one of these, skip workspace extraction.
var knownPrefixes = map[string]bool{
	"api": true, "chat": true, "ws": true,
	"health": true, "login": true, "verify-email": true,
	"forgot-password": true, "reset-password": true, "pricing": true,
	"docs": true, "c": true, "cluster": true, "debug": true,
	"sessions": true, "queue": true, "guide": true, "extension": true,
	"assets": true, "favicon.png": true,
	"icon-192.png": true, "memdoor-logo.png": true, "og-image.png": true,
	"features": true, "workflows": true, "download": true, "devs": true, "setup": true,
	"r": true, "s": true, "topup": true, "pro": true, "dl": true, "billing": true, "state": true,
	"install.sh": true, "install.ps1": true, "sitemap.xml": true, "robots.txt": true, "llms.txt": true,
}

// workspaceMiddleware extracts workspace from URL prefix: /{workspace}/api/...
// Strips the prefix so downstream handlers see /api/... and sets WorkspaceSlug
// in ExecutionContext. Passes through URLs without workspace prefix unchanged.
// Also respects X-Forwarded-Workspace header from load balancer.
// A first segment is a workspace only when a workspace of that slug EXISTS
// (slugExists): until 2026-10-06 any word was one, so /zzz and every URL of
// the old wiki (/cyberlaw, /localllm) were rewritten to / and answered the
// home page with 200 — Google kept three pages of them as copies of it.
func workspaceMiddleware(next http.Handler, slugExists func(string) bool) http.Handler {
	slugRe := regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)

		// Check if first segment is a workspace slug (not a known prefix)
		slug := ""
		if len(parts) >= 1 && parts[0] != "" && !knownPrefixes[parts[0]] && slugRe.MatchString(parts[0]) && slugExists(parts[0]) {
			slug = parts[0]
			// Strip workspace prefix from path
			if len(parts) == 2 {
				r.URL.Path = "/" + parts[1]
			} else {
				r.URL.Path = "/"
			}
		}

		// Fallback: X-Forwarded-Workspace header from load balancer
		if slug == "" {
			slug = r.Header.Get("X-Forwarded-Workspace")
		}

		// If workspace found, set it in context
		if slug != "" {
			execCtx := shared.GetExecutionContext(r.Context())
			if execCtx != nil {
				execCtx.WorkspaceSlug = slug
			} else {
				// Create minimal context with workspace slug
				execCtx = &shared.ExecutionContext{
					WorkspaceSlug: slug,
				}
			}
			ctx := shared.WithExecutionContext(r.Context(), execCtx)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// workspaceLanguageResolver reads workspace language from the workspaces table.
// Implements message.WorkspaceLanguageResolver via duck typing.
type workspaceLanguageResolver struct {
	db *sql.DB
}

func (r *workspaceLanguageResolver) GetLanguage(ctx context.Context) string {
	execCtx := shared.GetExecutionContext(ctx)

	var lang string
	// Resolve by slug first (most specific), then by ID. If neither
	// resolves, return "" — the caller skips injecting the language
	// directive entirely. The previous `SELECT language FROM workspaces
	// LIMIT 1` fallback was a single-tenant relic: in a multi-workspace
	// install it picked an arbitrary workspace's language and forced
	// every other workspace's agent responses into that language. The
	// injected "respond in <lang> ONLY" prompt overrode every other
	// language directive — including ones inside the agent's own
	// system prompt. Returning "" lets the LLM mirror the user's input
	// language naturally.
	if execCtx != nil && execCtx.WorkspaceSlug != "" {
		_ = r.db.QueryRowContext(ctx,
			`SELECT language FROM workspaces WHERE slug = ?`, execCtx.WorkspaceSlug,
		).Scan(&lang)
	}
	if lang == "" && execCtx != nil && execCtx.WorkspaceID != "" {
		_ = r.db.QueryRowContext(ctx,
			`SELECT language FROM workspaces WHERE id = ?`, execCtx.WorkspaceID,
		).Scan(&lang)
	}
	return lang
}
