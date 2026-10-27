package ui

import (
	"testing"

	"memdoor/pkg/shell"
)

// Every key /connect offers is one the agent's commands never see
// (shell.Env): a provider added here without its variable there would leak.
func TestConnectKeysAreHiddenFromCommands(t *testing.T) {
	for _, k := range ConnectKinds {
		if k.KeyVar != "" && !shell.IsModelKey(k.KeyVar) {
			t.Errorf("%s (%s) is not in shell.modelKeys", k.KeyVar, k.Name)
		}
	}
}
