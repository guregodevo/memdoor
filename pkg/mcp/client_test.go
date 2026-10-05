package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a stdio MCP server when MCP_FAKE_SERVER=1.
func TestMain(m *testing.M) {
	if os.Getenv("MCP_FAKE_SERVER") == "1" {
		fakeStdioServer(os.Stdin, os.Stdout)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeStdioServer(in io.Reader, out io.Writer) {
	fmt.Fprintln(out, "fake server starting (a log line on stdout, not JSON)")
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var m message
		if json.Unmarshal(sc.Bytes(), &m) != nil || len(m.ID) == 0 {
			continue
		}
		reply := func(result interface{}) {
			b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": m.ID, "result": result})
			fmt.Fprintln(out, string(b))
		}
		switch m.Method {
		case "initialize":
			reply(map[string]interface{}{"protocolVersion": ProtocolVersion,
				"serverInfo": map[string]string{"name": "fake", "version": "0.1"}, "capabilities": map[string]interface{}{}})
		case "tools/list":
			var p struct{ Cursor string }
			_ = json.Unmarshal(m.Params, &p)
			if p.Cursor == "" {
				reply(map[string]interface{}{"tools": []Tool{{Name: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)}}, "nextCursor": "page2"})
			} else {
				reply(map[string]interface{}{"tools": []Tool{{Name: "crash"}}})
			}
		case "tools/call":
			var p struct {
				Name      string
				Arguments map[string]interface{}
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.Name == "crash" {
				fmt.Fprintln(os.Stderr, "fatal: database is locked")
				os.Exit(3)
			}
			fmt.Fprintln(out, `{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info"}}`)
			reply(map[string]interface{}{"content": []Content{{Type: "text", Text: fmt.Sprint(p.Arguments["text"])}}})
		default:
			b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": m.ID, "error": map[string]interface{}{"code": -32601, "message": "nope"}})
			fmt.Fprintln(out, string(b))
		}
	}
}

func fakeStdio(t *testing.T) Client {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := NewStdio(Spec{Name: "fake", Command: exe, Env: map[string]string{"MCP_FAKE_SERVER": "1"}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Stop() })
	return c
}

// The handshake, every page of tools, and a call — past a log line on
// stdout and a notification in the middle of a response.
func TestStdioHandshakeToolsAndCall(t *testing.T) {
	c := fakeStdio(t)
	if c.ServerInfo().Name != "fake" || c.ServerInfo().ProtocolVersion != ProtocolVersion {
		t.Fatalf("server info: %+v", c.ServerInfo())
	}
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 2 || tools[0].Name != "echo" || tools[1].Name != "crash" {
		t.Fatalf("tools %+v, err %v", tools, err)
	}
	res, err := c.CallTool(context.Background(), "echo", map[string]interface{}{"text": "hello"})
	if err != nil || len(res.Content) != 1 || res.Content[0].Text != "hello" {
		t.Fatalf("result %+v, err %v", res, err)
	}
}

// A server that dies says why: its last stderr lines are in the error.
func TestStdioCrashQuotesStderr(t *testing.T) {
	c := fakeStdio(t)
	_, err := c.CallTool(context.Background(), "crash", nil)
	if err == nil || !strings.Contains(err.Error(), "database is locked") {
		t.Fatalf("want the server's stderr in the error, got %v", err)
	}
}

func TestStdioCommandNotFound(t *testing.T) {
	c := NewStdio(Spec{Name: "x", Command: "no-such-mcp-server-binary"}, nil)
	err := c.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "command not found: no-such-mcp-server-binary") {
		t.Fatalf("got %v", err)
	}
}

// Streamable HTTP: the session id from initialize travels on every later
// request with the negotiated protocol version, an event-stream answer is
// read past the server's own ping, and a missing token is a sign-in error.
func TestHTTPSessionStreamAndAuth(t *testing.T) {
	var pinged bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://x/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var m message
		_ = json.NewDecoder(r.Body).Decode(&m)
		if m.Method != "initialize" && m.Method != "" {
			if r.Header.Get("Mcp-Session-Id") != "s1" || r.Header.Get("MCP-Protocol-Version") != ProtocolVersion {
				http.Error(w, "missing session or protocol header", 400)
				return
			}
		}
		switch m.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "s1")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":%q,"serverInfo":{"name":"web","version":"2"}}}`, m.ID, ProtocolVersion)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":99,\"method\":\"ping\"}\n\n")
			fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"tools\":[{\"name\":\"search\"}]}}\n\n", m.ID)
		case "":
			pinged = true // the reply to the server's ping
		}
	}))
	defer srv.Close()

	noTok := NewHTTP(Spec{Name: "web", URL: srv.URL}, nil)
	err := noTok.Start(context.Background())
	var authErr *AuthRequiredError
	if !IsAuthRequired(err) || !errors.As(err, &authErr) || !strings.Contains(authErr.WWWAuthenticate, "resource_metadata") {
		t.Fatalf("want a sign-in error with the challenge, got %v", err)
	}

	c := NewHTTP(Spec{Name: "web", URL: srv.URL, Token: func(context.Context) (string, error) { return "tok", nil }}, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools %+v, err %v", tools, err)
	}
	if !pinged {
		t.Fatal("the server's ping was not answered")
	}
}

func TestNewPicksTheTransport(t *testing.T) {
	if _, err := New(Spec{Name: "a", URL: "https://x", Command: "y"}, nil); err == nil {
		t.Fatal("both a URL and a command must be refused")
	}
	if _, err := New(Spec{Name: "a"}, nil); err == nil {
		t.Fatal("neither must be refused")
	}
}
