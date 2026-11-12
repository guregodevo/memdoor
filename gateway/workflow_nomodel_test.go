package gateway

import (
	"memdoor/internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/providers"
)

func writeWorkflowSteps(t *testing.T, dir, name string, steps map[string]string) {
	t.Helper()
	d := filepath.Join(dir, ".memdoor", "workflows", name, "steps")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, yaml := range steps {
		if err := os.WriteFile(filepath.Join(d, n+".yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// With no provider connected, a run that asks for a model is refused at the
// start with the one line that says what to set, not run into a failing step
// for each (#215: `memdoor workflow try` on a machine with no key). A workflow
// that did not ask still starts: fix-tests on a project with nothing to fix
// runs no model, and the action's self-test runs it with no key.
func TestARunThatNeedsAModelIsRefusedWithNoModel(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	for _, k := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	s := runServer(t, &Server{clientFactory: providers.NewClientFactory()})
	if err := s.requireModel(); err == nil || !strings.Contains(err.Error(), "no model is configured") {
		t.Fatalf("requireModel with no provider: err = %v", err)
	}
	dir := t.TempDir()
	writeWorkflowSteps(t, dir, "commands-only", map[string]string{"hello": "type: command\ncommand: echo hi\n"})
	run, err := s.startWorkflow(dir, "commands-only", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("a workflow that did not ask for a model starts: %v", err)
	}
	t.Cleanup(run.cancel)
}
