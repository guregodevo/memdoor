package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A local runtime's model list keeps the runtime's order (the loaded model
// first); a keyed provider's is sorted by id as before.
func TestALocalRuntimesModelOrderIsKept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"Qwen3.8-27B-4bit"},{"id":"Qwen3-8B-4bit"}]}`))
	}))
	defer srv.Close()
	local, err := openAIShapeReader{}.ListModels(context.Background(), Provider{ID: "local", API: APIChat, Base: srv.URL + "/v1", Keyless: true})
	if err != nil || len(local) != 2 || local[0].ID != "Qwen3.8-27B-4bit" {
		t.Fatalf("local runtime: %v %v", local, err)
	}
	keyed, err := openAIShapeReader{}.ListModels(context.Background(), Provider{ID: "host", API: APIChat, Base: srv.URL + "/v1", Key: "k"})
	if err != nil || len(keyed) != 2 || keyed[0].ID != "Qwen3-8B-4bit" {
		t.Fatalf("keyed provider: %v %v", keyed, err)
	}
}
