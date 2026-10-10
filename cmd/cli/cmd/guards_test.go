package cmd

import (
	"strings"
	"testing"
)

// The setting round-trips through the command's shape, a bad regexp is
// refused by number (the gateway would drop it silently), and no rules is
// the empty value that clears the setting.
func TestGuardsSettingRoundTrip(t *testing.T) {
	rules := []guardRule{{Tool: "bash", Pattern: `\bgit push\b`, Message: "pushing is mine"}, {Pattern: `\.env\b`}}
	raw, err := guardsSetting(rules)
	if err != nil {
		t.Fatal(err)
	}
	back, err := guardRulesFrom(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 || back[0].Message != "pushing is mine" || back[1].Tool != "*" {
		t.Fatalf("round trip lost something: %+v", back)
	}
	if _, err := guardsSetting([]guardRule{{Pattern: `\bok\b`}, {Pattern: `(`}}); err == nil || !strings.Contains(err.Error(), "rule 2") {
		t.Fatalf("a bad regexp must be refused by its number, got %v", err)
	}
	if raw, _ := guardsSetting(nil); raw != "" {
		t.Fatalf("no rules must clear the setting, got %q", raw)
	}
	if got := formatGuard(1, rules[0]); !strings.Contains(got, "bash") || !strings.Contains(got, "pushing is mine") {
		t.Fatalf("the listing must show the tool and the message: %s", got)
	}
}
