package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
	"memdoor/pkg/secrets"
	"memdoor/tools"
)

// A tool's output never carries a secret: the key a command printed is
// redacted in what the model reads, in the record the window and the
// transcript keep, and the words around it stay (live 2026-10-10: `env |
// grep openrouter` and `cat ~/.memdoor/credentials.json` put a key and two
// tokens in a session).
func TestAToolsOutputNeverCarriesASecret(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	ar.tools = append([]tools.ToolDefinition{tools.BashDefinition}, ar.tools...)
	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	// Built at run time: a literal of this shape is a secret to GitHub's
	// push protection, which refused the public push carrying it.
	key := "sk-or-v1-" + strings.Repeat("0123456789abcdef", 4)
	token := "gXN2j5AM3h2HRg39ncJNbm63JfEGdAAn2vzgr70do1s="
	in, _ := json.Marshal(map[string]string{"command": "echo OPEN_ROUTER_API_KEY=" + key + "; echo '{\"token\": \"" + token + "\", \"email\": \"dev@example.com\"}'"})
	info, result := ar.executeTool(context.Background(), &llm.ToolUseBlock{ID: "tu", Name: "bash", Input: in}, "run-1", session)
	if info.Error != "" {
		t.Fatalf("the command runs: %s", info.Error)
	}
	for _, leaked := range []string{key, token} {
		if strings.Contains(info.Output, leaked) {
			t.Fatalf("the record carries the secret: %q", info.Output)
		}
		if strings.Contains(result.OfToolResult.Content[0].OfText.Text, leaked) {
			t.Fatalf("the model reads the secret: %q", result.OfToolResult.Content[0].OfText.Text)
		}
	}
	if !strings.Contains(info.Output, "OPEN_ROUTER_API_KEY="+secrets.Redacted) || !strings.Contains(info.Output, "dev@example.com") {
		t.Fatalf("the name stays and the words around it stay: %q", info.Output)
	}
}
