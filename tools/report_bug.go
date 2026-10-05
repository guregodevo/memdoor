package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"memdoor/gateway/logs"
)

// ============================================================================
// TOOL: report_bug
// ============================================================================
//
// Lets the AI agent surface anomalies it observes while doing its work — a
// tool the model can call when it notices something that "shouldn't have
// happened." The report becomes a structured WARN/ERROR event in the local
// gateway log; when telemetry is enabled (MEMDOOR_TELEMETRY_ENABLED=1), the
// event ships through the existing gateway/telemetry transport to the memdoor.ai
// monitoring inbox. Same pipeline as the maintainer's 2026-05-29 picker-rejection
// diagnostic logs — agents and the system itself emit through the same
// channel.
//
// Why this isn't a separate emission path: every event memdoor already cares
// about (errors, warnings, anomalous metrics) flows through gateway/logs.
// Wiring this tool to ALSO go through logger.Warn/Error means a single set
// of monitoring queries (`memdoor logs query --component agent-bug-report
// --since 24h`) sees every agent-emitted report alongside the system's own
// silent-failure events. One pipeline, one query surface.
//
// Severity → log-level mapping:
//
//	severity     log level   monitoring impact
//	---------    ---------   -----------------
//	low          WARN        ships under default telemetry filter
//	medium       WARN        ships under default telemetry filter
//	high         ERROR       ships, more visible in error-only dashboards
//	critical     ERROR       ships, expected to page someone in Phase 2
//
// All severities currently land in the same monitoring inbox. The
// distinction is metadata for filtering and future paging policy.

// ReportBugInput is the structured report the agent constructs. Field
// docs in jsonschema_description tags double as inline guidance the
// model sees when deciding what to put in each slot.
type ReportBugInput struct {
	// Title is the one-line summary. The model writes this in its own
	// voice — "grep returned 0 matches for a symbol the file
	// plainly defines" — not a stack trace. Short enough to scan in a dashboard
	// row; specific enough to disambiguate from neighbors.
	Title string `json:"title" jsonschema_description:"One-line summary of the anomaly. Specific, in your own voice — e.g. 'grep returned 0 matches for a symbol the file plainly defines'. Not a stack trace; not a generic 'something failed'."`

	// Severity is the model's judgment of how blocking this is. Used
	// for level mapping and (eventually) Phase 2 paging policy.
	Severity string `json:"severity" jsonschema_description:"Severity assessment: 'low' (cosmetic / one-off), 'medium' (productivity-blocking but worked around), 'high' (couldn't make progress), 'critical' (data loss / user-visible breakage). Be honest — 'critical' should be rare."`

	// WhatITried is the model's narration of the steps that led to
	// the anomaly. The monitoring reader uses this to reproduce —
	// "I called grep for X; got 0 matches; read the file
	// instead; X is defined on line 12." Short paragraphs are fine.
	WhatITried string `json:"what_i_tried" jsonschema_description:"What you actually did, step by step. The reader will use this to reproduce. Mention the specific tool calls / parameters / inputs you used. Avoid 'I tried various things' — be concrete."`

	// WhyItLooksWrong is the model's reasoning for filing this as a
	// bug instead of treating it as user error or transient noise.
	// Forces the model to articulate the discrepancy between observed
	// and expected behavior.
	WhyItLooksWrong string `json:"why_it_looks_wrong" jsonschema_description:"Why you think this is a bug, not normal behavior. Compare observed to expected — 'the docs say X returns Y but I got Z', 'the same call worked an hour ago', etc."`

	// Component is the optional scope tag. When set, monitoring
	// queries can filter by it ("show me all workflow bugs from the last
	// week"). When empty, the tool defaults to "agent-bug-report"
	// so reports stay grouped under one queryable component name.
	Component string `json:"component,omitempty" jsonschema_description:"Optional scope tag — what subsystem the bug is in. Examples: 'workflow', 'providers', 'auth', 'mcp'. Leave empty if unsure; reports are queryable by 'agent-bug-report' as the catch-all."`

	// Evidence is a free-form structured payload the agent attaches:
	// recent log excerpts (from logs_query), tool error messages,
	// status snapshots, model responses. Becomes Data on the emitted
	// event, so a monitoring reader can drill into the same JSON the
	// agent collected.
	Evidence map[string]any `json:"evidence,omitempty" jsonschema_description:"Free-form structured evidence: keys you choose, values that support the report. Useful examples: 'logs_excerpt' (recent log lines from logs_query), 'tool_error' (raw tool failure response), 'expected_vs_observed' ({expected: X, observed: Y}), 'reproduction_steps' (array of CLI commands). The reader sees this verbatim."`
}

// ReportBugInputSchema is the JSON schema the LLM sees when deciding
// whether to call report_bug.
var ReportBugInputSchema = GenerateSchema[ReportBugInput]()

// maxEvidenceBytes caps how much serialized evidence we accept on a
// single report. 64 KB is well above any reasonable "logs excerpt +
// error JSON + before/after snapshot" payload but well below the
// threshold where a runaway agent could fill the local sqlite (and
// the telemetry inbox) with megabytes of attached blobs. The reject
// is a clean error the agent can react to — narrow the evidence
// and retry — rather than a silent truncation that hides what was
// dropped.
const maxEvidenceBytes = 64 * 1024

