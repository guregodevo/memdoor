package gateway

import (
	"strings"
	"testing"
)

func TestParseToolGuardsSkipsInvalidRuleKeepsRest(t *testing.T) {
	rules := parseToolGuards(`[{"tool":"bash","pattern":"("},{"tool":"bash","pattern":"\\bsudo\\b"}]`)
	if len(rules) != 1 || rules[0].Pattern != `\bsudo\b` {
		t.Fatalf("want the one valid rule, got %+v", rules)
	}
}

func TestGuardBlocksMatchingToolAndPattern(t *testing.T) {
	rules := parseToolGuards(`[{"tool":"bash","pattern":"\\bsudo\\b","message":"no sudo"}]`)
	if msg := guardBlocks(rules, "bash", []byte(`{"command":"sudo rm x"}`)); !strings.Contains(msg, "no sudo") {
		t.Fatalf("want block, got %q", msg)
	}
	if msg := guardBlocks(rules, "bash", []byte(`{"command":"ls"}`)); msg != "" {
		t.Fatalf("clean command must pass, got %q", msg)
	}
	if msg := guardBlocks(rules, "read_file", []byte(`{"path":"sudo"}`)); msg != "" {
		t.Fatalf("other tools must pass a bash-scoped rule, got %q", msg)
	}
}

func TestGuardWildcardToolProtectsPaths(t *testing.T) {
	rules := parseToolGuards(`[{"tool":"*","pattern":"\\.env\\b"}]`)
	for _, tool := range []string{"read_file", "bash", "apply_patch"} {
		if msg := guardBlocks(rules, tool, []byte(`{"path":"cfg/.env"}`)); msg == "" {
			t.Fatalf("wildcard rule must block %s touching .env", tool)
		}
	}
}

func TestEmptyGuardSettingIsNoRules(t *testing.T) {
	if rules := parseToolGuards(""); rules != nil {
		t.Fatalf("empty setting must yield no rules, got %+v", rules)
	}
}
