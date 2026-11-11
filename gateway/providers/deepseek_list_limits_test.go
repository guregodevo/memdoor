package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func deepseekList(t *testing.T, body string) Model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	ms, err := deepseekReader{}.ListModels(context.Background(), Provider{ID: "deepseek", Base: srv.URL, Key: "k"})
	if err != nil || len(ms) != 1 {
		t.Fatalf("list: %v %v", ms, err)
	}
	return ms[0]
}

// A relay serving DeepSeek states the window and the reply cap in its own
// shape; each is read, and DeepSeek's own fields still win.
func TestADeepSeekListInARelaysShapeIsRead(t *testing.T) {
	for name, c := range map[string]struct {
		body            string
		context, output int
		inputs          int
	}{
		"models.dev limit": {`{"data":[{"id":"m","limit":{"context":1000000,"output":384000}}]}`, 1000000, 384000, 0},
		"openrouter": {`{"data":[{"id":"m","context_length":1000000,"top_provider":{"context_length":900000,"max_completion_tokens":384000},"architecture":{"input_modalities":["text","image"]}}]}`,
			1000000, 384000, 2},
		"openrouter top only": {`{"data":[{"id":"m","top_provider":{"context_length":900000}}]}`, 900000, 0, 0},
		"deepseek's own wins": {`{"data":[{"id":"m","context_window":128000,"max_output_tokens":8000,"limit":{"context":1000000,"output":384000}}]}`, 128000, 8000, 0},
	} {
		m := deepseekList(t, c.body)
		if m.Context != c.context || m.MaxOutput != c.output || len(m.Inputs) != c.inputs {
			t.Errorf("%s: window %d, output %d, inputs %v", name, m.Context, m.MaxOutput, m.Inputs)
		}
	}
}
