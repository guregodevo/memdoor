package gateway

import (
	"encoding/json"

	"memdoor/gateway/logs"
	"memdoor/gateway/rpc"
)

// handleAgentRPC handles the "agent" RPC method
// Pattern: OpenClaw src/gateway/server-methods/agent.ts - agent handler
func (s *Server) handleAgentRPC(c *Client, msg *Message) {
	// Parse params from message data
	var params rpc.AgentParams

	// Convert map to JSON and back to struct (simple approach)
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.sendMessage(Message{
			Type:  "error",
			Error: "Failed to parse agent params: " + err.Error(),
		})
		return
	}

	if err := json.Unmarshal(dataBytes, &params); err != nil {
		c.sendMessage(Message{
			Type:  "error",
			Error: "Invalid agent params: " + err.Error(),
		})
		return
	}

	// Use client's session if not specified
	if params.SessionKey == "" {
		params.SessionKey = c.Session.ID
	}

	// Call RPC handler
	result, err := rpc.AgentHandler(params, s.queueManager, s.verbose)
	if err != nil {
		log := logs.New("RPC")
		log.WithError(err).Error("Agent RPC call failed")
		c.sendMessage(Message{
			Type:  "error",
			Error: err.Error(),
		})
		return
	}

	// Send response
	c.sendMessage(Message{
		Type: "agent_response",
		Data: map[string]interface{}{
			"run_id":      result.RunID,
			"status":      result.Status,
			"accepted_at": result.AcceptedAt,
		},
	})
}

// handleAgentWaitRPC handles the "agent.wait" RPC method
// Pattern: OpenClaw src/gateway/server-methods/agent.ts - agent.wait handler
func (s *Server) handleAgentWaitRPC(c *Client, msg *Message) {
	// Parse params from message data
	var params rpc.AgentWaitParams

	// Convert map to JSON and back to struct
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.sendMessage(Message{
			Type:  "error",
			Error: "Failed to parse agent.wait params: " + err.Error(),
		})
		return
	}

	if err := json.Unmarshal(dataBytes, &params); err != nil {
		c.sendMessage(Message{
			Type:  "error",
			Error: "Invalid agent.wait params: " + err.Error(),
		})
		return
	}

	// Use subagentRegistry for waiting on subagent runs (Week 25-26)
	// Pattern: SubagentRegistry implements rpc.RunTracker interface
	if s.subagentRegistry == nil {
		c.sendMessage(Message{
			Type:  "error",
			Error: "Subagent registry not initialized",
		})
		return
	}

	// Call RPC handler
	result, err := rpc.AgentWaitHandler(params, s.subagentRegistry, s.verbose)
	if err != nil {
		log := logs.New("RPC")
		log.WithError(err).Error("Agent wait RPC call failed")
		c.sendMessage(Message{
			Type:  "error",
			Error: err.Error(),
		})
		return
	}

	// Send response
	responseData := map[string]interface{}{
		"run_id": result.RunID,
		"status": result.Status,
	}

	if result.StartedAt != nil {
		responseData["started_at"] = result.StartedAt
	}
	if result.EndedAt != nil {
		responseData["ended_at"] = result.EndedAt
	}
	if result.Error != "" {
		responseData["error"] = result.Error
	}

	c.sendMessage(Message{
		Type: "agent_wait_response",
		Data: responseData,
	})
}
