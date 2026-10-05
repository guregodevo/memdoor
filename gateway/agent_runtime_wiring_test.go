package gateway

import (
	"testing"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/providers"
)

// The cut-off escalation asks for twice the cap, but never past the model's
// own output cap when the reference states one.
func TestEscalationStopsAtTheModelsOwnCap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: "t", Endpoint: "http://127.0.0.1:1/v1", APIKey: "k", Model: "m", CtxLen: 131072}); err != nil {
		t.Fatal(err)
	}
	ctxmgmt.SetRemoteWindow(func(string) int { return 131072 })
	ctxmgmt.SetRemoteOutput(func(model string) int {
		if model == "capped" {
			return 20000
		}
		return 0
	})
	t.Cleanup(func() { ctxmgmt.SetRemoteWindow(nil); ctxmgmt.SetRemoteOutput(nil) })
	if got := escalatedMaxOutputTokens("free"); got != 32768 {
		t.Fatalf("no ceiling: twice 16k, got %d", got)
	}
	if got := escalatedMaxOutputTokens("capped"); got != 20000 {
		t.Fatalf("stops at the model's cap, got %d", got)
	}
}
