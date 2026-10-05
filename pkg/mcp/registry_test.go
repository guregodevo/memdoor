package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Fixtures in the registry's own shapes (2026-10-01): a hosted server whose
// header is a template, an npm package with a required variable, an old
// version the search must skip, and an entry nothing here can run.
const registryPage = `{"servers":[
 {"server":{"name":"ai.smithery/obsidian-github-mcp","description":"Obsidian vault","version":"0.4.0",
   "remotes":[{"type":"streamable-http","url":"https://server.smithery.ai/obsidian/mcp",
     "headers":[{"name":"Authorization","value":"Bearer {smithery_api_key}","isRequired":true,"isSecret":true}]}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"isLatest":true}}},
 {"server":{"name":"com.pulsemcp/remote-filesystem","version":"0.1.2",
   "packages":[{"registryType":"npm","identifier":"remote-filesystem-mcp-server","version":"0.1.2","transport":{"type":"stdio"},
     "environmentVariables":[{"name":"GCS_BUCKET","isRequired":true,"description":"bucket"},{"name":"GCS_ROOT_PATH"}]}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"isLatest":true}}},
 {"server":{"name":"com.pulsemcp/remote-filesystem","version":"0.1.0","packages":[{"registryType":"npm","identifier":"old"}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"isLatest":false}}},
 {"server":{"name":"io.example/nuget-only","packages":[{"registryType":"nuget","identifier":"X"}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"isLatest":true}}}
]}`

func TestSearchRegistry(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.RawQuery
		fmt.Fprint(w, registryPage)
	}))
	defer srv.Close()
	prev := RegistryURL
	RegistryURL = srv.URL
	defer func() { RegistryURL = prev }()

	got, err := SearchRegistry(context.Background(), nil, "files", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asked, "search=files") {
		t.Fatalf("query %q", asked)
	}
	// The registry drops the old versions, so a page the size of the limit
	// holds that many servers instead of a tenth as many.
	if !strings.Contains(asked, "version=latest") || !strings.Contains(asked, "limit=10") {
		t.Fatalf("the search must ask for current versions only, and a page the size of the limit: %q", asked)
	}
	if len(got) != 2 {
		t.Fatalf("want the hosted one and the latest npm one, got %+v", got)
	}
	hosted, npm := got[0], got[1]
	if hosted.Via != "hosted" || hosted.Entry.URL != "https://server.smithery.ai/obsidian/mcp" ||
		hosted.Entry.Headers["Authorization"] != "Bearer ${SMITHERY_API_KEY}" || hosted.Suggested != "obsidian-github" {
		t.Fatalf("hosted: %+v", hosted)
	}
	if npm.Via != "npm" || npm.Entry.Command != "npx" || strings.Join(npm.Entry.Args, " ") != "-y remote-filesystem-mcp-server@0.1.2" ||
		npm.Entry.Env["GCS_BUCKET"] != "${GCS_BUCKET}" || len(npm.Entry.Env) != 1 || npm.Needs[0].Name != "GCS_BUCKET" {
		t.Fatalf("npm: %+v", npm)
	}
}
