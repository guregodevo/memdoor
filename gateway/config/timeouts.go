package config

import (
	"time"
)

// AgentBackstopTimeout is how long a single agent execution may run before
// being reaped as hung. It is a hang backstop, NOT a pacer: every layer that
// bounds an agent turn (outer execution in pkg/message, inner inference in
// the agent runtime, cron/heartbeat/A2A waiters) must use this same value,
// or the shortest layer silently kills slow-but-healthy turns (ff04d7f, then
// the same bug again one layer deeper at 600s).
func AgentBackstopTimeout() time.Duration { return 1800 * time.Second }
