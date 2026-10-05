package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"memdoor/gateway/subagents"
)

// subagentFailure names a failed sub-session's outcome for its requester: a
// turn cut by its own limit is a timeout, told with the limit, so the
// requester can shorten the task or raise runTimeoutSeconds; anything else
// is an error with the runtime's text. Until 2026-09-14 a failed sub-session
// announced nothing and its requester waited for good.
func subagentFailure(err error, limit time.Duration) (status, text string) {
	if errors.Is(err, context.DeadlineExceeded) && limit > 0 {
		return subagents.OutcomeTimeout, fmt.Sprintf("the run was stopped at its %s limit (runTimeoutSeconds) before it finished; give it a shorter task or a longer limit", limit.Round(time.Second))
	}
	return subagents.OutcomeError, err.Error()
}
