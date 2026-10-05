package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleMemory handles /api/memory endpoints for per-agent memory CRUD
// Routes: GET /api/memory?agent=X (list/search), POST (store), PUT (update), DELETE (delete)
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Reading an agent's memory requires authentication; writing/deleting
	// is admin-only — otherwise anyone could poison or wipe an agent's
	// memory store. In-process agent tools run as system:internal, which
	// passes both gates.
	if _, ok := requireAuth(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		if !requireAdmin(w, r, s.authzService) {
			return
		}
	}

	agentID := r.URL.Query().Get("agent")
	if agentID == "" {
		agentID = "main"
	}

	store := s.agent.GetMemoryStore(agentID)
	if store == nil {
		http.Error(w, `{"error":"memory store not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Route by path suffix for retrieve/delete single memory
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/memory/") {
		memID := strings.TrimPrefix(path, "/api/memory/")
		if memID != "" {
			switch r.Method {
			case http.MethodGet:
				mem, err := store.Retrieve(memID)
				if err != nil {
					http.Error(w, fmt.Sprintf(`{"error":"memory not found: %s"}`, memID), http.StatusNotFound)
					return
				}
				json.NewEncoder(w).Encode(mem)
			case http.MethodDelete:
				if err := store.Delete(memID); err != nil {
					http.Error(w, fmt.Sprintf(`{"error":"failed to delete: %v"}`, err), http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": memID})
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query().Get("query")
		if query != "" {
			// Search
			limit := 10
			if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
				fmt.Sscanf(limitStr, "%d", &limit)
			}
			results, err := store.Search(query, limit)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"search failed: %v"}`, err), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"count":   len(results),
				"results": results,
			})
		} else {
			// List
			var tags []string
			if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
				tags = strings.Split(tagsStr, ",")
			}
			memories, err := store.List(tags)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"list failed: %v"}`, err), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"count":    len(memories),
				"memories": memories,
			})
		}

	case http.MethodPost:
		// Store new memory
		var req struct {
			Content  string         `json:"content"`
			Tags     []string       `json:"tags,omitempty"`
			Metadata map[string]any `json:"metadata,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Content == "" {
			http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
			return
		}
		id, err := store.Store(req.Content, req.Tags, req.Metadata)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to store: %v"}`, err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "stored"})

	case http.MethodPut:
		// Update existing memory
		var req struct {
			ID       string         `json:"id"`
			Content  *string        `json:"content,omitempty"`
			Tags     []string       `json:"tags,omitempty"`
			Metadata map[string]any `json:"metadata,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}
		if err := store.Update(req.ID, req.Content, req.Tags, req.Metadata); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to update: %v"}`, err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"id": req.ID, "status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
