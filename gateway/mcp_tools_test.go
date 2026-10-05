package gateway

import (
	"context"
	"strings"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

func mcpTurnCtx(palette []string) context.Context {
	ctx := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, palette)
	return context.WithValue(ctx, ctxKeyMCPTurn{}, mcpTurn{
		defs:    []tools.ToolDefinition{{Name: "mcp__linear__create_issue"}, {Name: "mcp__github__search"}},
		servers: []string{"linear", "github"},
	})
}

// An agent gets the turn's MCP tools only when its palette carries "mcp".
func TestMCPToolsPassThePaletteOnlyWithTheEntry(t *testing.T) {
	ar := &AgentRuntime{tools: []tools.ToolDefinition{{Name: "bash"}, {Name: "read_file"}}}
	names := func(defs []tools.ToolDefinition) string {
		var n []string
		for _, d := range defs {
			n = append(n, d.Name)
		}
		return strings.Join(n, ",")
	}
	if got := names(ar.filteredToolsFor(mcpTurnCtx([]string{"bash", "mcp"}))); got != "bash,mcp__linear__create_issue,mcp__github__search" {
		t.Fatalf("with the entry: %s", got)
	}
	if got := names(ar.filteredToolsFor(mcpTurnCtx([]string{"bash", "read_file"}))); got != "bash,read_file" {
		t.Fatalf("without it: %s", got)
	}
}

// A routed turn keeps the MCP tools through the "mcp__*" entry.
func TestRoutedTurnKeepsMCPToolsByWildcard(t *testing.T) {
	all := []tools.ToolDefinition{{Name: "bash"}, {Name: "grep"}, {Name: "mcp__linear__create_issue"}}
	ctx := withTurnToolsAllow(context.Background(), []string{"bash", "mcp__*"})
	var got []string
	for _, d := range submittedTools(ctx, all) {
		got = append(got, d.Name)
	}
	if strings.Join(got, ",") != "bash,mcp__linear__create_issue" {
		t.Fatalf("submitted %v", got)
	}
}

// The "mcp" family exists only on a turn with MCP tools, and names the
// servers so the decision model knows what they are for.
func TestMCPFamilyOnlyWithServers(t *testing.T) {
	base := builtinToolRouting["coder"]
	if _, ok := withMCPFamily(context.Background(), base).Families[mcpRoutingFamily]; ok {
		t.Fatal("no servers, no family")
	}
	fam, ok := withMCPFamily(mcpTurnCtx([]string{"mcp"}), base).Families[mcpRoutingFamily]
	if !ok || !strings.Contains(fam.Description, "github, linear") || fam.Tools[0] != "mcp__*" {
		t.Fatalf("family %+v", fam)
	}
	if _, leaked := builtinToolRouting["coder"].Families[mcpRoutingFamily]; leaked {
		t.Fatal("the built-in config must not be changed")
	}
}
