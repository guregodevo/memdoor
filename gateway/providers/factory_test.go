package providers

import (
	"testing"

	ctxlim "memdoor/gateway/context"
)

// TestModelNameRegisteredInLimits locks the cross-package contract that
// limits.go documents: the model name ClientFactory advertises must be
// registered in the modelLimits map. Otherwise NewPreflightChecker
// fails at agent runtime init and the gateway can't service the agent.
//
// There is a single label (ModelName); an unregistered one once
// hard-failed boot.
func TestModelNameRegisteredInLimits(t *testing.T) {
	if ModelName == "" {
		t.Fatal("ModelName is empty")
	}
	if _, err := ctxlim.GetModelLimits(ModelName); err != nil {
		t.Errorf("ModelName=%q has no entry in modelLimits — "+
			"add it to gateway/context/limits.go or NewPreflightChecker "+
			"will fail at agent runtime init: %v", ModelName, err)
	}
}
