package gateway

// The rung of the agent's model ladder a session runs on, kept on the
// session ("route_tier" metadata) so it holds for the session: the client
// for the rung's model is picked from it (providers.ClientFactory).
// Nothing raises it yet but a test; the escalation on evidence and the person's pin are
// phase 4 (docs/roadmap/SHOULD.md).

const sessionTierKey = "route_tier"

// sessionTier reads the session's tier; metadata read back from disk comes
// as float64, set in memory as int.
func sessionTier(s *Session) int {
	if s == nil {
		return 0
	}
	v, ok := s.GetMetadataValue(sessionTierKey)
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case int:
		return max(t, 0)
	case float64:
		return max(int(t), 0)
	}
	return 0
}
