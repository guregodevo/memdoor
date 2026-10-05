package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	"memdoor/pkg/mcp"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// MCP servers the person connected (pkg/mcp): one manager per gateway, and
// for an agent whose palette carries "mcp", the servers' tools on every
// turn, named mcp__<server>__<tool>.

// mcpPaletteEntry in an agent's palette gives it the person's MCP tools.
const mcpPaletteEntry = "mcp"

// mcpRoutingFamily is the tool family that holds them when a decision model
// routes the turn's tools.
const mcpRoutingFamily = "mcp"

var (
	mcpOnce  sync.Once
	mcpMgr   mcp.Manager
	mcpAuth  *mcp.OAuth
	mcpStore mcp.Store
)

// mcpManager is the gateway's manager, made on first use. An HTTP server
// signs in with OAuth unless its config sends its own Authorization header
// (an API key).
func mcpManager() mcp.Manager {
	mcpOnce.Do(func() {
		dir := shared.MemdoorHome()
		mcpStore = mcp.NewFileStore(dir)
		mcpAuth = mcp.NewOAuth(mcp.NewFileTokenStore(dir))
		mcpMgr = mcp.NewManager(mcp.Options{
			Store:  mcpStore,
			Logger: logs.New("MCP"),
			TokenFor: func(_ mcp.Server, sp mcp.Spec) func(context.Context) (string, error) {
				for k := range sp.Headers {
					if strings.EqualFold(k, "Authorization") {
						return nil
					}
				}
				return mcpAuth.TokenSource(sp.URL)
			},
		})
	})
	return mcpMgr
}

type ctxKeyMCPTurn struct{}

// mcpTurn is the turn's MCP tools and the servers they come from.
type mcpTurn struct {
	defs    []tools.ToolDefinition
	servers []string
}

// paletteHasMCP reports whether the turn's agent was given MCP tools.
func paletteHasMCP(ctx context.Context) bool {
	palette, _ := ctx.Value(sharedctx.BuddyToolsKey).([]string)
	for _, n := range palette {
		if n == mcpPaletteEntry {
			return true
		}
	}
	return false
}

// withMCPTools puts the turn's MCP tools on ctx: the connected servers of
// the project the turn works in. Each tool is a closure over ctx, so a call
// ends with the turn.
func withMCPTools(ctx context.Context) context.Context {
	if !paletteHasMCP(ctx) {
		return ctx
	}
	dir := turnWorkdir(ctx)
	mgr := mcpManager()
	var turn mcpTurn
	var withResources []string
	for _, st := range mgr.Tools(ctx, dir) {
		turn.servers = append(turn.servers, st.Server)
		for _, t := range st.Tools {
			turn.defs = append(turn.defs, mcpToolDefinition(ctx, mgr, dir, st.Server, t))
		}
		if st.Info.Resources {
			withResources = append(withResources, st.Server)
		}
	}
	if len(withResources) > 0 {
		turn.defs = append(turn.defs, mcpResourceTools(ctx, mgr, dir, withResources)...)
	}
	if len(turn.defs) == 0 {
		return ctx
	}
	logs.New("MCP").Info("MCP tools for the turn", "servers", strings.Join(turn.servers, ","), "tools", len(turn.defs))
	return context.WithValue(ctx, ctxKeyMCPTurn{}, turn)
}

func mcpToolDefinition(ctx context.Context, mgr mcp.Manager, dir, server string, t mcp.Tool) tools.ToolDefinition {
	props, required := mcp.InputSchema(t)
	desc := strings.TrimSpace(t.Description)
	if r := []rune(desc); len(r) > 1000 {
		desc = string(r[:1000]) + "…"
	}
	return tools.ToolDefinition{
		Name:        mcp.ToolName(server, t.Name),
		Description: "(MCP server " + server + ") " + desc,
		InputSchema: llm.ToolInputSchemaParam{Type: "object", Properties: props, Required: required},
		Function: func(input json.RawMessage) (string, error) {
			var args map[string]interface{}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &args); err != nil {
					return "", err
				}
			}
			res, err := mgr.Call(ctx, dir, server, t.Name, args)
			if err != nil {
				return "", err
			}
			return mcp.ResultText(res)
		},
	}
}

// The resource tools: two for every server that offers resources, as
// Claude Code has them, instead of one per resource. Their names start
// with the MCP prefix, so the palette and routing treat them as MCP tools.
const (
	mcpListResources = mcp.ToolPrefix + "list_resources"
	mcpReadResource  = mcp.ToolPrefix + "read_resource"
)

