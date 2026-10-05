package subagents

import (
	"fmt"
	"strings"
)

// AnnounceParams contains parameters for announcing subagent results
// Pattern: OpenClaw src/agents/announce.ts
type AnnounceParams struct {
	RunID               string
	Label               string
	Task                string
	RequesterSessionKey string
	ChildSessionKey     string
	FinalOutput         string
	Stats               *SubagentStats
	Outcome             *SubagentOutcome
	Check               string // the result review's note, "" when there is none (gateway/result_acceptance.go)
	// Verdict is the result review's answer: VerdictDone, VerdictUnfinished,
	// or "" when the result was not judged. It picks the closing directive.
	Verdict string
}

const (
	VerdictDone       = "done"
	VerdictUnfinished = "unfinished"
)

// SubagentStats contains runtime statistics for a subagent run
type SubagentStats struct {
	RuntimeMs     int64  `json:"runtime_ms"`
	TokensUsed    int    `json:"tokens_used,omitempty"`
	EstimatedCost string `json:"estimated_cost,omitempty"`
}

// BuildAnnouncementMessage builds the announcement message for the main agent
// Pattern: OpenClaw src/agents/announce.ts - buildAnnounceMessage()
//
// The announcement follows OpenClaw's pattern:
// - Brief introduction of the completed task
// - Final output/findings from the subagent
// - Runtime stats (optional)
// - Instructions to summarize naturally for the user
func BuildAnnouncementMessage(params AnnounceParams) string {
	var builder strings.Builder

	// Introduction
	if params.Label != "" {
		builder.WriteString(fmt.Sprintf("A subagent task %q just completed", params.Label))
	} else {
		builder.WriteString("A subagent task just completed")
	}

	// Add status
	if params.Outcome != nil {
		switch params.Outcome.Status {
		case OutcomeError:
			builder.WriteString(" with an error")
		case OutcomeTimeout:
			builder.WriteString(" by running past its time limit")
		default:
			builder.WriteString(" successfully")
		}
	}
	builder.WriteString(".\n\n")

	// Original task
	if params.Task != "" {
		builder.WriteString(fmt.Sprintf("Task: %s\n\n", params.Task))
	}

	// Final output
	if params.FinalOutput != "" {
		builder.WriteString("Findings:\n")
		builder.WriteString(params.FinalOutput)
		builder.WriteString("\n\n")
	} else if params.Outcome != nil && (params.Outcome.Status == OutcomeError || params.Outcome.Status == OutcomeTimeout) {
		builder.WriteString("Error:\n")
		builder.WriteString(params.Outcome.Error)
		builder.WriteString("\n\n")
	}

	if params.Check != "" {
		builder.WriteString(params.Check)
		builder.WriteString("\n\n")
	}

	// Stats (if available)
	if params.Stats != nil {
		var statsParts []string

		// Runtime
		if params.Stats.RuntimeMs > 0 {
			seconds := float64(params.Stats.RuntimeMs) / 1000.0
			statsParts = append(statsParts, fmt.Sprintf("runtime %.1fs", seconds))
		}

		// Tokens
		if params.Stats.TokensUsed > 0 {
			statsParts = append(statsParts, fmt.Sprintf("tokens %d", params.Stats.TokensUsed))
		}

		// Cost
		if params.Stats.EstimatedCost != "" {
			statsParts = append(statsParts, fmt.Sprintf("est %s", params.Stats.EstimatedCost))
		}

		// Session key (for debugging)
		statsParts = append(statsParts, fmt.Sprintf("sessionKey %s", params.ChildSessionKey))

		if len(statsParts) > 0 {
			builder.WriteString("Stats: ")
			builder.WriteString(strings.Join(statsParts, " • "))
			builder.WriteString("\n\n")
		}
	}

	// Directive for the requesting (driver) agent. This is the result of ONE
	// dispatched step, not proof the whole task is done — so DON'T tell the driver
	// to summarize and stop (that made the planner quit after the coder's first
	// write). Tell it to keep driving: verify, then dispatch the next step, and only
	// report done when the whole task is actually complete AND verified.
	//
	// When the result was judged (gateway/result_acceptance.go) the directive
	// follows the verdict. Unconditionally, it told every requester to dispatch
	// more: an agent that had delegated a whole task and got it done spawned a
	// package nobody asked for (2026-09-26). A driver building something larger
	// still reads "dispatch the next step".
	switch params.Verdict {
	case VerdictDone:
		builder.WriteString("The result check found the task above done. If it was one step of a larger job you are " +
			"driving, verify it and dispatch the next small step. If the task above was the whole job, report the result " +
			"in one sentence — do not dispatch more work nobody asked for.")
	case VerdictUnfinished:
		builder.WriteString("The result check found the task above NOT done. Dispatch a fix or the missing piece, or " +
			"finish it yourself, before reporting anything as done.")
	default:
		builder.WriteString("This is ONE step done, NOT the whole task. Dispatch the coder to add the next small piece " +
			"(or the runner/verifier to check it). If it failed, dispatch a fix. Don't stop until the whole program is " +
			"finished and works — only then report the final result in one sentence.")
	}

	return builder.String()
}

// CalculateStats calculates runtime statistics from a SubagentRunRecord
// Pattern: OpenClaw stats collection
func CalculateStats(record *SubagentRunRecord) *SubagentStats {
	if record == nil {
		return nil
	}

	stats := &SubagentStats{}

	// Calculate runtime
	if record.StartedAt != nil && record.EndedAt != nil {
		duration := record.EndedAt.Sub(*record.StartedAt)
		stats.RuntimeMs = duration.Milliseconds()
	}

	// Token usage and cost would be tracked during execution
	// For now, we don't have this data - it would come from the agent runtime
	// In a full implementation, this would be stored in the run record

	return stats
}

// ShouldAnnounce determines if a subagent run should trigger an announcement
// Pattern: OpenClaw announce conditions
func ShouldAnnounce(record *SubagentRunRecord) bool {
	if record == nil {
		return false
	}

	// Only announce completed runs
	if record.EndedAt == nil {
		return false
	}

	// Check if announce was requested
	// In OpenClaw, this is controlled by the announceBack parameter
	// For now, we'll announce all completed subagent runs
	// In a full implementation, this would check record.AnnounceBack field
	return true
}
