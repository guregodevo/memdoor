package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeSSE is a server on the old transport: POST to its URL is refused
// (405), a GET opens the stream, whose first event names where to POST,
// and every answer comes back on the stream.
func fakeSSE(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	var stream chan string
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		ch := make(chan string, 8)
		mu.Lock()
		stream = ch
		mu.Unlock()
		fmt.Fprint(w, "event: endpoint\ndata: /messages?sessionId=s1\n\n")
		fl.Flush()
		for {
			select {
			case msg := <-ch:
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				fl.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sessionId") != "s1" {
			http.Error(w, "no session", 400)
			return
		}
		var m message
		json.NewDecoder(r.Body).Decode(&m)
		w.WriteHeader(http.StatusAccepted)
		mu.Lock()
		ch := stream
		mu.Unlock()
		switch m.Method {
		case "initialize":
			ch <- fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"legacy"},"capabilities":{"prompts":{}}}}`, m.ID)
		case "tools/list":
			ch <- `{"jsonrpc":"2.0","method":"notifications/message","params":{}}`
			ch <- fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"tools":[{"name":"old_tool"}]}}`, m.ID)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// A URL given as HTTP that turns out to be the old transport works: the
// refused POST falls back to the stream, as the spec says.
func TestHTTPFallsBackToSSE(t *testing.T) {
	srv := fakeSSE(t)
	c := NewHTTP(Spec{Name: "legacy", URL: srv.URL + "/sse"}, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	if c.ServerInfo().Name != "legacy" || !c.ServerInfo().Prompts {
		t.Fatalf("info %+v", c.ServerInfo())
	}
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "old_tool" {
		t.Fatalf("tools %+v err %v", tools, err)
	}
}

// "type": "sse" in the config goes straight to the stream.
func TestConfiguredSSE(t *testing.T) {
	srv := fakeSSE(t)
	c, err := New(Spec{Name: "legacy", URL: srv.URL + "/sse", SSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	if tools, err := c.ListTools(context.Background()); err != nil || len(tools) != 1 {
		t.Fatalf("tools %+v err %v", tools, err)
	}
}
