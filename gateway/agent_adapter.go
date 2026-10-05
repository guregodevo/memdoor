package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"memdoor/gateway/compaction"
	"memdoor/gateway/config"
	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/gateway/skills"
	"memdoor/pkg/notes"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
	"memdoor/tools"
)

// AgentRuntime provides an interface for AI agent interactions
// Pattern: OpenClaw multi-agent with configuration-based tool filtering
type AgentRuntime struct {
	clientFactory       *providers.ClientFactory
	tools               []tools.ToolDefinition           // All available tools
	sessionsSpawn       *tools.SessionsSpawnTool         // Special tool with runtime dependencies
	taskFlow            *tools.TaskFlowTool              // Special tool: deterministic managed multi-step flow
	sessionsSend        interface{}                      // A2A messaging tool - using interface{} to avoid import cycle
	agentLog            *tools.AgentLogTool              // Agent log reading tool
	secretRepo          repository.AgentSecretRepository // Per-agent secrets
	agentConfig         *config.AgentConfig              // Agent-specific configuration
	compactionConfig    *config.CompactionConfig         // Resolved compaction settings (agent + defaults)
	resolvedModel       string                           // Resolved model name (agent → defaults → fallback)
	persistence         *SessionPersistence
	events              *infra.EventEmitter              // Event streaming (OpenClaw pattern)
	scheduler           cronScheduler                    // the agent's own cron tool, when the gateway runs one
	workflows           workflowOps                      // the agent's workflow tool: the server's own run/stop/approve
	questions           *questionBroker                  // interactive ask_user_question round-trips
	approvals           approvals                        // approval mode's "always" per session (approval.go)
	preflight           *ctxmgmt.PreflightChecker        // Context overflow detection
	compactor           *compaction.Compactor            // Auto-compaction
	compactionPercent   int                              // Stored for lazy preflight/compactor build
	loadedSkills        []skills.Skill                   // Loaded skills (OpenClaw pattern)
	flowTracker         *infra.ExecutionFlowTracker      // Execution flow tracking for visualization
	agentMemRepo        repository.AgentMemoryRepository // Per-agent memory repo (main DB, Raft-replicated)
	memoryStores        sync.Map                         // Per-agent cached MemoryStore adapters: agentName → memory.MemoryStore
	interactionCounters sync.Map                         // Per-agent interaction counters: agentName → *atomic.Int64
	workspaceID         string                           // Workspace ID from config
	workspaceSetting    func(key string) string          // Workspace-settings reader (duck-typed; nil = no guards)
	toolRouter          *toolRouter                      // per-turn toolsAllow from the decision model (tool_routing.go); nil = off
	turnVerdict         *turnVerdict                     // does a turn-ending reply only announce work? (turn_verdict.go); nil = off
	effortJudge         *effortJudge                     // the decision model's reasoning-effort pick (turn_effort.go); nil = default
	verbose             bool
}

// AgentResponse represents the response from the agent
type AgentResponse struct {
	Text          string              `json:"text"`
	ToolsExecuted []ToolExecutionInfo `json:"tools_executed,omitempty"`
	Error         string              `json:"error,omitempty"`
	// Model is the model that answered the turn's last inference, as the
	// vendor names it. The person sees which model answers (Greg,
	// 2026-09-26: the product routes across agents and models, so the model
	// is shown — never its upstream hosts).
	Model string `json:"model,omitempty"`
}

// ToolExecutionInfo tracks tool usage during agent execution
type ToolExecutionInfo struct {
	Name       string `json:"name"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"` // wall time of the tool call; 0 for a call blocked before it ran
}

// GetTools returns all registered tool definitions
func (ar *AgentRuntime) GetTools() []tools.ToolDefinition {
	return ar.tools
}

// NewAgentRuntime creates a new agent runtime instance with default configuration
// Pattern: Backwards compatible constructor
func NewAgentRuntime(apiKey string, verbose bool) (*AgentRuntime, error) {
	// Use default config with main agent
	defaultCfg := config.DefaultConfig()
	return NewAgentRuntimeWithConfig(apiKey, defaultCfg, defaultCfg.GetDefaultAgent(), verbose)
}

