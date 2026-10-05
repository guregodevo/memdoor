package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"memdoor/pkg/authorization"
	"memdoor/pkg/domain"
	"memdoor/pkg/sandbox"
	"memdoor/pkg/shared"
)

func (cs *ChatServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		cs.handleCreateAgent(w, r)

	case http.MethodGet:
		// Get agents from repository (DDD model)
		ctx := r.Context()

		if cs.buddyRepo == nil {
			// Fallback to mock if repository not initialized
			getChatLogger().Info("buddyRepo not initialized, returning mock agents")
			// Mock placeholder while the repo finishes initializing.
			// Once the repo is up, real buddy rows replace this.
			agents := []map[string]interface{}{
				{
					"id":      "agent-writer",
					"name":    "writer",
					"profile": "full",
					"tools":   []string{"web_search", "read_file", "write_file"},
					"status":  "online",
				},
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"agents": agents,
			})
			return
		}

		buddies, err := cs.buddyRepo.List(ctx, shared.AllPages())
		if err != nil {
			getChatLogger().Info("Failed to list agents from repository",
				slog.String("error", err.Error()),
				slog.String("error_type", fmt.Sprintf("%T", err)))
			http.Error(w, fmt.Sprintf("Failed to list agents: %v", err), http.StatusInternalServerError)
			return
		}

		getChatLogger().Info("Retrieved buddies from repository", slog.Int("count", len(buddies)))

		// RBAC: Filter admin-only agents for non-admin users
		agentActorID := authorization.GetActorID(r.Context())
		isAgentListAdmin := false
		if cs.authzService != nil && agentActorID != "" {
			isAgentListAdmin, _ = cs.authzService.IsWorkspaceAdmin(r.Context(), agentActorID)
		}
		if !isAgentListAdmin {
			visible := make([]*domain.Buddy, 0, len(buddies))
			for _, b := range buddies {
				if !b.AdminOnly {
					visible = append(visible, b)
				}
			}
			buddies = visible
		}

		// Convert domain models to API response
		agents := make([]map[string]interface{}, len(buddies))
		for i, buddy := range buddies {
			agents[i] = map[string]interface{}{
				"id":            fmt.Sprintf("agent:%s", buddy.Name),
				"name":          buddy.Name,
				"avatar_emoji":  buddy.AvatarEmoji,
				"avatar_url":    buddy.AvatarURL,
				"icon":          buddy.Icon,
				"profile":       "full",
				"tools":         buddy.Tools,
				"sandbox_scope": string(buddy.SandboxScope),
				"status":        statusFromActive(buddy.IsActive),
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": agents,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAgentsSubpath routes /api/agents/{name}[/secrets[/{secretName}]]
func (cs *ChatServer) handleAgentsSubpath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	agentName := parts[0]

	// Route: /api/agents/{name}/secrets[/{secretName}]
	if len(parts) >= 2 && parts[1] == "secrets" {
		secretName := ""
		if len(parts) == 3 {
			secretName = parts[2]
		}
		cs.handleAgentSecrets(w, r, agentName, secretName)
		return
	}

	// Route: /api/agents/{name} — GET or PUT
	if len(parts) == 1 {
		cs.handleAgentByName(w, r, agentName)
		return
	}

	http.NotFound(w, r)
}

// handleAgentByName handles GET/PUT for a single agent
func (cs *ChatServer) handleAgentByName(w http.ResponseWriter, r *http.Request, agentName string) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	buddy, err := cs.buddyRepo.GetByName(ctx, agentName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %s", agentName), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":               fmt.Sprintf("agent:%s", buddy.Name),
			"name":             buddy.Name,
			"avatar_emoji":     buddy.AvatarEmoji,
			"icon":             buddy.Icon,
			"tools":            buddy.Tools,
			"sandbox_scope":    string(buddy.SandboxScope),
			"learning_enabled": buddy.LearningEnabled,
			"status":           statusFromActive(buddy.IsActive),
			"description":      buddy.Description,
			"personality":      buddy.Personality,
			"system_prompt":    buddy.SystemPrompt,
			"execution_type":   buddy.ExecutionType,
		})

	case http.MethodPut:
		// Mutating an agent — including repointing it at a remote endpoint —
		// is admin-only. (GET above is read-only and intentionally open, like
		// the agent list.)
		if !requireAdmin(w, r, cs.authzService) {
			return
		}
		var req struct {
			LearningEnabled *bool                     `json:"learning_enabled"`
			Tools           []string                  `json:"tools"`
			SandboxScope    *string                   `json:"sandbox_scope"`
			AvatarEmoji     *string                   `json:"avatar_emoji"`
			Description     *string                   `json:"description"`
			Personality     *string                   `json:"personality"`
			SystemPrompt    *string                   `json:"system_prompt"`
			ExecutionType   *string                   `json:"execution_type"`
			Endpoint        *string                   `json:"endpoint"`
			AdminOnly       *bool                     `json:"admin_only"`
			RemoteConfig    *domain.RemoteAgentConfig `json:"remote_config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.LearningEnabled != nil {
			buddy.LearningEnabled = *req.LearningEnabled
		}
		if req.Tools != nil {
			buddy.Tools = req.Tools
		}
		if req.SandboxScope != nil {
			buddy.SandboxScope = sandbox.SandboxScope(*req.SandboxScope)
		}
		if req.AvatarEmoji != nil {
			buddy.AvatarEmoji = *req.AvatarEmoji
		}
		if req.Description != nil {
			buddy.Description = req.Description
		}
		if req.Personality != nil {
			buddy.Personality = req.Personality
		}
		if req.SystemPrompt != nil {
			buddy.SystemPrompt = req.SystemPrompt
		}
		if req.ExecutionType != nil {
			buddy.ExecutionType = *req.ExecutionType
		}
		if req.AdminOnly != nil {
			buddy.AdminOnly = *req.AdminOnly
		}
		if req.RemoteConfig != nil {
			buddy.RemoteConfig = req.RemoteConfig
			if buddy.ExecutionType != "remote" {
				buddy.ExecutionType = "remote"
			}
		}
		if req.Endpoint != nil {
			if buddy.RemoteConfig == nil {
				buddy.RemoteConfig = &domain.RemoteAgentConfig{}
			}
			buddy.RemoteConfig.Endpoint = *req.Endpoint
			if buddy.ExecutionType != "remote" {
				buddy.ExecutionType = "remote"
			}
		}

		if err := cs.buddyRepo.Update(ctx, buddy); err != nil {
			http.Error(w, fmt.Sprintf("Failed to update agent: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "updated",
			"name":    buddy.Name,
			"message": fmt.Sprintf("Agent %s updated successfully", buddy.Name),
		})

	case http.MethodDelete:
		if !requireAdmin(w, r, cs.authzService) {
			return
		}
		// Delete by name. The handler already has the name from the URL
		// path, so the GetByName → Delete(buddy.ID) round-trip the
		// previous version did was redundant — and worse, observed
		// 2026-04-11 to fail when GetByName returned an `id` value
		// that no row in the buddies table actually had (the
		// stale-id-state bug). Deleting by name uses the column the
		// caller already identified the row by and avoids the round
		// trip entirely.
		if err := cs.buddyRepo.DeleteByName(ctx, buddy.Name); err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete agent: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "deleted",
			"name":    buddy.Name,
			"message": fmt.Sprintf("Agent %s deleted successfully", buddy.Name),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAgentSecrets handles agent secret CRUD operations
func (cs *ChatServer) handleAgentSecrets(w http.ResponseWriter, r *http.Request, agentName, secretName string) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	if cs.agentSecretRepo == nil {
		http.Error(w, "Agent secrets not available", http.StatusInternalServerError)
		return
	}

	// Authorization: admin only
	if !requireAdmin(w, r, cs.authzService) {
		return
	}
	actorID := authorization.GetActorID(ctx)

	agentID := fmt.Sprintf("agent:%s", agentName)

	switch r.Method {
	case http.MethodGet:
		// List secret names for agent
		names, err := cs.agentSecretRepo.List(ctx, agentID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list secrets: %v", err), http.StatusInternalServerError)
			return
		}
		if names == nil {
			names = []string{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent":   agentName,
			"secrets": names,
		})

	case http.MethodPost:
		// Set a secret
		var req struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Value == "" {
			http.Error(w, "name and value are required", http.StatusBadRequest)
			return
		}
		if err := cs.agentSecretRepo.Set(ctx, agentID, req.Name, req.Value, string(actorID)); err != nil {
			http.Error(w, fmt.Sprintf("Failed to set secret: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case http.MethodDelete:
		if secretName == "" {
			http.Error(w, "Secret name is required for deletion", http.StatusBadRequest)
			return
		}
		if err := cs.agentSecretRepo.Delete(ctx, agentID, secretName); err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete secret: %v", err), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSecrets handles /api/secrets and /api/secrets/{name}
// System-level secrets stored with agentID="system" in the agent_secrets table.
func (cs *ChatServer) handleSecrets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	if cs.agentSecretRepo == nil {
		http.Error(w, `{"error":"secrets not available"}`, http.StatusInternalServerError)
		return
	}

	// Admin only. System secrets are global API keys (LLM, email); any
	// authenticated user — including guest tokens — must not be able to read,
	// overwrite, or delete them.
	if !requireAdmin(w, r, cs.authzService) {
		return
	}
	actorID := authorization.GetActorID(ctx)

	const systemScope = "system"

	// Extract secret name from path: /api/secrets/{name}
	secretName := strings.TrimPrefix(r.URL.Path, "/api/secrets/")
	if secretName == r.URL.Path {
		secretName = "" // path was exactly /api/secrets
	}

	switch r.Method {
	case http.MethodGet:
		if secretName != "" {
			// Get specific secret value (masked)
			value, err := cs.agentSecretRepo.Get(ctx, systemScope, secretName)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"secret not found: %s"}`, secretName), http.StatusNotFound)
				return
			}
			masked := "****"
			if len(value) > 8 {
				masked = value[:4] + "..." + value[len(value)-4:]
			}
			json.NewEncoder(w).Encode(map[string]string{
				"name":  secretName,
				"value": masked,
			})
		} else {
			// List all secret names
			names, err := cs.agentSecretRepo.List(ctx, systemScope)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"failed to list secrets: %v"}`, err), http.StatusInternalServerError)
				return
			}
			if names == nil {
				names = []string{}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"secrets": names,
			})
		}

	case http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Value == "" {
			http.Error(w, `{"error":"name and value are required"}`, http.StatusBadRequest)
			return
		}
		if err := cs.agentSecretRepo.Set(ctx, systemScope, req.Name, req.Value, string(actorID)); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to set secret: %v"}`, err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "name": req.Name})

	case http.MethodDelete:
		if secretName == "" {
			http.Error(w, `{"error":"secret name is required"}`, http.StatusBadRequest)
			return
		}
		if err := cs.agentSecretRepo.Delete(ctx, systemScope, secretName); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to delete secret: %v"}`, err), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// statusFromActive converts buddy active status to agent status string
