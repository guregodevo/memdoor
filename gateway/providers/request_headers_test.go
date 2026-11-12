package providers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every request this code builds to a model host names the app (SetAppHeaders,
// AppHeaders, or pkg/attribution directly): OpenRouter ranks an app by the
// tokens that arrive with its headers, and a request without them happened
// and did not count (memdoor#173). The test reads the source: each function
// that builds an http.Request in the packages that talk to a model host must
// apply the headers in that same function.
func TestEveryRequestToAModelHostNamesTheApp(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// The packages whose requests reach a model host or OpenRouter's API.
	scopes := []string{"gateway/providers", "pkg/decision/systemone", "tools", "eval/editbench", "gateway/byok_catalog.go"}
	var files []string
	for _, s := range scopes {
		p := filepath.Join(root, s)
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if !st.IsDir() {
			files = append(files, p)
			continue
		}
		m, _ := filepath.Glob(filepath.Join(p, "*.go"))
		for _, f := range m {
			if !strings.HasSuffix(f, "_test.go") {
				files = append(files, f)
			}
		}
	}
	// Requests in these packages that go nowhere near a model host: the app's
	// own reference document on memdoor.ai, the gateway's own API from a tool,
	// and a page the person asked for. Named here so a new one is a decision.
	notAHost := map[string]bool{
		"gateway/providers/reference.go: referenceDoc": true,
		"tools/core_tools.go: agentsAPICall":           true,
		"tools/web_tools.go: WebFetch":                 true,
	}
	fset := token.NewFileSet()
	var missing []string
	for _, f := range files {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			builds, names := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch sel := call.Fun.(type) {
				case *ast.SelectorExpr:
					name := sel.Sel.Name
					pkg, _ := sel.X.(*ast.Ident)
					if pkg != nil && pkg.Name == "http" && (name == "NewRequest" || name == "NewRequestWithContext") {
						builds = true
					}
					if pkg != nil && pkg.Name == "attribution" && (name == "SetFor" || name == "Set") {
						names = true
					}
					if name == "SetAppHeaders" || name == "AppHeaders" || name == "setAttributionHeaders" {
						names = true
					}
				case *ast.Ident:
					if sel.Name == "SetAppHeaders" || sel.Name == "AppHeaders" || sel.Name == "setAttributionHeaders" {
						names = true
					}
				}
				return true
			})
			// A request built from a headers map the caller filled (tools.WebSearchBackend.Headers,
			// from AppHeaders) counts: the map is applied in a range over Headers.
			if builds && !names {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if r, ok := n.(*ast.RangeStmt); ok {
						if sel, ok := r.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Headers" {
							names = true
						}
					}
					return true
				})
			}
			if builds && !names {
				rel, _ := filepath.Rel(root, f)
				rel = filepath.ToSlash(rel)
				if !notAHost[rel+": "+fn.Name.Name] {
					missing = append(missing, rel+": "+fn.Name.Name)
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("requests built without the app's headers (SetAppHeaders, AppHeaders or pkg/attribution in the same function):\n  %s", strings.Join(missing, "\n  "))
	}
}