// NewAgentRuntimeWithConfig creates a new agent runtime with specific configuration
// Pattern: OpenClaw multi-agent initialization with tool filtering
func NewAgentRuntimeWithConfig(apiKey string, cfg *config.Config, agentCfg *config.AgentConfig, verbose bool) (*AgentRuntime, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if agentCfg == nil {
		agentCfg = cfg.GetDefaultAgent()
	}

	return NewAgentRuntimeWithFactory(providers.NewClientFactory(), apiKey, cfg, agentCfg, nil, verbose, "")
}

// NewAgentRuntimeWithFactory creates an agent runtime using a provider-based client factory.
func NewAgentRuntimeWithFactory(clientFactory *providers.ClientFactory, apiKey string, cfg *config.Config, agentCfg *config.AgentConfig, agentMemRepo repository.AgentMemoryRepository, verbose bool, workspaceSlug string) (*AgentRuntime, error) {
	// Notes belong to the conversation and live under ~/.memdoor, never in
	// the person's project (pkg/notes).
	repo, err := notes.NewFileRepository(shared.MemdoorHome("notes"))
	if err != nil {
		return nil, err
	}
	tools.SetNotesRepository(repo)

	// THE BELT MUST FIT THE BRAIN THAT IS ACTUALLY WEARING IT.
	//
	// context/limits.go has always known to prefer the answering model's
	// window over the 32K placeholder — and nothing ever registered the
	// probe, so remoteWindow stayed nil and every turn was measured against
	// the placeholder. Live 2026-09-16, a clipping run on a 65K model: "Context window exceeded: 32524 tokens >
	// 28672 effective limit", the turn killed by emergency compaction that
	// could not get under a ceiling less than half the real one. Half the
	// window we pay for was unreachable.
	//
	// And the window is the ANSWERING model's: on the person's own key every
	// catalogue model is one /model away, from 128K to 1.3M, and all of them
	// were measured against one fixed 262K (2026-09-29). The catalogue's
	// context_length answers; a model it does not list, or a seat, keeps
	// the engine's.
	ctxmgmt.SetRemoteWindow(func(model string) int {
		re := providers.ActiveRemoteEngine()
		if model != "" {
			// A pinned model is measured against ITS provider's window
			// (providers/registry.go): a Groq pin at 131k while OpenRouter,
			// the active engine, serves 262k (live 2026-10-02). A window
			// nobody states is a guess, said once per model in the log.
			if p, m, ok := providers.FindModel(context.Background(), model); ok && m.Context > 0 {
				if m.ContextSource == providers.ContextDefault {
					warnGuessedWindow(p.ID, m.ID, m.Context)
				}
				return m.Context
			}
		}
		if re == nil {
			return 0
		}
		if re.Byok && model != "" {
			if list, err := theByokCatalog.models(providers.ByokKey()); err == nil {
				for _, m := range list {
					if m.ID == model && m.Context > 0 {
						return m.Context
					}
				}
			}
		}
		return re.CtxLen
	})
	// The OpenRouter entry of the provider registry reads the catalogue
	// with its real prices (byok_catalog.go); every other provider reads its
	// own list endpoint (providers/registry.go).
	providers.OpenRouterModels = func(key string) ([]providers.Model, error) {
		list, err := theByokCatalog.models(key)
		if err != nil {
			return nil, err
		}
		out := make([]providers.Model, 0, len(list))
		for _, m := range list {
			out = append(out, providers.Model{ID: m.ID, Name: m.Name, Context: m.Context, Tools: m.Tools, InPerM: m.InPerM, OutPerM: m.OutPerM, MaxOutput: m.MaxOutput, Thinking: m.Reasoning})
		}
		return out, nil
	}
	// The model's own output cap, from the reference catalogue
	// (providers/reference.go): the per-call cap stays under it and the
	// cut-off escalation stops at it.
	ctxmgmt.SetRemoteOutput(func(model string) int {
		if model == "" {
			return 0
		}
		if _, m, ok := providers.FindModel(context.Background(), model); ok {
			return m.MaxOutput
		}
		return 0
	})
	// Attribution for a company gateway (providers/attribution.go): the
	// project's directory name from the turn; user and team come from the
	// preset (MEMDOOR_ATTRIBUTION), the session id from the context.
	providers.SetAttributionFunc(func(ctx context.Context) map[string]string {
		out := map[string]string{}
		if wd := turnWorkdir(ctx); wd != "" {
			out[providers.AttrProject] = filepath.Base(wd)
		}
		return out
	})
	providers.SetReasoningSupport(func(model string) bool {
		if list, err := theByokCatalog.models(providers.ByokKey()); err == nil {
			for _, m := range list {
				if m.ID == model {
					return m.Reasoning
				}
			}
		}
		return false
	})

	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if agentCfg == nil {
		agentCfg = cfg.GetDefaultAgent()
	}

	// Initialize all available tools
	allTools := []tools.ToolDefinition{
		// Local file tools
		tools.ReadFileDefinition,
		tools.ListFilesDefinition,
		tools.BashDefinition,
		tools.EditFileDefinition,
		tools.ApplyPatchDefinition, // Codex-format patch: single create/edit/delete tool
		tools.LocateDefinition,     // large-repo localization: find WHERE to edit (pkg/coding)
		tools.SkillDefinition,      // load a named workflow (e.g. large-repo-change)

		tools.VerifyDefinition,
		tools.NotesDefinition,
		tools.RecallDefinition,
		// Web tools (OpenClaw parity)
		tools.WebFetchDefinition,
		tools.WebSearchDefinition,
		// Session management tools (OpenClaw parity)
		tools.SessionsListDefinition,
		tools.SessionStatusDefinition,
		tools.SessionsHistoryDefinition,
		// Agent debugging tools
		tools.AgentLogDefinition,
		// Agent-driven bug reporting — the agent's judgment becomes a
		// WARN/ERROR event in gateway/logs, which flows through
		// gateway/telemetry to the monitoring inbox when enabled. See
		// docs/internal/TELEMETRY.md. Available to every agent so any
		// observation worth surfacing has a path out of the silent-
		// failure zone.
		tools.ReportBugDefinition,
		// Inter-session communication tools (OpenClaw parity - Tier 2)
		tools.SessionsSendDefinition,
		// NOTE: SessionsSpawn tool is added later via WireSessionsSpawnTool()
		// because it requires dependencies (SubagentRegistry, QueueManager, SessionManager)
		// Core tools (OpenClaw parity)
		tools.WriteFileDefinition,
		tools.GlobDefinition,
		tools.GrepDefinition,
		// Decision-model tools (pkg/decision): the service is installed by
		// tools.SetDecisionService; palettes decide which agents see them.
		tools.JgrepDefinition,
		tools.JlogsDefinition,
		tools.JreadDefinition,
		tools.DecisionEvaluateDefinition,
		tools.AskUserQuestionDefinition,
		tools.TodoWriteDefinition,
		tools.TodoReadDefinition,
		// Plan mode: the single coding agent, while in plan (read-only) mode,
		// researches and then calls exit_plan_mode to present its plan for
		// Accept/Refuse approval (Claude-style). Available so a plan-mode turn
		// has a way to signal "plan ready".
		tools.ExitPlanModeDefinition,
		tools.SearchReplaceDefinition,
		tools.AgentsListDefinition,
		tools.GatewayDefinition,
		// Memory & Knowledge (OpenClaw parity)
		tools.MemoryDefinition,
		// Centralized event log query — lets an agent answer
		// "what did I do yesterday?" from the same audit trail
		// `memdoor logs query` reads. Reuses
		// gateway/logs.GetGlobalStorage so there's one source of truth
		// for activity history (no parallel log.md file).
		tools.LogsQueryDefinition,
		// Visualization tools removed — not needed for chat agents
		// Advanced capabilities
		tools.ChromeDevToolsDefinition,
		// Context management tools
		tools.ContextDefinition,
		tools.StatusDefinition,
		// cron is built per turn (gateway/cron_tool.go): it carries the turn's
		// directory and session, which a static definition cannot.
		tools.ChannelsDefinition,
		tools.SendInviteDefinition,
		tools.SendEmailDefinition,
	}

	// Initialize session persistence with config (OpenClaw pattern: per-agent JSONL files)
	persistence, err := NewSessionPersistence(cfg, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize persistence: %w", err)
	}

	// Initialize event emitter (OpenClaw pattern: event streaming)
	events := infra.NewEventEmitter(verbose)

	// Resolve compaction config BEFORE creating checkers (agent-specific + defaults fallback)
	// Pattern: OpenClaw config resolution - agent overrides defaults
	compactionCfg := resolveCompactionConfig(cfg, agentCfg)

	// The compaction percent from config; 0 takes the preflight default, 60%
	// of the answering model's window (Greg, 2026-09-29: "60%, capped at
	// 200k"). It was 45 here, and compacted at 49.8% (live 2026-09-29).
	compactionPercent := 0
	if compactionCfg != nil && compactionCfg.CompactionPercent > 0 {
		compactionPercent = compactionCfg.CompactionPercent
	}

	model := resolveModel(clientFactory)
	preflight, err := ctxmgmt.NewPreflightChecker(model, events, compactionPercent, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize preflight checker: %w", err)
	}
	if compactionCfg != nil {
		preflight.SetThresholdTokens(compactionCfg.ThresholdTokens)
	}
	compactor, err := compaction.NewCompactor(model, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize compactor: %w", err)
	}

	// Load skills (OpenClaw pattern)
	loadedSkills, skillDiags := loadSkills(cfg, agentCfg, verbose)
	if len(skillDiags) > 0 {
		log := logs.New("Agent")
		log.Debug("Skills loading diagnostics", slog.String("agent_id", agentCfg.ID))
		for _, diag := range skillDiags {
			log.Debug("Skill diagnostic", slog.String("level", diag.Level), slog.String("message", diag.Message))
		}
	}

	// Initialize execution flow tracker
	flowTracker := infra.NewExecutionFlowTracker(verbose)

	// Create execution_flow tool with access to flowTracker
	executionFlowFunc := tools.CreateExecutionFlowFunction(func() interface{} {
		return flowTracker
	})
	executionFlowDef := tools.ExecutionFlowDefinition
	executionFlowDef.Function = executionFlowFunc
	allTools = append(allTools, executionFlowDef)

	// Initialize agent_log tool (SQLite-based)
	agentLogTool, err := tools.NewAgentLogTool(agentCfg.ID)
	if err != nil {
		log := logs.New("Agent")
		log.Warn("Failed to initialize agent_log tool (will continue without it)",
			slog.String("error", err.Error()))
		agentLogTool = nil
	}

	log := logs.New("Agent")
	log.Debug("Agent runtime initialized",
		slog.String("agent_id", agentCfg.ID),
		slog.Int("total_tools", len(allTools)),
		slog.Int("skills", len(loadedSkills)),
		slog.String("model", model))

	ar := &AgentRuntime{
		clientFactory:     clientFactory,
		tools:             allTools,
		agentLog:          agentLogTool,
		agentConfig:       agentCfg,
		compactionConfig:  compactionCfg,
		resolvedModel:     model,
		persistence:       persistence,
		events:            events,
		questions:         newQuestionBroker(),
		preflight:         preflight,
		compactor:         compactor,
		compactionPercent: compactionPercent,
		loadedSkills:      loadedSkills,
		flowTracker:       flowTracker,
		agentMemRepo:      agentMemRepo,
		workspaceID:       workspaceSlug,
		verbose:           verbose,
	}
	// A stub names its call; recall returns the original from the transcript.
	tools.SetRecaller(ar.recallOutput)
	tools.SetWebSearchBackend(providers.WebSearch)
	return ar, nil
}

