package mcp

import "testing"

func TestServerEnvDropsNodeOptions(t *testing.T) {
	got := withoutNodeOptions([]string{"PATH=/bin", "NODE_OPTIONS=--require /tmp/gone.cjs", "HOME=/x", "NODE_EXTRA_CA_CERTS=/y"})
	if len(got) != 2 || got[0] != "PATH=/bin" || got[1] != "HOME=/x" {
		t.Fatalf("got %v", got)
	}
}
