package context

import (
	"fmt"
	"log/slog"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// PreflightChecker performs context window checks before API calls
// Pattern: OpenClaw pre-flight validation to prevent context overflow
type PreflightChecker struct {
	model             string
	tokenCounter      *TokenCounter
	limits            *ModelLimits
	events            *infra.EventEmitter
	log               *logs.EventLogger
	compactionPercent int // share of the effective limit that triggers compaction (default 60), never past CompactionCeiling
	thresholdTokens   int // a fixed trigger; takes precedence over compactionPercent
}

// NewPreflightChecker creates a new preflight checker
func NewPreflightChecker(
	model string,
	events *infra.EventEmitter,
	compactionPercent int,
	verbose bool,
) (*PreflightChecker, error) {
	limits, err := GetModelLimits(model)
	if err != nil {
		return nil, err
	}

	// Apply default if not configured
	if compactionPercent == 0 {
		compactionPercent = 60 // Lowered to 60% for aggressive early compaction
	}

	return &PreflightChecker{
		model:             model,
		tokenCounter:      NewTokenCounter(model, verbose),
		limits:            limits,
		events:            events,
		log:               logs.New("Agent"),
		compactionPercent: compactionPercent,
	}, nil
}

// CheckResult contains the result of a preflight check
type CheckResult struct {
	TotalTokens    int     `json:"total_tokens"`
	EffectiveLimit int     `json:"effective_limit"`
	Utilization    float64 `json:"utilization"`
	IsOverLimit    bool    `json:"is_over_limit"`
	IsNearLimit    bool    `json:"is_near_limit"`
	ShouldCompact  bool    `json:"should_compact"`
	// CompactAt is the size at which the conversation compacts: the
	// configured share of the effective limit, never past CompactionCeiling.
	CompactAt  int    `json:"compact_at"`
	CanProceed bool   `json:"can_proceed"`
	Warning    string `json:"warning,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Check performs pre-flight context window validation against model's window
// ("" = the checker's own model): the model answering the turn.
// Returns: CheckResult with validation status and recommendations
func (pc *PreflightChecker) Check(
	runID string,
	sessionID string,
	model string,
	systemPrompt string,
	toolSchemas []llm.ToolParam,
	messages []llm.MessageParam,
) (*CheckResult, error) {
	// Count total tokens
	systemTokens := pc.tokenCounter.CountSystemPromptTokens(systemPrompt)
	toolTokens := pc.tokenCounter.CountToolSchemaTokens(toolSchemas)
	conversationTokens := pc.tokenCounter.CountConversationTokens(messages)
	totalTokens := systemTokens + toolTokens + conversationTokens

	if model == "" {
		model = pc.model
	}
	limits := LiveLimits(model, pc.limits)
	effectiveLimit := limits.EffectiveLimit()
	utilization := limits.GetUtilization(totalTokens)

	result := &CheckResult{
		TotalTokens:    totalTokens,
		EffectiveLimit: effectiveLimit,
		Utilization:    utilization,
		IsOverLimit:    limits.IsOverLimit(totalTokens),
		IsNearLimit:    limits.IsNearLimit(totalTokens, 80.0),
		CompactAt:      pc.compactAt(limits),
		CanProceed:     !limits.IsOverLimit(totalTokens),
	}

	result.ShouldCompact = totalTokens >= result.CompactAt

	// Check if over limit
	if result.IsOverLimit {
		result.Error = fmt.Sprintf(
			"Context window exceeded: %d tokens > %d effective limit (%.1f%% utilization)",
			totalTokens, effectiveLimit, utilization,
		)
		result.CanProceed = false

		// Emit overflow error event
		if pc.events != nil {
			pc.events.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
				"event":           "context_overflow",
				"total_tokens":    totalTokens,
				"effective_limit": effectiveLimit,
				"utilization":     utilization,
				"message":         result.Error,
			})
		}

		pc.log.Debug("Context overflow",
			slog.Int("total_tokens", totalTokens),
			slog.Int("effective_limit", effectiveLimit),
			slog.Float64("utilization", utilization))

		return result, fmt.Errorf("%s", result.Error)
	}

	// Check if near limit (warning)
	if result.IsNearLimit {
		result.Warning = fmt.Sprintf(
			"Context window approaching limit: %d / %d tokens (%.1f%% utilization)",
			totalTokens, effectiveLimit, utilization,
		)

		// Emit warning event
		if pc.events != nil {
			pc.events.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
				"event":           "context_warning",
				"total_tokens":    totalTokens,
				"effective_limit": effectiveLimit,
				"utilization":     utilization,
				"threshold":       80.0,
				"message":         result.Warning,
			})
		}

		pc.log.Debug("Context warning",
			slog.Int("total_tokens", totalTokens),
			slog.Int("effective_limit", effectiveLimit),
			slog.Float64("utilization", utilization))
	}

	// Check if should compact (>= configured threshold)
	if result.ShouldCompact {
		pc.log.Debug("Context compaction recommended",
			slog.Float64("utilization", utilization),
			slog.Int("threshold", pc.compactionPercent))
	}

	// Always emit context update event (for TUI display)
	// Use "default" for empty runID/sessionID to ensure event is always sent
	eventRunID := runID
	eventSessionID := sessionID
	if eventRunID == "" {
		eventRunID = "default"
	}
	if eventSessionID == "" {
		eventSessionID = "session_main"
	}

	if pc.events != nil {
		pc.events.EmitEvent(eventRunID, infra.EventStreamLifecycle, eventSessionID, map[string]interface{}{
			"event":           "context_update",
			"total_tokens":    totalTokens,
			"effective_limit": effectiveLimit,
			"utilization":     utilization,
			"tokens":          totalTokens,    // For TUI compatibility
			"limit":           effectiveLimit, // For TUI compatibility
		})
	}

	return result, nil
}

// SetThresholdTokens sets a fixed trigger in tokens (compaction.thresholdTokens):
// it takes precedence over the percentage, never past the effective limit.
func (pc *PreflightChecker) SetThresholdTokens(n int) { pc.thresholdTokens = max(n, 0) }

// CompactAt is the size at which a conversation on model compacts: the
// configured share of its effective limit, never past CompactionCeiling, or
// the fixed threshold when one is set.
func (pc *PreflightChecker) CompactAt(model string) int {
	if model == "" {
		model = pc.model
	}
	return pc.compactAt(LiveLimits(model, pc.limits))
}

// CompactRule says where CompactAt comes from, for /context.
func (pc *PreflightChecker) CompactRule() string {
	if pc.thresholdTokens > 0 {
		return "thresholdTokens"
	}
	return fmt.Sprintf("%d%% of the window, at most %dK", pc.compactionPercent, CompactionCeiling/1000)
}

func (pc *PreflightChecker) compactAt(limits *ModelLimits) int {
	if pc.thresholdTokens > 0 {
		return min(pc.thresholdTokens, limits.EffectiveLimit())
	}
	return min(int(float64(limits.EffectiveLimit())*float64(pc.compactionPercent)/100.0), CompactionCeiling)
}

// CheckQuick performs a quick context check without events
// Useful for status checks and logging
func (pc *PreflightChecker) CheckQuick(
	systemPrompt string,
	toolSchemas []llm.ToolParam,
	messages []llm.MessageParam,
) (*CheckResult, error) {
	return pc.Check("", "", "", systemPrompt, toolSchemas, messages)
}

// GetBreakdown returns detailed token breakdown
func (pc *PreflightChecker) GetBreakdown(
	systemPrompt string,
	toolSchemas []llm.ToolParam,
	messages []llm.MessageParam,
) *ContextBreakdown {
	return pc.tokenCounter.ComputeBreakdown(systemPrompt, toolSchemas, messages)
}

// FormatCheckResult formats check result as human-readable string
func (cr *CheckResult) FormatCheckResult() string {
	status := "OK"
	if cr.IsOverLimit {
		status = "OVER LIMIT"
	} else if cr.IsNearLimit {
		status = "NEAR LIMIT"
	}

	msg := fmt.Sprintf("Context: %d / %d tokens (%.1f%%) - %s",
		cr.TotalTokens, cr.EffectiveLimit, cr.Utilization, status)

	if cr.Warning != "" {
		msg += "\n⚠️  " + cr.Warning
	}

	if cr.Error != "" {
		msg += "\n❌ " + cr.Error
	}

	if cr.ShouldCompact {
		msg += "\n💡 Compaction recommended"
	}

	return msg
}

// ContextHealth represents overall context health status
type ContextHealth struct {
	Status         string  `json:"status"` // "ok", "warning", "critical", "error"
	Utilization    float64 `json:"utilization"`
	TotalTokens    int     `json:"total_tokens"`
	EffectiveLimit int     `json:"effective_limit"`
	CanProceed     bool    `json:"can_proceed"`
	Message        string  `json:"message,omitempty"`
}

// GetHealth returns overall context health assessment
func (pc *PreflightChecker) GetHealth(
	systemPrompt string,
	toolSchemas []llm.ToolParam,
	messages []llm.MessageParam,
) *ContextHealth {
	result, err := pc.CheckQuick(systemPrompt, toolSchemas, messages)
	if err != nil {
		return &ContextHealth{
			Status:         "error",
			Utilization:    result.Utilization,
			TotalTokens:    result.TotalTokens,
			EffectiveLimit: result.EffectiveLimit,
			CanProceed:     false,
			Message:        err.Error(),
		}
	}

	status := "ok"
	message := "Context window healthy"

	if result.IsOverLimit {
		status = "error"
		message = "Context window exceeded - cannot proceed"
	} else if result.ShouldCompact {
		status = "critical"
		message = fmt.Sprintf("Context window at %d%%+ - compaction recommended", pc.compactionPercent)
	} else if result.IsNearLimit {
		status = "warning"
		message = "Context window at 80%+ - approaching limit"
	}

	return &ContextHealth{
		Status:         status,
		Utilization:    result.Utilization,
		TotalTokens:    result.TotalTokens,
		EffectiveLimit: result.EffectiveLimit,
		CanProceed:     result.CanProceed,
		Message:        message,
	}
}
