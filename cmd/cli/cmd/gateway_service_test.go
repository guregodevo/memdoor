package cmd

import (
	"strings"
	"testing"
)

// The unit a box that stays on runs the gateway from: the binary that wrote
// it, the port and host it was asked for, a restart when it stops, and the
// person's own home, since nothing here runs as root.
func TestSystemdUnitNamesTheBinaryPortAndRestart(t *testing.T) {
	u := systemdUnit(serviceSpec{Binary: "/home/ana/.local/bin/memdoor", Port: 18799, Host: "0.0.0.0", Home: "/home/ana", Log: "/home/ana/.memdoor/gateway.log"})
	for _, want := range []string{
		"ExecStart=/home/ana/.local/bin/memdoor gateway --port 18799 --host 0.0.0.0",
		"Restart=always",
		"EnvironmentFile=-/home/ana/.memdoor/env",
		"WantedBy=default.target",
		"Environment=HOME=/home/ana",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit lacks %q:\n%s", want, u)
		}
	}
}

func TestLaunchdPlistKeepsTheGatewayAliveAndLogsBesideTheData(t *testing.T) {
	p := launchdPlist(serviceSpec{Binary: "/Users/ana/.local/bin/memdoor", Port: 18789, Host: "localhost", Home: "/Users/ana", Log: "/Users/ana/.memdoor/gateway.log", Label: launchdLabel})
	for _, want := range []string{
		"<string>ai.memdoor.gateway</string>",
		"<string>/Users/ana/.local/bin/memdoor</string>",
		"<string>18789</string>",
		"<key>KeepAlive</key>\n\t<true/>",
		"<key>RunAtLoad</key>\n\t<true/>",
		"<string>/Users/ana/.memdoor/gateway.log</string>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist lacks %q:\n%s", want, p)
		}
	}
}

func TestLoopbackHostsAreTheOnesThatNeedNoWarning(t *testing.T) {
	for host, loop := range map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": false, "192.168.1.10": false, "": false} {
		if got := isLoopbackHost(host); got != loop {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, loop)
		}
	}
}