func mcpResourceTools(ctx context.Context, mgr mcp.Manager, dir string, servers []string) []tools.ToolDefinition {
	names := strings.Join(servers, ", ")
	return []tools.ToolDefinition{
		{
			Name: mcpListResources,
			Description: "List the resources (files, records, pages) the connected MCP servers offer to read: " + names +
				". Read one with " + mcpReadResource + ".",
			InputSchema: llm.ToolInputSchemaParam{Type: "object", Properties: map[string]interface{}{
				"server": map[string]interface{}{"type": "string", "description": "Only this server's (one of: " + names + "); all of them when empty."},
			}},
			Function: func(input json.RawMessage) (string, error) {
				var in struct {
					Server string `json:"server"`
				}
				_ = json.Unmarshal(input, &in)
				list, err := mgr.Resources(ctx, dir, in.Server)
				if err != nil {
					return "", err
				}
				if len(list) == 0 {
					return "No resources.", nil
				}
				var b strings.Builder
				for _, r := range list {
					fmt.Fprintf(&b, "%s  %s  %s", r.Server, r.URI, r.Name)
					if r.Description != "" {
						b.WriteString(" — " + r.Description)
					}
					b.WriteString("\n")
				}
				return strings.TrimSuffix(b.String(), "\n"), nil
			},
		},
		{
			Name:        mcpReadResource,
			Description: "Read one resource from a connected MCP server (" + names + "), by the URI " + mcpListResources + " gave.",
			InputSchema: llm.ToolInputSchemaParam{Type: "object", Properties: map[string]interface{}{
				"server": map[string]interface{}{"type": "string", "description": "The server: one of " + names + "."},
				"uri":    map[string]interface{}{"type": "string", "description": "The resource's URI."},
			}, Required: []string{"server", "uri"}},
			Function: func(input json.RawMessage) (string, error) {
				var in struct {
					Server string `json:"server"`
					URI    string `json:"uri"`
				}
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				contents, err := mgr.ReadResource(ctx, dir, in.Server, in.URI)
				if err != nil {
					return "", err
				}
				parts := make([]mcp.Content, 0, len(contents))
				for i := range contents {
					c := contents[i]
					if c.Text == "" && c.Blob != "" {
						parts = append(parts, mcp.Content{Type: "text", Text: fmt.Sprintf("[%s: %s, %d KB, not shown]", c.URI, c.MimeType, len(c.Blob)*3/4/1024)})
						continue
					}
					parts = append(parts, mcp.Content{Type: "resource", Resource: &c})
				}
				return mcp.ResultText(&mcp.ToolResult{Content: parts})
			},
		},
	}
}

// mcpToolsFrom is the turn's MCP tools.
func mcpToolsFrom(ctx context.Context) []tools.ToolDefinition {
	turn, _ := ctx.Value(ctxKeyMCPTurn{}).(mcpTurn)
	return turn.defs
}

// turnTools is the agent's tools plus the turn's MCP tools.
func (ar *AgentRuntime) turnTools(ctx context.Context) []tools.ToolDefinition {
	extra := mcpToolsFrom(ctx)
	// The cron tool is built per turn because it carries where this turn runs
	// and where its answers belong: a poll scheduled from your project answers
	// in your window.
	if c := ar.turnCronTool(ctx); c != nil {
		extra = append(extra, *c)
	}
	if w := ar.turnWorkflowTool(ctx); w != nil {
		extra = append(extra, *w)
	}
	if len(extra) == 0 {
		return ar.tools
	}
	return append(ar.tools[:len(ar.tools):len(ar.tools)], extra...)
}

// withMCPFamily adds the "mcp" family to a routing config when the turn has
// MCP tools: its description names the servers, so the decision model knows
// what they are for. Without servers there is no family to pick.
func withMCPFamily(ctx context.Context, cfg agentToolRouting) agentToolRouting {
	turn, _ := ctx.Value(ctxKeyMCPTurn{}).(mcpTurn)
	if len(turn.defs) == 0 {
		return cfg
	}
	servers := append([]string(nil), turn.servers...)
	sort.Strings(servers)
	fams := make(map[string]toolFamily, len(cfg.Families)+1)
	for k, v := range cfg.Families {
		fams[k] = v
	}
	fams[mcpRoutingFamily] = toolFamily{
		Description: "Use one of the person's connected MCP servers (" + strings.Join(servers, ", ") + "): their own services, data or accounts.",
		Tools:       []string{mcp.ToolPrefix + "*"},
	}
	cfg.Families = fams
	return cfg
}

// toolAllowed reports whether name is in allow: by name, or by an entry
// ending in "*" that it starts with (mcp__*).
func toolAllowed(allow map[string]bool, name string) bool {
	if allow[name] {
		return true
	}
	for n := range allow {
		if p, ok := strings.CutSuffix(n, "*"); ok && strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
