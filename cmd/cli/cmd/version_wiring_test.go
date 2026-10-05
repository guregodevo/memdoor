package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every long-running service in this binary must publish the version it is
// running, and each one is a SEPARATE systemd unit that does not inherit the
// others' setup.
//
// The 2026-08-29 deploy proved why this needs a test rather than care: the
// version field was added to the billing service's /v1/health, shipped, and
// reported "" — because telemetry.GatewayVersion was assigned in the gateway
// command only. The change looked complete and did nothing, and only a deploy
// showed it. Reading the source is the cheapest way to catch the next one.
func TestEveryServiceCommandPublishesItsVersion(t *testing.T) {
	// Commands that start a long-lived server and expose a health endpoint.
	services := []string{"gateway.go", "billing.go"}

	assign := regexp.MustCompile(`telemetry\.GatewayVersion\s*=\s*Version`)
	for _, f := range services {
		b, err := os.ReadFile(filepath.Join(".", f))
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		if !assign.Match(b) {
			t.Errorf("%s starts a service but never sets telemetry.GatewayVersion — "+
				"its health endpoint will report an empty version, which is how you cannot "+
				"tell what is running in production", f)
		}
	}
}

// Version itself must have a value the ldflags can replace, and must not be
// empty in a plain build — an empty default is indistinguishable from a
// deploy that failed to stamp one.
func TestVersionHasADefault(t *testing.T) {
	if strings.TrimSpace(Version) == "" {
		t.Error("Version must default to something (e.g. \"dev\"), or an unstamped build " +
			"looks identical to a broken deploy")
	}
}
