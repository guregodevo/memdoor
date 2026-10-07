package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/llm"
	"memdoor/tools"
)

// Only bash running apply_patch as a command is told it is a tool. A line
// starting "apply_patch" inside a heredoc is text: live 2026-09-30 the old
// text check refused `cat > README.md <<EOF` for a line reading
// "apply_patch calls and failures".
func TestBashIsToldApplyPatchIsAToolOnlyWhenItRanIt(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	ar.tools = append([]tools.ToolDefinition{tools.BashDefinition}, ar.tools...)
	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	run := func(command string) ToolExecutionInfo {
		in, _ := json.Marshal(map[string]string{"command": command})
		info, _ := ar.executeTool(context.Background(),
			&llm.ToolUseBlock{ID: "tu", Name: "bash", Input: in}, "run-1", session)
		return info
	}

	readme := filepath.Join(t.TempDir(), "README.md")
	info := run("cat > " + readme + " <<'EOF'\nPer run: pass, output tokens,\napply_patch calls and failures.\nEOF")
	if info.Error != "" {
		t.Fatalf("a heredoc mentioning apply_patch must run: %s", info.Error)
	}
	if b, _ := os.ReadFile(readme); !strings.Contains(string(b), "apply_patch calls") {
		t.Fatalf("the file was not written: %q", b)
	}

	info = run("echo start\napply_patch \"*** Begin Patch\"")
	if !strings.Contains(info.Error, "apply_patch is a TOOL") {
		t.Fatalf("running apply_patch in bash must be explained: error=%q output=%q", info.Error, info.Output)
	}
}
