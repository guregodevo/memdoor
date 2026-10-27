package shell

import (
	"os"
	"strings"
)

// modelKeys are the variables the gateway calls models with: the providers
// it reads from the environment (gateway/providers registry.go, vendor.go,
// byok.go) and the decision model's key. ui.ConnectKinds' are among them
// (cmd/tui/ui test).
var modelKeys = map[string]bool{
	"OPEN_ROUTER_API_KEY": true, "OPENROUTER_API_KEY": true,
	"ANTHROPIC_API_KEY": true, "OPENAI_API_KEY": true, "GEMINI_API_KEY": true,
	"BASETEN_API_KEY": true, "GROQ_API_KEY": true, "DEEPSEEK_API_KEY": true,
	"XAI_API_KEY": true, "GROK_API_KEY": true, "TYPESAFE_API_KEY": true,
	"MEMDOOR_SYSTEMONE_API_KEY": true, "AI_GATEWAY_TOKEN": true,
}

// IsModelKey says whether the variable name holds a key the gateway calls
// models with: one of modelKeys, or a company AI gateway's <X>_AI_GATEWAY_TOKEN.
func IsModelKey(name string) bool {
	return modelKeys[name] || strings.HasSuffix(name, "_AI_GATEWAY_TOKEN")
}

// Env is the environment a command runs with: this process's, without the
// model keys. The gateway holds them; nothing it starts for the agent needs
// them, and a command that prints its environment (an `env` while debugging,
// a test that dumps it, a line a pull request planted for the agent) would
// put them in the model's context, a log or a CI job's summary (Greg,
// 2026-10-27: "dont share key"). Defense in depth, not a boundary: a
// process of the same user can still read a parent's starting environment
// (/proc/<pid>/environ on Linux). A project's tests that need one of these
// keys get it under another name.
func Env() []string {
	env := os.Environ()
	out := env[:0:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !IsModelKey(name) {
			out = append(out, kv)
		}
	}
	return out
}
