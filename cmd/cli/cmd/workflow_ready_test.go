package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The first command a stranger types after install can be any workflow
// command, and each one sets the machine up as `workflow run` does instead of
// answering "gateway unreachable" or "Authentication required" (#217).
func TestEveryWorkflowCommandSetsTheMachineUpFirst(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "workflow.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"workflowCmd": false, "workflowRunCmd": false, "workflowStatusCmd": false, "workflowStopCmd": false,
		"workflowApproveCmd": false, "workflowChangesCmd": false, "workflowDiffCmd": false, "workflowResumeCmd": false, "workflowHistoryCmd": false}
	callsReady := func(n ast.Node) bool {
		found := false
		ast.Inspect(n, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "firstRunReady" || id.Name == "runWorkflowIn") {
					found = true
				}
			}
			return !found
		})
		return found
	}
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, sp := range g.Specs {
			v, ok := sp.(*ast.ValueSpec)
			if !ok || len(v.Names) != 1 {
				continue
			}
			if _, tracked := want[v.Names[0].Name]; tracked {
				want[v.Names[0].Name] = callsReady(v)
			}
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("%s does not set the machine up first (firstRunReady)", name)
		}
	}
}
