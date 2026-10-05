package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/sandbox"
)

// ============================================================================
// TOOL: logs_query
// ============================================================================
//
// Lets an agent read past activity from the centralized event log — what ran,
// what failed, which model answered — without a parallel log of its own.
//
// Reuses the same Storage backend the `memdoor logs query` CLI uses (via
// `gateway/logs.GetGlobalStorage`) so there's exactly one source of truth
// for activity history. No new endpoints, no new tables, no parallel log.

type LogsQueryInput struct {
	// Filter by component (e.g. "Agent" for LLM calls and tool runs,
	// "Message" for chat messages). Use empty string to query all.
	Component string `json:"component,omitempty" jsonschema_description:"Component name filter, e.g. 'Agent' for model calls and tool runs, 'Message' for chat messages. Empty = all components."`

	// Time range as a duration string ("5m", "1h", "24h", "7d"). Omit
	// for the default last 24 hours.
	Since string `json:"since,omitempty" jsonschema_description:"Time range — Go duration string like '5m', '1h', '24h', '7d'. Defaults to '24h' if omitted."`

	// Optional regex filter on the message text.
	Regex string `json:"regex,omitempty" jsonschema_description:"Optional regex pattern to filter event messages."`

	// Maximum events to return. Defaults to 30.
	Limit int `json:"limit,omitempty" jsonschema_description:"Maximum number of events to return. Defaults to 30, capped at 200."`

	// Filter to events whose data.workspace equals this slug. Event
	// writers attach the workspace via slog.String("workspace", ws).
	// Empty = inherit the current workspace from the execution context
	// (so the agent doesn't have to know its own workspace name).
	// To deliberately query across all workspaces, pass workspace="*".
	Workspace string `json:"workspace,omitempty" jsonschema_description:"Filter to events whose data.workspace equals this slug. Defaults to the current workspace from execution context. Pass '*' to query across all workspaces (use sparingly)."`
}

var LogsQueryInputSchema = GenerateSchema[LogsQueryInput]()

const logsQueryDescription = `Read the activity log — past agent turns, tool runs, model calls, errors, and any other event written through the centralized event store.

Use this when the user asks about WHAT THE SYSTEM HAS DONE recently. Examples:
  - "what did you run yesterday?"   → component="Agent" since="24h" regex="Tool call started"
  - "show me recent errors"         → since="24h" regex="failed|error"
  - "when did the last turn fail?"  → regex="failed" limit=1
  - "which model answered today?"   → component="Agent" since="24h" regex="model"

Pass workspace="<slug>" when you only want events scoped to one workspace (matches data.workspace).
Omit it to use the current workspace; pass "*" to see all workspaces.

Returns a chronological list (most recent first) of events: timestamp, component, level, message.`

// LogsQueryDefinition is the logs_query tool. Reads from the global event
// store via gateway/logs.GetGlobalStorage — no HTTP round-trip, no auth
// token, no per-call setup. The agent gets observability into past
// operations for free.
//
// Workspace scoping: by default the tool inherits the workspace from the
// execution context (sandbox.SandboxContext.WorkspaceSlug), so the agent
// doesn't have to know its own workspace name. Pass workspace="*" to
// deliberately query across all workspaces (rare). Pass an explicit slug
// to query a different workspace (also rare). The default is the safe
// per-workspace path that matches what the chat UI wants.
var LogsQueryDefinition = ToolDefinition{
	Name:        "logs_query",
	Description: logsQueryDescription,
	InputSchema: LogsQueryInputSchema,
	FunctionWithContext: func(input json.RawMessage, ctx interface{}) (string, error) {
		log := logs.New("logs_query")
		var params LogsQueryInput
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}

		// Defaults
		if params.Limit <= 0 {
			params.Limit = 30
		}
		if params.Limit > 200 {
			params.Limit = 200
		}
		if params.Since == "" {
			params.Since = "24h"
		}

		since, err := time.ParseDuration(params.Since)
		if err != nil {
			return "", fmt.Errorf("invalid since duration %q: %w (use Go duration syntax like '24h', '7d', '30m')", params.Since, err)
		}

		// Resolve workspace scope. Three cases:
		//   1. params.Workspace == "*"  → cross-workspace query (no filter)
		//   2. params.Workspace == ""   → inherit from context (per-ws default)
		//   3. params.Workspace == slug → use as-is (override context)
		workspaceFilter := params.Workspace
		switch workspaceFilter {
		case "*":
			workspaceFilter = "" // explicit cross-workspace
		case "":
			// Try to inherit from sandbox context. If no context (e.g.
			// CLI / non-agent caller), fall through to cross-workspace
			// since we have no scoping signal.
			if ws, werr := extractWorkspaceSlug(ctx); werr == nil {
				workspaceFilter = ws
			}
		}

		storage := logs.GetGlobalStorage()
		if storage == nil {
			return "", fmt.Errorf("event log storage not initialized")
		}

		qb := logs.NewQueryBuilder().
			Since(since).
			Limit(params.Limit)
		if params.Component != "" {
			qb = qb.Components(params.Component)
		}
		if params.Regex != "" {
			qb = qb.MessageRegex(params.Regex)
		}
		if workspaceFilter != "" {
			qb = qb.Workspace(workspaceFilter)
		}

		// QueryEvents → database/sql.QueryContext, which panics on a
		// nil context. The pre-existing code passed nil here and got
		// away with it only because logs_query was rarely called. Use a fresh background context for the per-call
		// scope — the tool runtime doesn't propagate the agent's
		// request context this far down.
		result, err := storage.QueryEvents(context.Background(), qb.Build())
		if err != nil {
			log.Warn("Query failed", slog.String("error", err.Error()))
			return "", fmt.Errorf("query events: %w", err)
		}

		log.Info("Logs query",
			slog.String("component", params.Component),
			slog.String("since", params.Since),
			slog.String("regex", params.Regex),
			slog.String("workspace", workspaceFilter),
			slog.Int("returned", len(result.Events)))

		if len(result.Events) == 0 {
			return fmt.Sprintf("No events found in the last %s%s%s%s.\n",
				params.Since,
				ifNonEmpty(" matching component=", params.Component),
				ifNonEmpty(" matching regex=", params.Regex),
				ifNonEmpty(" in workspace=", workspaceFilter),
			), nil
		}

		// Render as a compact human-readable format the agent can quote in
		// its answer. One line per event: "HH:MM:SS [component] message".
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d event(s) in the last %s:\n\n", len(result.Events), params.Since)
		for _, e := range result.Events {
			component := ""
			if e.Component != "" {
				component = "[" + e.Component + "] "
			}
			fmt.Fprintf(&sb, "%s %s%s\n",
				e.Timestamp.Format("2006-01-02 15:04:05"),
				component,
				e.Message,
			)
		}
		return sb.String(), nil
	},
}

func ifNonEmpty(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

// extractWorkspaceSlug is the workspace a tool call belongs to. The log query
// is its only caller, so it keeps the four lines it needs rather than a
// package of them.
func extractWorkspaceSlug(ctx interface{}) (string, error) {
	sctx, ok := ctx.(sandbox.SandboxContext)
	if !ok || sctx.WorkspaceSlug == "" {
		return "", fmt.Errorf("no workspace available")
	}
	return sctx.WorkspaceSlug, nil
}
