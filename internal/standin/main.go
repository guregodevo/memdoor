// Command standin is a model provider for tests: an OpenAI-compatible chat
// endpoint on 127.0.0.1 that answers one fixed reply, and only to the key it
// was started with. A first-session check (scripts/first-session.sh) connects
// memdoor to it the ways a newcomer connects a real provider, and the reply
// on screen proves the key reached it. Nothing leaves the host and no key is
// spent.
//
//	go build -o standin ./internal/standin
//	STANDIN_KEY=sk-… ./standin -port 18961
//
// It serves GET /v1/models and POST /v1/chat/completions (streamed or not).
// Every other path is a 404, so memdoor connect's probe falls back from the
// Responses shape to chat. Each request is logged on stderr with whether its
// key was the right one.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// Model is the one model the stand-in lists.
	Model = "standin-1"
	// Reply is what every chat request is answered with; a check looks for it.
	Reply = "Hello from the stand-in provider."
)

func main() {
	port := flag.Int("port", 18961, "the port on 127.0.0.1")
	flag.Parse()
	key := os.Getenv("STANDIN_KEY")
	if key == "" {
		log.Fatal("STANDIN_KEY is not set: the stand-in answers only to that key")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("stand-in provider on http://%s/v1", ln.Addr())
	log.Fatal(http.Serve(ln, handler(key)))
}

func handler(key string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"object": "list",
			"data": []map[string]any{{
				"id": Model, "object": "model", "context_length": 32768,
				"pricing": map[string]string{"prompt": "0", "completion": "0"},
			}},
		})
	})
	mux.HandleFunc("POST /v1/chat/completions", chat)
	return logged(key, mux)
}

// logged refuses a request without the right key, as a provider does, and
// logs each one.
func logged(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok := r.Header.Get("Authorization") == "Bearer "+key
		log.Printf("%s %s key=%s", r.Method, r.URL.Path, map[bool]string{true: "ok", false: "refused"}[ok])
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "invalid api key", "code": 401}})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Stream bool `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	id := fmt.Sprintf("chatcmpl-standin-%d", time.Now().UnixNano())
	usage := map[string]int{"prompt_tokens": 10, "completion_tokens": 8, "total_tokens": 18}
	if !req.Stream {
		writeJSON(w, http.StatusOK, map[string]any{
			"id": id, "object": "chat.completion", "created": time.Now().Unix(), "model": Model,
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop",
				"message": map[string]string{"role": "assistant", "content": Reply}}},
			"usage": usage,
		})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	chunk := func(delta map[string]string, finish any, u any) {
		c := map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": Model,
			"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}}
		if u != nil {
			c["usage"] = u
		}
		b, _ := json.Marshal(c)
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	chunk(map[string]string{"role": "assistant", "content": ""}, nil, nil)
	chunk(map[string]string{"content": Reply}, nil, nil)
	chunk(map[string]string{}, "stop", usage)
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
