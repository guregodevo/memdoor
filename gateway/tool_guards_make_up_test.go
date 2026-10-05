package gateway

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// scripts/tool_guards_make_up.sh is the guard that stops the coder restarting
// the gateway it runs on (it ran `make up` twice on 2026-09-30 and lost both
// turns). Its rules, through the gateway's own matcher: every target that
// stops the gateway is refused, building and testing are not.
func TestMakeUpGuardRules(t *testing.T) {
	out, err := exec.Command("bash", "../scripts/tool_guards_make_up.sh").Output()
	if err != nil {
		t.Skipf("script needs bash and python3: %v", err)
	}
	var setting struct {
		ToolGuards string `json:"tool_guards"`
	}
	if err := json.Unmarshal(out, &setting); err != nil {
		t.Fatal(err)
	}
	rules := parseToolGuards(setting.ToolGuards)
	if len(rules) != 3 {
		t.Fatalf("%d of 3 rules compiled", len(rules))
	}
	for cmd, blocked := range map[string]bool{
		"make up": true, "make up 2>&1 | tail": true, "make clean stop start": true,
		"make stop-gateway": true, "make gateway-verbose": true, "make -j4 up": true,
		"go build ./... && make up": true, "cd /x; make up": true, "make build\nmake up": true,
		"(make up)": true, "pkill -f memdoor": true, "lsof -ti:18789 | xargs kill -9": true,

		"make build": false, "make -C gateway build": false, "make test": false,
		"go test ./...": false, "cd gateway && make build": false, "make build-gateway": false,
		"make build && git log --grep up": false, "echo make it up": false,
		"grep -n 'make up' AGENTS.md": false,
	} {
		escaped, _ := json.Marshal(map[string]string{"command": cmd}) // & as &
		raw := strings.ReplaceAll(string(escaped), `&`, "&")
		for _, in := range []string{string(escaped), raw} {
			if got := guardBlocks(rules, "bash", []byte(in)) != ""; got != blocked {
				t.Errorf("%s: blocked=%v, want %v", in, got, blocked)
			}
		}
	}
}