// resolveCompactionConfig resolves compaction configuration
// Pattern: OpenClaw config resolution - agent overrides defaults
func resolveCompactionConfig(cfg *config.Config, agentCfg *config.AgentConfig) *config.CompactionConfig {
	// Agent-specific config takes precedence
	if agentCfg != nil && agentCfg.Compaction != nil {
		return agentCfg.Compaction
	}

	// Fallback to global defaults
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil && cfg.Agents.Defaults.Compaction != nil {
		return cfg.Agents.Defaults.Compaction
	}

	// Return nil if no config (will use hardcoded defaults)
	return nil
}

// resolveModel returns the label a request carries before the engine names
// its model (providers.ModelName).
func resolveModel(clientFactory *providers.ClientFactory) string {
	_ = clientFactory
	return providers.ModelName
}

// loadSkills loads and filters skills based on system context
// Pattern: OpenClaw skills loading with eligibility filtering
func loadSkills(cfg *config.Config, agentCfg *config.AgentConfig, verbose bool) ([]skills.Skill, []skills.Diagnostic) {
	var allDiags []skills.Diagnostic

	// Determine skill directories
	// Use workspace-relative paths if available
	workspace := agentCfg.Workspace
	if workspace == "" && cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil {
		workspace = cfg.Agents.Defaults.Workspace
	}
	if workspace == "" {
		workspace, _ = os.Getwd()
	}

	// Define skill search paths (OpenClaw pattern: workspace > user > bundled > plugin)
	bundledSkillsDir := filepath.Join(workspace, "skills") // Project-local skills
	userSkillsDir := shared.MemdoorHome("skills")          // User-global skills

	// Load skills from all sources
	loadOpts := skills.LoadOptions{
		BundledDir:   bundledSkillsDir,
		UserDir:      userSkillsDir,
		WorkspaceDir: "", // No separate workspace dir (using bundled)
		PluginDirs:   []string{},
		Verbose:      verbose,
	}

	allSkills, loadDiags, err := skills.LoadSkills(loadOpts)
	allDiags = append(allDiags, loadDiags...)

	if err != nil {
		allDiags = append(allDiags, skills.Diagnostic{
			Level:   "error",
			Message: fmt.Sprintf("failed to load skills: %v", err),
		})
		return []skills.Skill{}, allDiags
	}

	// Create eligibility context
	envVars := make(map[string]string)
	for _, e := range os.Environ() {
		// Split key=value
		parts := filepath.SplitList(e)
		if len(parts) > 0 {
			// More robust env var parsing
			kv := filepath.SplitList(parts[0])
			if len(kv) == 2 {
				envVars[kv[0]] = kv[1]
			}
		}
	}

	ctx := skills.EligibilityContext{
		OS:      runtime.GOOS,
		Env:     envVars,
		BinPath: filepath.SplitList(os.Getenv("PATH")),
	}

	// Filter eligible skills
	eligibleSkills, filterDiags := skills.FilterEligibleSkills(allSkills, ctx)
	allDiags = append(allDiags, filterDiags...)

	return eligibleSkills, allDiags
}

