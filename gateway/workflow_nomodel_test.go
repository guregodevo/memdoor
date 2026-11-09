package gateway

import (
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

// With no provider connected, a workflow that has an llm or agent step is
// refused at the start with the one line that says what to set, not run into a
// failing step for each (#215: `memdoor workflow try` on a machine with no
// key). A workflow of commands needs no model and starts.
func TestAWorkflowWithModelStepsAndNoModelIsRefusedAtTheStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	writeWorkflowSteps(t, dir, "needs-model", map[string]string{"plan": "type: llm\nprompt: Plan.\n"})
	s := &Server{clientFactory: providers.NewClientFactory()}
	_, err := s.startWorkflow(dir, "needs-model", "", "", "", "", "", "", 0)
	if err == nil || !strings.Contains(err.Error(), "no model is configured") {
		t.Fatalf("a workflow with an llm step and no model: err = %v", err)
	}
	writeWorkflowSteps(t, dir, "commands-only", map[string]string{"hello": "type: command\ncommand: echo hi\n"})
	run, err := s.startWorkflow(dir, "commands-only", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("a workflow of commands needs no model: %v", err)
	}
	t.Cleanup(run.cancel)
}
