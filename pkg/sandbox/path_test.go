package sandbox

import (
	"errors"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var (
	testUser  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testChan  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	testWksID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

func ctxWithScope(scope SandboxScope) SandboxContext {
	return SandboxContext{
		WorkspaceID:      testWksID,
		ChannelID:        testChan,
		InitiatingUserID: testUser,
		CurrentAgentID:   "agent:writer",
		AgentScope:       scope,
	}
}

func TestCanAccessPath_ScopeBoundaries(t *testing.T) {
	userP := "/user/" + testUser.String() + "/notes.txt"
	chanP := "/channel/" + testChan.String() + "/shared.txt"
	wksP := "/workspace/" + testWksID.String() + "/policy.txt"
	otherUserP := "/user/" + uuid.New().String() + "/secret.txt"

	cases := []struct {
		name  string
		scope SandboxScope
		path  string
		want  bool
	}{
		{"user can access own", ScopeUser, userP, true},
		{"user cannot access channel", ScopeUser, chanP, false},
		{"user cannot access workspace", ScopeUser, wksP, false},
		{"user cannot access other user", ScopeUser, otherUserP, false},

		{"channel can access channel", ScopeChannel, chanP, true},
		{"channel can access own user", ScopeChannel, userP, true},
		{"channel cannot access workspace", ScopeChannel, wksP, false},
		{"channel cannot access other user", ScopeChannel, otherUserP, false},

		{"workspace can access workspace", ScopeWorkspace, wksP, true},
		{"workspace can access channel", ScopeWorkspace, chanP, true},
		{"workspace can access own user", ScopeWorkspace, userP, true},
		{"workspace cannot access other user", ScopeWorkspace, otherUserP, false},

		{"invalid scope denies all", SandboxScope("bogus"), userP, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ctxWithScope(tc.scope)
			if got := ctx.CanAccessPath(tc.path); got != tc.want {
				t.Errorf("CanAccessPath(%q) under %s = %v, want %v", tc.path, tc.scope, got, tc.want)
			}
		})
	}
}

// TestCanAccessPath_TraversalEscapesDenied is the security regression test:
// `..` segments must not let an agent slip past its scope prefix.
func TestCanAccessPath_TraversalEscapesDenied(t *testing.T) {
	ctx := ctxWithScope(ScopeUser)
	base := "/user/" + testUser.String()

	escapes := []string{
		base + "/../../etc/passwd",
		base + "/../../../../../../etc/cron.d/evil",
		base + "/sub/../../" + uuid.New().String() + "/steal.txt", // hop to another user
		base + "/../channel/" + testChan.String() + "/x.txt",      // hop to a forbidden scope
	}
	for _, p := range escapes {
		if ctx.CanAccessPath(p) {
			t.Errorf("traversal path was allowed but must be denied: %q (cleans to %q)", p, filepath.Clean(p))
		}
	}

	// A `..` that stays within scope is fine.
	ok := base + "/sub/../notes.txt"
	if !ctx.CanAccessPath(ok) {
		t.Errorf("in-scope path with .. was denied: %q", ok)
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		scope SandboxScope
		in    string
		want  string
	}{
		{ScopeUser, "notes.txt", "/user/" + testUser.String() + "/notes.txt"},
		{ScopeChannel, "shared.txt", "/channel/" + testChan.String() + "/shared.txt"},
		{ScopeWorkspace, "policy.txt", "/workspace/" + testWksID.String() + "/policy.txt"},
		{ScopeUser, "/user/x/already-virtual.txt", "/user/x/already-virtual.txt"},
	}
	for _, tc := range cases {
		ctx := ctxWithScope(tc.scope)
		if got := ctx.NormalizePath(tc.in); got != tc.want {
			t.Errorf("NormalizePath(%q) under %s = %q, want %q", tc.in, tc.scope, got, tc.want)
		}
	}
}

// A relative traversal path must be rejected once normalized + resolved — this
// is the realistic agent-tool attack: write_file with path "../../../etc/x".
func TestResolvePath_RelativeTraversalDenied(t *testing.T) {
	ctx := ctxWithScope(ScopeUser)
	virtual := ctx.NormalizePath("../../../../../../etc/cron.d/evil")
	if _, err := ctx.ResolvePath(virtual); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied for traversal, got %v", err)
	}
}

func TestResolvePath_ValidStaysInsideSandbox(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	sandboxBase := filepath.Join(home, shared.MemdoorDirName, "sandbox")

	ctx := ctxWithScope(ScopeUser)
	virtual := ctx.NormalizePath("project/notes.txt")
	real, err := ctx.ResolvePath(virtual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(real, sandboxBase+string(os.PathSeparator)) {
		t.Errorf("resolved path %q is not under sandbox base %q", real, sandboxBase)
	}
}

func TestScopeAllows(t *testing.T) {
	if !ScopeWorkspace.Allows(ScopeUser) {
		t.Error("workspace should allow user")
	}
	if ScopeUser.Allows(ScopeWorkspace) {
		t.Error("user must not allow workspace")
	}
	if !ScopeChannel.Allows(ScopeChannel) {
		t.Error("channel should allow channel")
	}
}
