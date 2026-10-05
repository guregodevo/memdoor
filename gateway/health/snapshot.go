package health

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"memdoor/gateway/config"
	"memdoor/pkg/shared"
)

var (
	gatewayStartTime = time.Now()

	// ATOMIC, not plain ints. Every WebSocket connection increments these from
	// its own HTTP handler goroutine, so unsynchronised counters were a real
	// data race on every connect and disconnect — the detector fails on it in
	// TestServer_InvalidMessage. A dropped increment would only ever be a
	// wrong number on a status page, but the race is undefined behaviour, and
	// counters are the cheapest possible thing to make correct.
	totalConnections  atomic.Int64
	activeConnections atomic.Int64
)

// IncrementActiveConnections increments the active connection counter
func IncrementActiveConnections() {
	activeConnections.Add(1)
	totalConnections.Add(1)
}

// DecrementActiveConnections decrements the active connection counter
func DecrementActiveConnections() {
	activeConnections.Add(-1)
}

// GenerateSnapshot creates a comprehensive health snapshot
func GenerateSnapshot(cfg *config.Config, opts SnapshotOptions) (*HealthSummary, error) {
	start := time.Now()

	// Get gateway health
	gatewayHealth := getGatewayHealth(cfg)

	// Get default agent ID
	defaultAgentID := getDefaultAgentID(cfg)

	// Get all agents
	agents := getAgentsHealth(cfg, defaultAgentID)

	// Get session health for default agent
	sessionsHealth := getSessionsHealth(cfg, defaultAgentID)

	summary := &HealthSummary{
		OK:             true,
		Timestamp:      time.Now().Unix(),
		DurationMs:     time.Since(start).Milliseconds(),
		Gateway:        gatewayHealth,
		DefaultAgentID: defaultAgentID,
		Agents:         agents,
		Sessions:       sessionsHealth,
	}

	return summary, nil
}

func getGatewayHealth(cfg *config.Config) GatewayHealth {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	port := 18789 // Default port, no gateway config in Config

	uptime := int64(time.Since(gatewayStartTime).Seconds())

	agentsCount := 0
	if cfg != nil && cfg.Agents != nil {
		agentsCount = len(cfg.Agents.List)
	}

	return GatewayHealth{
		Version:     "1.0.0",
		Port:        port,
		Uptime:      uptime,
		StartTime:   gatewayStartTime.Unix(),
		ActiveConns: int(activeConnections.Load()),
		TotalConns:  totalConnections.Load(),
		MemoryUsage: memStats.Alloc,
		Goroutines:  runtime.NumGoroutine(),
		ToolsCount:  0, // Will be updated when tool registry is available
		AgentsCount: agentsCount,
	}
}

func getDefaultAgentID(cfg *config.Config) string {
	// Get the default agent from config
	defaultAgent := cfg.GetDefaultAgent()
	if defaultAgent != nil {
		return defaultAgent.ID
	}
	return "main" // Default agent ID
}

func getAgentsHealth(cfg *config.Config, defaultAgentID string) []AgentHealthSummary {
	agents := []AgentHealthSummary{}

	if cfg == nil || cfg.Agents == nil {
		// Return default agent
		defaultAgent := AgentHealthSummary{
			AgentID:   defaultAgentID,
			IsDefault: true,
			Sessions:  getSessionsHealth(cfg, defaultAgentID),
		}
		return []AgentHealthSummary{defaultAgent}
	}

	// Add configured agents
	for _, agent := range cfg.Agents.List {
		isDefault := agent.ID == defaultAgentID

		agentHealth := AgentHealthSummary{
			AgentID:   agent.ID,
			Name:      agent.Name,
			IsDefault: isDefault,
			Sessions:  getSessionsHealth(cfg, agent.ID),
		}
		agents = append(agents, agentHealth)
	}

	// If no agents configured, add default
	if len(agents) == 0 {
		defaultAgent := AgentHealthSummary{
			AgentID:   defaultAgentID,
			IsDefault: true,
			Sessions:  getSessionsHealth(cfg, defaultAgentID),
		}
		agents = append(agents, defaultAgent)
	}

	return agents
}

func getSessionsHealth(cfg *config.Config, agentID string) SessionsHealth {
	// Determine session store path
	sessionsPath := shared.MemdoorHome("sessions")

	// Try to read session files
	sessions := []RecentSession{}
	count := 0

	entries, err := os.ReadDir(sessionsPath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
				continue
			}

			count++

			// Get file info for last modified time
			info, err := entry.Info()
			if err != nil {
				continue
			}

			sessionKey := entry.Name()
			// Remove .jsonl extension
			if len(sessionKey) > 6 {
				sessionKey = sessionKey[:len(sessionKey)-6]
			}

			updatedAt := info.ModTime().UnixMilli()
			age := time.Now().UnixMilli() - updatedAt

			sessions = append(sessions, RecentSession{
				Key:       sessionKey,
				UpdatedAt: &updatedAt,
				Age:       &age,
			})
		}
	}

	// Sort by most recent first
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt == nil {
			return false
		}
		if sessions[j].UpdatedAt == nil {
			return true
		}
		return *sessions[i].UpdatedAt > *sessions[j].UpdatedAt
	})

	// Keep only recent 5
	if len(sessions) > 5 {
		sessions = sessions[:5]
	}

	return SessionsHealth{
		Path:   sessionsPath,
		Count:  count,
		Recent: sessions,
	}
}

// SessionStore represents a session store file
type SessionStore map[string]SessionEntry

// SessionEntry represents a session entry
type SessionEntry struct {
	UpdatedAt int64 `json:"updatedAt,omitempty"`
}
