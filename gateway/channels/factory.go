package channels

import (
	"memdoor/gateway/channels/adapters"
	"memdoor/gateway/config"
	"memdoor/gateway/logs"
)

// AdapterFactory creates a channel adapter from configuration
// Pattern: Factory pattern for interface-based initialization
type AdapterFactory func(cfg interface{}, verbose bool) (adapters.ChannelAdapter, error)

// InitializeChannels creates and registers channel adapters from configuration
// Pattern: Config-driven channel initialization with abstract interfaces
func InitializeChannels(cfg *config.Config, router *Router, verbose bool) error {
	log := logs.New("Channels")

	if cfg.Channels == nil {
		log.Debug("No channels configured")
		return nil
	}

	// Slack (future)
	if cfg.Channels.Slack != nil && cfg.Channels.Slack.Enabled {
		log.Debug("Slack adapter not yet implemented")
	}

	return nil
}
