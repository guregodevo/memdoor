package gateway

import (
	"context"
	"testing"

	"memdoor/gateway/providers"
	sharedctx "memdoor/pkg/shared/context"
)

// With no engine chosen at start (a provider added with `memdoor connect`
// only), the turn is sized by the model that answers, never by the 32K
// placeholder window.
func TestNoEngineSizesTheTurnByThePin(t *testing.T) {
	_ = providers.ClearRemoteEngine()
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	ar := &AgentRuntime{resolvedModel: providers.ModelName}
	ctx := context.WithValue(context.Background(), sharedctx.ModelKey, "deepseek/deepseek-v4-flash")
	model, re := ar.answeringModel(ctx)
	if re != nil || model != "deepseek/deepseek-v4-flash" {
		t.Fatalf("answeringModel = %q, %v; want the pin and no engine", model, re)
	}
}