// agentBackstopTimeout aliases config.AgentBackstopTimeout for the gateway
// package's inference deadline — see that function for the contract (hang
// backstop, not a pacer; all layers must agree).
func agentBackstopTimeout() time.Duration {
	return config.AgentBackstopTimeout()
}

// jobWaitTimeout is how long a synchronous job waiter (heartbeat, cron, A2A)
// waits on its response channel: the execution backstop plus a grace minute.
// The waiter must fire strictly AFTER the execution deadline so the run's own
// error — which names the actual failure — wins the select over the waiter's
// generic "timed out".
func jobWaitTimeout() time.Duration {
	return config.AgentBackstopTimeout() + time.Minute
}

var guessedWindows sync.Map

// warnGuessedWindow says, once per model, that its window is the provider's
// default because neither the provider's list nor the reference states it
// (Greg, 2026-10-02: "will it work for any model?" — for any model someone
// states; this is the other case, made loud).
func warnGuessedWindow(provider, model string, window int) {
	if _, seen := guessedWindows.LoadOrStore(provider+"/"+model, true); seen {
		return
	}
	logs.New("Providers").Warn(fmt.Sprintf("window of %s on %s is a guess (%d tokens): neither its provider nor the reference catalogue states it — set it in ~/.memdoor/providers.json (models: [{id, context}]) or MEMDOOR_MODEL_CONTEXT", model, provider, window),
		slog.String("provider", provider), slog.String("model", model), slog.Int("window", window))
}