func statusFromActive(isActive bool) string {
	if isActive {
		return "online"
	}
	return "offline"
}

// handleCreateAgent handles POST /api/agents for creating new agents
func (cs *ChatServer) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if cs.buddyRepo == nil {
		http.Error(w, "Buddy repository not available", http.StatusInternalServerError)
		return
	}

	// Only workspace admins can create agents.
	if !requireAdmin(w, r, cs.authzService) {
		return
	}

	// Parse request body. No LLM provider/model on buddy rows: the
	// active engine and the agent's ladder decide the model.
	var req struct {
		Name            string   `json:"name"`
		AvatarEmoji     string   `json:"avatar_emoji"`
		Description     *string  `json:"description"`
		Personality     *string  `json:"personality"`
		SystemPrompt    *string  `json:"system_prompt"`
		Tools           []string `json:"tools"`
		SandboxScope    *string  `json:"sandbox_scope"` // Optional, defaults to "user"
		LearningEnabled bool     `json:"learning_enabled"`
		// Remote agent fields
		ExecutionType string                    `json:"execution_type"` // "local" (default) or "remote"
		RemoteConfig  *domain.RemoteAgentConfig `json:"remote_config"`  // Required when execution_type="remote"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	// Validate based on execution type
	execType := req.ExecutionType
	if execType == "" {
		execType = "local"
	}
	if execType != "local" && execType != "remote" {
		http.Error(w, "execution_type must be 'local' or 'remote'", http.StatusBadRequest)
		return
	}

	if execType == "remote" {
		if req.RemoteConfig == nil {
			http.Error(w, "remote_config is required for remote agents", http.StatusBadRequest)
			return
		}
		if req.RemoteConfig.Endpoint == "" {
			http.Error(w, "remote_config.endpoint is required", http.StatusBadRequest)
			return
		}
		if req.RemoteConfig.Model == "" {
			http.Error(w, "remote_config.model is required", http.StatusBadRequest)
			return
		}
	}

	// Determine sandbox scope (default to "user" if not specified)
	scopeStr := "user"
	if req.SandboxScope != nil && *req.SandboxScope != "" {
		scopeStr = *req.SandboxScope
	}

	// Validate sandbox scope
	scope := sandbox.SandboxScope(scopeStr)
	if !scope.IsValid() {
		http.Error(w, "Invalid sandbox_scope: must be 'user', 'channel', or 'workspace'", http.StatusBadRequest)
		return
	}

	// Create domain.Buddy model. No LLM provider/model on the
	// buddy row: the engine is global.
	buddy := &domain.Buddy{
		ID:              uuid.New(),
		Name:            req.Name,
		AvatarEmoji:     req.AvatarEmoji,
		Description:     req.Description,
		Personality:     req.Personality,
		SystemPrompt:    req.SystemPrompt,
		ExecutionType:   execType,
		RemoteConfig:    req.RemoteConfig,
		Temperature:     0.6,        // a tool-calling default
		MaxTokens:       4096,       // Default max tokens
		Skills:          []string{}, // Empty skills for now
		Tools:           req.Tools,
		SandboxScope:    scope, // Set validated sandbox scope
		LearningEnabled: true,  // Learning on by default — memory retrieval + reflection
		IsActive:        true,
		AdminOnly:       true,                                                   // New agents are admin-only by default
		CreatedBy:       uuid.MustParse("00000000-0000-0000-0000-000000000002"), // Default system user
		WorkspaceScoped: domain.WorkspaceScoped{
			WorkspaceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Default workspace
		},
	}

	// Save to database
	if err := cs.buddyRepo.Create(ctx, buddy); err != nil {
		getChatLogger().Info("Failed to create buddy", slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to create agent: %v", err), http.StatusInternalServerError)
		return
	}

	getChatLogger().Info("Created new agent",
		slog.String("name", buddy.Name),
		slog.String("id", buddy.ID.String()))

	// Return success response
	response := map[string]interface{}{
		"id":             buddy.ID.String(),
		"name":           buddy.Name,
		"avatar_emoji":   buddy.AvatarEmoji,
		"description":    buddy.Description,
		"personality":    buddy.Personality,
		"execution_type": buddy.ExecutionType,
		"tools":          buddy.Tools,
		"sandbox_scope":  string(buddy.SandboxScope),
		"status":         "online",
		"created_at":     buddy.CreatedAt.Format(time.RFC3339),
	}
	if buddy.RemoteConfig != nil {
		response["remote_config"] = map[string]interface{}{
			"endpoint": buddy.RemoteConfig.Endpoint,
			"model":    buddy.RemoteConfig.Model,
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
