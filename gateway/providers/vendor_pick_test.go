package providers

import "testing"

// Which environment leaves a vendor key with no model to pick for (#206).
func TestVendorKeyWithoutModel(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{"no key", map[string]string{}, ""},
		{"a Groq key alone", map[string]string{"GROQ_API_KEY": "gsk_x"}, "groq"},
		{"Grok's alias", map[string]string{"GROK_API_KEY": "xai-x"}, "xai"},
		{"the order of vendors", map[string]string{"GROQ_API_KEY": "gsk_x", "OPENAI_API_KEY": "sk-x"}, "openai"},
		{"Vercel's key, read when the table is", map[string]string{"AI_GATEWAY_API_KEY": "vck_x"}, "vercel"},
		{"MEMDOOR_MODEL names it", map[string]string{"GROQ_API_KEY": "gsk_x", "MEMDOOR_MODEL": "m"}, ""},
		{"a company gateway", map[string]string{"GROQ_API_KEY": "gsk_x", "AI_GATEWAY_BASE_URL": "https://gw.example/ai"}, ""},
		{"Anthropic has a default", map[string]string{"GROQ_API_KEY": "gsk_x", "ANTHROPIC_API_KEY": "sk-ant-x"}, ""},
		{"vendor mode off", map[string]string{"GROQ_API_KEY": "gsk_x", "MEMDOOR_VENDOR": "off"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearProviderEnv(t)
			for _, v := range []string{"MEMDOOR_VENDOR", "LLMGATEWAY_API_KEY", "AI_GATEWAY_API_KEY", "VERCEL_API_KEY"} {
				t.Setenv(v, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := VendorKeyWithoutModel(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// VendorEngineOn runs the vendor key on the picked model; MEMDOOR_MODEL wins.
func TestVendorEngineOnThePickedModel(t *testing.T) {
	clearProviderEnv(t)
	for _, v := range []string{"MEMDOOR_VENDOR", "MEMDOOR_MODEL_CONTEXT", "GROQ_BASE_URL", "LLMGATEWAY_API_KEY"} {
		t.Setenv(v, "")
	}
	t.Setenv("GROQ_API_KEY", "gsk_x")
	if _, ok, reason := VendorEngine(); ok || reason == "" {
		t.Fatalf("no model named or picked: ok=%v reason=%q", ok, reason)
	}
	re, ok, _ := VendorEngineOn("openai/gpt-oss-20b")
	if !ok || re.Vendor != VendorGroq || re.Model != "openai/gpt-oss-20b" || re.APIKey != "gsk_x" || re.Endpoint != groqDefaultBase {
		t.Fatalf("the picked model on the Groq key: %+v", re)
	}
	t.Setenv("MEMDOOR_MODEL", "llama-named")
	if re, _, _ := VendorEngineOn("openai/gpt-oss-20b"); re.Model != "llama-named" {
		t.Fatalf("MEMDOOR_MODEL wins: %+v", re)
	}
}
