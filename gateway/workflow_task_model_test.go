package gateway

import "testing"

// A task's model line carries the host preference the way /model does.
func TestATaskModelLineNamesTheHostPreference(t *testing.T) {
	cases := []struct{ in, id, sort, order, effort string }{
		{"openrouter:qwen/qwen3.8-27b", "openrouter:qwen/qwen3.8-27b", "", "", ""},
		{"openrouter:qwen/qwen3.8-27b throughput", "openrouter:qwen/qwen3.8-27b", "throughput", "", ""},
		{"qwen/qwen3.8-27b order cerebras,deepinfra", "qwen/qwen3.8-27b", "", "cerebras,deepinfra", ""},
		{"  qwen/qwen3.8-27b  Price order chutes ", "qwen/qwen3.8-27b", "price", "chutes", ""},
		{"openrouter:qwen/qwen3.8-27b throughput thinking off", "openrouter:qwen/qwen3.8-27b", "throughput", "", "off"},
		{"qwen/qwen3.8-27b effort High", "qwen/qwen3.8-27b", "", "", "high"},
		{"qwen/qwen3.8-27b thinking sideways", "qwen/qwen3.8-27b", "", "", ""},
		{"openrouter:qwen/qwen3.8-27b thinking high order modal max_tokens 128000", "openrouter:qwen/qwen3.8-27b", "", "modal", "high"},
		{"", "", "", "", ""},
	}
	for _, c := range cases {
		id, sort, order, effort, _ := parseTaskModel(c.in)
		if id != c.id || sort != c.sort || order != c.order || effort != c.effort {
			t.Errorf("parseTaskModel(%q) = %q %q %q %q, want %q %q %q %q", c.in, id, sort, order, effort, c.id, c.sort, c.order, c.effort)
		}
	}
}

func TestATaskModelLineCapsTheReply(t *testing.T) {
	if _, _, _, _, n := parseTaskModel("openrouter:qwen/qwen3.8-27b thinking high max_tokens 128000"); n != 128000 {
		t.Errorf("max_tokens = %d", n)
	}
	if _, _, _, _, n := parseTaskModel("qwen/qwen3.8-27b max_tokens lots"); n != 0 {
		t.Errorf("a non-number cap = %d, want 0", n)
	}
	if got := capped(235929, 128000); got != 128000 {
		t.Errorf("capped = %d", got)
	}
	if got := capped(16384, 128000); got != 16384 {
		t.Errorf("a cap above the ceiling must not raise it: %d", got)
	}
}
