package mcp

import (
	"context"
	"os"
	"testing"
	"time"
)

func fakeManager(t *testing.T) (Manager, Store, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home, proj := t.TempDir(), t.TempDir()
	st := NewFileStore(home)
	if err := st.Add(ScopeUser, proj, "fake", Entry{Command: exe, Env: map[string]string{"MCP_FAKE_SERVER": "1"}}); err != nil {
		t.Fatal(err)
	}
	m := NewManager(Options{Store: st, ConnectWindow: 10 * time.Second})
	t.Cleanup(m.Shutdown)
	return m, st, proj
}

// A configured server connects on first need; its tools are listed and
// called; the panel sees it connected with its tools.
func TestManagerConnectsListsAndCalls(t *testing.T) {
	m, _, proj := fakeManager(t)
	ctx := context.Background()
	got := m.Tools(ctx, proj)
	if len(got) != 1 || got[0].Server != "fake" || len(got[0].Tools) != 2 {
		t.Fatalf("tools %+v", got)
	}
	res, err := m.Call(ctx, proj, "fake", "echo", map[string]interface{}{"text": "hi"})
	if text, terr := ResultText(res); err != nil || terr != nil || text != "hi" {
		t.Fatalf("call: %q %v %v", text, err, terr)
	}
	sts, err := m.Status(ctx, proj)
	if err != nil || len(sts) != 1 || sts[0].State != StateConnected || len(sts[0].Tools) != 2 || sts[0].Info.Name != "fake" || sts[0].Transport != "stdio" {
		t.Fatalf("status %+v, err %v", sts, err)
	}
}

// Off is off; a project's servers wait for trust; Test connects one on
// the person's request even before trust.
func TestManagerOffAndTrust(t *testing.T) {
	m, st, proj := fakeManager(t)
	ctx := context.Background()
	if err := st.SetDisabled(ScopeUser, proj, "fake", true); err != nil {
		t.Fatal(err)
	}
	if got := m.Tools(ctx, proj); len(got) != 0 {
		t.Fatalf("a server turned off must not start: %+v", got)
	}
	if sts, _ := m.Status(ctx, proj); sts[0].State != StateOff {
		t.Fatalf("status %+v", sts)
	}

	exe, _ := os.Executable()
	st.Add(ScopeProject, proj, "repo", Entry{Command: exe, Env: map[string]string{"MCP_FAKE_SERVER": "1"}})
	if got := m.Tools(ctx, proj); len(got) != 0 {
		t.Fatalf("an untrusted project's server must not start: %+v", got)
	}
	if st := m.Test(ctx, proj, "repo"); st.State != StateConnected {
		t.Fatalf("test: %+v", st)
	}
	if err := st.Trust(proj); err != nil {
		t.Fatal(err)
	}
	if got := m.Tools(ctx, proj); len(got) != 1 || got[0].Server != "repo" {
		t.Fatalf("trusted: %+v", got)
	}
}

// A server whose command fails is reported failed with the reason, and
// left alone until its retry delay or a Test.
func TestManagerFailureIsReported(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	st := NewFileStore(home)
	st.Add(ScopeUser, proj, "gone", Entry{Command: "no-such-mcp-server-binary"})
	m := NewManager(Options{Store: st})
	defer m.Shutdown()
	if got := m.Tools(context.Background(), proj); len(got) != 0 {
		t.Fatalf("tools %+v", got)
	}
	sts, _ := m.Status(context.Background(), proj)
	if sts[0].State != StateFailed || sts[0].Err == "" {
		t.Fatalf("status %+v", sts)
	}
}

func TestToolNameAndResult(t *testing.T) {
	if n := ToolName("github", "create_issue"); n != "mcp__github__create_issue" {
		t.Fatalf("%q", n)
	}
	long := ToolName("server", "a.very.long.tool.name.with.dots.that.goes.on.and.on.and.on.forever")
	if len(long) > 64 || long[:13] != "mcp__server__" {
		t.Fatalf("%q (%d)", long, len(long))
	}
	if _, err := ResultText(&ToolResult{IsError: true, Content: []Content{{Type: "text", Text: "rate limited"}}}); err == nil || err.Error() != "rate limited" {
		t.Fatalf("an error result must be an error: %v", err)
	}
	text, _ := ResultText(&ToolResult{Content: []Content{{Type: "image", MimeType: "image/png", Data: "AAAA"}}, StructuredContent: []byte(`{"a":1}`)})
	if text != "[image: image/png, 0 KB, not shown]" {
		t.Fatalf("%q", text)
	}
}