const reportBugDescription = `Report an anomaly you observed while doing your work. Use this when something happened that shouldn't have — a tool returned an unexpected result, a documented behavior didn't match what you saw, a workflow failed in a way the user can't be expected to debug, or you had to work around something that seemed broken.

WHEN TO USE:
  - A tool call failed in a way that suggests a system bug (not user input error)
  - You expected one outcome based on docs / prior calls / common sense; got a different one
  - You had to retry a deterministic operation more than twice to make progress
  - You noticed silent slow paths (a "fast" operation took 60 seconds, etc.)
  - The user complained about behavior that contradicts what the system claims to do
  - You worked around something by guessing — the next user without your guess will hit the same wall

WHEN NOT TO USE:
  - The user made a typo / asked for something nonsensical — that's not a bug
  - A network call timed out once on a known-flaky link — single transient, don't report
  - You're not sure how something works — use the docs / logs_query first; report only when you've confirmed the discrepancy

HOW TO WRITE A GOOD REPORT:
  - title: one specific sentence ("grep('Foo') returned 0 matches but foo.go defines Foo")
  - severity: be honest. 'critical' = data loss or hard breakage. 'high' = couldn't continue. 'medium' = worked around. 'low' = cosmetic.
  - what_i_tried: concrete steps (tool name + key args), in order. The reader will reproduce from this.
  - why_it_looks_wrong: the discrepancy between observed and expected. If you can't articulate it, you're probably reporting noise.
  - component: the subsystem name if you know it ('workflow', 'providers', 'auth'). Empty is fine.
  - evidence: paste relevant logs_query output, tool error JSON, expected-vs-observed objects. The reader sees this verbatim.

The report becomes a structured WARN/ERROR event in the gateway log. If telemetry is enabled, it ships to the monitoring inbox the team watches. Either way, your report is the SIGNAL — without it, the bug stays silent and the next user hits the same wall.`

// ReportBugDefinition is the tool wiring. Uses FunctionWithContext
// to keep the door open for future per-agent context (workspace,
// session ID, etc.) the wrapper might attach — the ctx parameter is
// currently unused but the call signature matches other tools so
// adding context-aware fields later doesn't require migrating the
// caller.
var ReportBugDefinition = ToolDefinition{
	Name:        "report_bug",
	Description: reportBugDescription,
	InputSchema: ReportBugInputSchema,
	FunctionWithContext: func(input json.RawMessage, _ interface{}) (string, error) {
		var params ReportBugInput
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}
		if strings.TrimSpace(params.Title) == "" {
			return "", fmt.Errorf("title is required")
		}
		if strings.TrimSpace(params.WhatITried) == "" {
			return "", fmt.Errorf("what_i_tried is required so the report is reproducible")
		}
		if strings.TrimSpace(params.WhyItLooksWrong) == "" {
			return "", fmt.Errorf("why_it_looks_wrong is required so we can tell signal from noise")
		}
		// Evidence size cap. Computed against the JSON-serialized
		// payload because that's what lands on disk and on the wire;
		// a Go-side map[string]any can hold things that look small
		// but serialize huge (deeply nested or with large string
		// values). The reject is explicit so the agent gets feedback
		// to narrow rather than a silent truncation.
		if len(params.Evidence) > 0 {
			ser, err := json.Marshal(params.Evidence)
			if err != nil {
				return "", fmt.Errorf("evidence not serializable: %w", err)
			}
			if len(ser) > maxEvidenceBytes {
				return "", fmt.Errorf("evidence too large (%d bytes, max %d) — narrow it (drop verbose log excerpts, keep only the smoking-gun lines)", len(ser), maxEvidenceBytes)
			}
		}

		// Normalize severity → log level. Unknown severities default
		// to WARN so a typo doesn't accidentally suppress the report.
		level := logs.LevelWarn
		switch strings.ToLower(params.Severity) {
		case "low", "medium":
			level = logs.LevelWarn
		case "high", "critical":
			level = logs.LevelError
		default:
			// Empty / unknown → WARN. The model gets pinged in the
			// returned status, in case it wants to amend.
		}

		// Component name: explicit > catch-all. The catch-all
		// component "agent-bug-report" is the canonical query filter
		// for "show me everything agents have reported" — keeping
		// it as the default means a clean baseline before per-subsystem
		// triage starts to matter.
		component := strings.TrimSpace(params.Component)
		if component == "" {
			component = "agent-bug-report"
		}

		// Emit through the standard EventLogger so the event flows
		// through every existing path: the local sqlite, the telemetry
		// wrapper (when enabled), and any future log fan-out. We pass
		// the structured fields as slog attrs so the existing
		// logs_query / memdoor logs query filters work without
		// schema changes.
		log := logs.New(component)
		attrs := []any{
			slog.String("bug_title", params.Title),
			slog.String("bug_severity", normalizedSeverity(params.Severity)),
			slog.String("bug_what_i_tried", params.WhatITried),
			slog.String("bug_why_it_looks_wrong", params.WhyItLooksWrong),
		}
		if len(params.Evidence) > 0 {
			attrs = append(attrs, slog.Any("bug_evidence", params.Evidence))
		}

		switch level {
		case logs.LevelError:
			log.Error(params.Title, attrs...)
		default:
			log.Warn(params.Title, attrs...)
		}

		// Return a small confirmation the agent can show / log. The
		// presence of the "received" key in the result is what tells
		// the model the call succeeded; deliberately terse so it
		// doesn't crowd the model's context.
		result := map[string]any{
			"received":  true,
			"component": component,
			"level":     string(level),
			"severity":  normalizedSeverity(params.Severity),
			"note":      "Bug report written to gateway log. If telemetry is enabled (MEMDOOR_TELEMETRY_ENABLED=1), it has been queued for the monitoring inbox.",
		}
		body, _ := json.Marshal(result)
		return string(body), nil
	},
}

// normalizedSeverity returns the canonical lower-case severity tag.
// Unknown values become "unspecified" so monitoring-side queries can
// distinguish "the agent skipped the field" from "the agent picked
// low explicitly."
func normalizedSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return "low"
	case "medium", "med":
		return "medium"
	case "high":
		return "high"
	case "critical", "crit":
		return "critical"
	default:
		return "unspecified"
	}
}
