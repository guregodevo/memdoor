package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/domain"

	"github.com/google/uuid"
)

func TestHandleRooms_GET(t *testing.T) {
	cs := newTestChatServer()

	// Seed 2 channels into the mock repo
	channelRepo := cs.channelRepo.(*mockChannelRepo)
	now := time.Now()
	ch1 := &domain.Channel{
		ID:   uuid.New(),
		Name: "general",
		Timestamps: domain.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	ch2 := &domain.Channel{
		ID:   uuid.New(),
		Name: "random",
		Timestamps: domain.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	channelRepo.channels = append(channelRepo.channels, ch1, ch2)

	// Use system:internal to bypass authz filtering
	req := authenticatedRequest("GET", "/api/channels", nil, "system:internal")
	rr := httptest.NewRecorder()

	cs.handleRooms(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	channels, ok := resp["channels"].([]interface{})
	if !ok {
		t.Fatalf("expected channels array in response")
	}
	if len(channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(channels))
	}
}

func TestHandleRooms_POST(t *testing.T) {
	cs := newTestChatServer()

	body := `{"name": "new-channel", "type": "public"}`
	// Use system:internal to bypass admin check
	req := authenticatedRequest("POST", "/api/channels", strings.NewReader(body), "system:internal")
	rr := httptest.NewRecorder()

	cs.handleRooms(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["name"] != "new-channel" {
		t.Errorf("expected name 'new-channel', got %v", resp["name"])
	}
}

func TestHandleRooms_Unauthenticated(t *testing.T) {
	cs := newTestChatServer()

	// No auth context
	req := httptest.NewRequest("POST", "/api/channels", strings.NewReader(`{"name":"x"}`))
	rr := httptest.NewRecorder()

	cs.handleRooms(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
