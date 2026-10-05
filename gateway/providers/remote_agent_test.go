package providers

import "testing"

func TestModelForAgent(t *testing.T) {
	re := &RemoteEngine{Model: "qwen3.8-omni-flash", AgentModels: map[string]string{"coder": "deepseek/deepseek-v4.1-flash"}}
	if re.ModelFor("coder") != "deepseek/deepseek-v4.1-flash" || re.ModelFor("Coder") != "deepseek/deepseek-v4.1-flash" {
		t.Fatal("the coder's own model")
	}
	if re.ModelFor("chief") != "qwen3.8-omni-flash" || re.ModelFor("") != "qwen3.8-omni-flash" {
		t.Fatal("everyone else: the engine's model")
	}
	if (&RemoteEngine{Model: "m"}).ModelFor("coder") != "m" {
		t.Fatal("no agent models: the engine's model")
	}
}
