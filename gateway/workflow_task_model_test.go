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
		// No id: the model that answers here, with the options.
		{"thinking off", "", "", "", "off"},
		{"Throughput thinking low", "", "throughput", "", "low"},
	}
	for _, c := range cases {
		m := parseTaskModel(c.in)
		if m.ID != c.id || m.Sort != c.sort || m.Order != c.order || m.Effort != c.effort {
			t.Errorf("parseTaskModel(%q) = %q %q %q %q, want %q %q %q %q", c.in, m.ID, m.Sort, m.Order, m.Effort, c.id, c.sort, c.order, c.effort)
		}
	}
}

func TestATaskModelLineCapsTheReply(t *testing.T) {
	if n := parseTaskModel("openrouter:qwen/qwen3.8-27b thinking high max_tokens 128000").MaxTokens; n != 128000 {
		t.Errorf("max_tokens = %d", n)
	}
	if n := parseTaskModel("qwen/qwen3.8-27b max_tokens lots").MaxTokens; n != 0 {
		t.Errorf("a non-number cap = %d, want 0", n)
	}
	// The task's number is sent as asked, above or below the gateway's
	// figure for the model; none named, the gateway's figure.
	if got := replyLimit(65536, 128000); got != 128000 {
		t.Errorf("asked above the listed figure: %d", got)
	}
	if got := replyLimit(235929, 16384); got != 16384 {
		t.Errorf("asked below: %d", got)
	}
	if got := replyLimit(65536, 0); got != 65536 {
		t.Errorf("none asked: %d", got)
	}
}

// A task's model line may set the sampling temperature, as GVS5H's harness
// does per role (0.2 to 0.4); a value out of range is ignored.
func TestATaskModelLineSetsTheTemperature(t *testing.T) {
	m := parseTaskModel("openrouter:qwen/qwen3.8-27b thinking high temperature 0.2 max_tokens 128000")
	if m.Temperature != 0.2 || m.MaxTokens != 128000 || m.Effort != "high" || m.ID != "openrouter:qwen/qwen3.8-27b" {
		t.Errorf("got %+v", m)
	}
	if m := parseTaskModel("temperature 0.4"); m.ID != "" || m.Temperature != 0.4 {
		t.Errorf("no id: %+v", m)
	}
	for _, bad := range []string{"qwen temperature hot", "qwen temperature 0", "qwen temperature 7"} {
		if m := parseTaskModel(bad); m.Temperature != 0 {
			t.Errorf("%q: temperature %v, want the default", bad, m.Temperature)
		}
	}
}
