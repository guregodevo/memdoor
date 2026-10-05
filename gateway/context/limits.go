package context

import (
	"fmt"
)

// remoteWindow, when set, reports the window of model on the ACTIVE remote
// engine (0 = no remote engine): the model's own when the catalogue knows it,
// else the engine's. Registered by the gateway at startup — a hook instead of
// an import so context stays dependency-free of providers.
var remoteWindow func(model string) int

// SetRemoteWindow registers the active-remote-window probe.
func SetRemoteWindow(fn func(model string) int) { remoteWindow = fn }

// remoteOutput, when set, reports the model's own output cap (0 = unknown):
// the reference catalogue's figure (providers/reference.go). A per-call cap
// never exceeds it — a model capped at 8k refuses a 16k request — and the
// cut-off escalation stops at it.
var remoteOutput func(model string) int

// SetRemoteOutput registers the model-output-cap probe.
func SetRemoteOutput(fn func(model string) int) { remoteOutput = fn }

// CompactionCeiling is the most a conversation carries before it compacts,
// however large the model's window: a 1M-token model at 60% would carry
// 600K tokens into every request, each one paid for and read (Greg,
// 2026-09-29: "60%, capped at 200k").
const CompactionCeiling = 200_000

// ModelLimits defines context window limits for a specific model
// Pattern: OpenClaw model-specific limits with compaction reserves
type ModelLimits struct {
	ContextWindow     int     // Total context window size
	CompactionReserve int     // Tokens reserved for compaction headroom
	MaxOutputTokens   int     // Maximum output tokens per request
	MaxOutputCeiling  int     // The model's own output cap, when known (0 = unknown); escalation stops here
	Temperature       float64 // Recommended temperature for this model
	TopP              float64 // Recommended top_p for this model
}

// Model config — one source of truth for all model parameters.
// One entry per model name we accept on the buddy `model_name`
// column; preflight, compactor, memory flush, and the inspector all
// look the limits up through GetModelLimits. Adding a new provider
// without an entry here means GetModelLimits returns "unknown
// model" and the agent fails the preflight, so each supported
// default-model name across all providers needs to be registered.
var modelLimits = map[string]*ModelLimits{
	// "default" is the label an agent carries before a provider names its
	// model (providers.ModelName). The real window comes from the provider's
	// own list (GetModelLimits, below); this floor is used only when none
	// reports one, small enough to fit any model the agent could be routed to.
	"default": {
		ContextWindow:     32_768,
		CompactionReserve: 4_096,
		MaxOutputTokens:   4_096,
		Temperature:       0.7,
		TopP:              0.95,
	},
}

// GetModelLimits returns the limits for a specific model. When a remote
// engine is ACTIVE, model's window on it overrides this fallback — the
// belt must fit the brain that is actually wearing it, and that is the model
// answering the turn (a rung, or one pinned with /model).
func GetModelLimits(model string) (*ModelLimits, error) {
	if remoteWindow != nil {
		if w := remoteWindow(model); w > 0 {
			// The reply's room is reserved out of the window, so a prompt
			// under the effective limit always fits with its reply.
			out := min(w/4, 16_384)
			ceiling := 0
			if remoteOutput != nil {
				if o := remoteOutput(model); o > 0 {
					ceiling = o
					out = min(out, o)
				}
			}
			return &ModelLimits{
				ContextWindow:     w,
				CompactionReserve: out,
				MaxOutputTokens:   out,
				MaxOutputCeiling:  ceiling,
				Temperature:       0.7,
				TopP:              0.95,
			}, nil
		}
	}
	limits, exists := modelLimits[model]
	if !exists {
		return nil, fmt.Errorf("unknown model %q — add it to modelLimits in context/limits.go", model)
	}
	return limits, nil
}

// LiveLimits is the model's limits NOW, falling back to the ones known at
// construction. The window belongs to the engine serving the turn, and that
// can change under a running gateway (a `/model` pin, another provider): a checker that kept the limits it was built
// with measured a 1.3M-token model against the local 32K profile and
// compacted twice in one review (2026-09-27 22:19 and 22:25).
func LiveLimits(model string, built *ModelLimits) *ModelLimits {
	if l, err := GetModelLimits(model); err == nil {
		return l
	}
	return built
}

// EffectiveLimit returns the effective context limit after reserves
// Pattern: OpenClaw effectiveLimit = MODEL_LIMITS[model] - COMPACTION_RESERVE
func (ml *ModelLimits) EffectiveLimit() int {
	return ml.ContextWindow - ml.CompactionReserve
}

// IsOverLimit checks if token count exceeds effective limit
func (ml *ModelLimits) IsOverLimit(tokens int) bool {
	return tokens > ml.EffectiveLimit()
}

// IsNearLimit checks if token count is approaching limit (>= threshold%)
func (ml *ModelLimits) IsNearLimit(tokens int, thresholdPercent float64) bool {
	threshold := float64(ml.EffectiveLimit()) * (thresholdPercent / 100.0)
	return float64(tokens) >= threshold
}

// GetUtilization returns context utilization as percentage (0-100)
func (ml *ModelLimits) GetUtilization(tokens int) float64 {
	return float64(tokens) / float64(ml.EffectiveLimit()) * 100.0
}

// FormatLimits returns a human-readable string of limits
func (ml *ModelLimits) FormatLimits() string {
	return fmt.Sprintf(`Context Window: %d tokens
Compaction Reserve: %d tokens
Effective Limit: %d tokens
Max Output: %d tokens`,
		ml.ContextWindow,
		ml.CompactionReserve,
		ml.EffectiveLimit(),
		ml.MaxOutputTokens,
	)
}

// ContextStatus represents current context window status
type ContextStatus struct {
	TotalTokens    int     `json:"total_tokens"`
	ContextWindow  int     `json:"context_window"`
	EffectiveLimit int     `json:"effective_limit"`
	Utilization    float64 `json:"utilization"` // Percentage (0-100)
	IsNearLimit    bool    `json:"is_near_limit"`
	IsOverLimit    bool    `json:"is_over_limit"`
}
