package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"memdoor/gateway/providers"
)

// Two providers list the same id: /model's one row for it is the answering
// provider's, so a pick pins the one that answers (live 2026-10-31: the row
// was the first in the registry's order, the OpenAI key's, and the pick
// billed the API while the plan served the same model; the mutation check:
// iterate providers.Providers() and the row is a's).
func TestTheModelListKeepsTheAnsweringProvidersRow(t *testing.T) {
	keptKeyGateway(t, false, true, freeCatalog, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "shared-model"}}})
	}))
	defer srv.Close()
	body := `{"providers":[{"id":"a","name":"A","api":"chat","base":"` + srv.URL + `/v1","key":"key-a"},{"id":"b","name":"B","api":"chat","base":"` + srv.URL + `/v1","key":"key-b"}]}`
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".memdoor", "providers.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	b, ok := providers.FindProvider("b")
	if !ok {
		t.Fatal("provider b")
	}
	if err := providers.SetRemoteEngine(b.Engine("shared-model")); err != nil {
		t.Fatal(err)
	}
	out, ok := providerModelsResponse(context.Background(), "", 50)
	if !ok {
		t.Fatal("connected providers answer")
	}
	rows := 0
	for _, m := range out["models"].([]providers.Model) {
		if m.ID == "shared-model" {
			rows++
			if m.Provider != "b" {
				t.Fatalf("the row is %q's, want the answering b's", m.Provider)
			}
		}
	}
	if rows != 1 {
		t.Fatalf("one row for the shared id, got %d", rows)
	}
}
