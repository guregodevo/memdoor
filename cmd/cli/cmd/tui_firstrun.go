package cmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/pkg/shared"
)

// THE FIRST COMMAND IS `memdoor tui` (Greg, 2026-10-08: "it should be 100%
// frictionless"). A stranger walk on a clean machine found three walls
// between the installer and a first answer, and the logs agreed: about
// twenty installs on 2026-10-07, not one TUI opened. The installer's PATH
// was not the shell's; `memdoor setup` asked a solo developer for an admin
// email and password; and a key exported after setup never reached the
// gateway setup had already started, so the TUI said "no model configured"
// to someone who had done exactly what it said. This file removes the last
// two: the TUI sets the machine up on its first run, and hands a key the
// shell holds to the gateway Memdoor started.

// gatewayPidFile holds the pid of the gateway Memdoor started itself
// (ensureGatewayRunning), the only one it may restart.
const gatewayPidFile = "gateway.pid"

// localGatewayPort is the port of the gateway the CLI talks to, and whether
// that gateway is on this machine.
func localGatewayPort() (string, bool) {
	u, err := url.Parse(gatewayAddr)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
	default:
		return "", false
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	return port, true
}

// firstRunReady leaves a machine that has never run Memdoor with a running
// gateway, a workspace and a session, without a question: the local user is
// generated (~/.memdoor/local-login, the same file the Pro sign-in writes).
// A machine that HAS been set up keeps its own messages: a gateway whose
// first user exists is never taken over from here.
func firstRunReady() error {
	if _, local := localGatewayPort(); local {
		ensureGatewayRunning()
	}
	if sessionProblem() == nil {
		return nil
	}
	if renewLocalSession() == nil {
		return nil
	}
	if _, err := loadCredentials(); err == nil {
		return sessionProblem()
	}
	var status struct {
		Initialized bool `json:"initialized"`
	}
	if err := NewClient().GetJSON("/api/setup/status", &status); err != nil || status.Initialized {
		return sessionProblem()
	}
	fmt.Fprintln(os.Stderr, "Setting up this machine (once)…")
	if ok, why := ensureLocalEngine(localUserEmail(), "Memdoor"); !ok {
		return fmt.Errorf("could not set this machine up: %s", why)
	}
	if slug, _, found := resolveWorkspaceSlug(); found {
		workspaceSlug = slug
	} else if only, single := soleWorkspace(); single {
		workspaceSlug = only
	}
	return nil
}

// localUserEmail names this machine's own user. Nobody signs in with it by
// hand: the password is generated and kept in ~/.memdoor/local-login.
func localUserEmail() string {
	name := "you"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = strings.ToLower(strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
				return r
			}
			return -1
		}, u.Username))
		if name == "" {
			name = "you"
		}
	}
	return name + "@localhost"
}

// providerKeyInShell is the first provider key this shell holds, by the
// variable it is under, or "" when it holds none.
func providerKeyInShell() string {
	vars := []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY"}
	for _, k := range ui.ConnectKinds {
		if k.KeyVar != "" && k.API != "decisions" {
			vars = append(vars, k.KeyVar)
		}
	}
	for _, v := range vars {
		if strings.TrimSpace(os.Getenv(v)) != "" {
			return v
		}
	}
	return ""
}

// handShellKeyToGateway restarts the gateway Memdoor started when it has no
// model and this shell holds a provider key: the gateway reads keys from its
// environment when it starts, so a key exported after it started never
// arrived. A gateway with no model runs no turn, so nothing is cut short. A
// gateway a person started by hand, or one that does not report the pid
// Memdoor recorded, is never touched; the window then says what to do.
func handShellKeyToGateway(c *Client) {
	if readBrain(c).State != "none" {
		return
	}
	keyVar := providerKeyInShell()
	if keyVar == "" {
		return
	}
	port, local := localGatewayPort()
	if !local {
		return
	}
	b, err := os.ReadFile(shared.MemdoorHome(gatewayPidFile))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	var st struct {
		PID int `json:"pid"`
	}
	if c.GetJSON("/api/status", &st) != nil || st.PID != pid {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if p.Signal(os.Interrupt) != nil {
		_ = p.Kill()
	}
	for i := 0; i < 40 && portAnswers(port); i++ {
		time.Sleep(250 * time.Millisecond)
	}
	if portAnswers(port) {
		return
	}
	ensureGatewayRunning()
	if readBrain(c).State != "none" {
		fmt.Fprintf(os.Stderr, "✓ The engine now runs on your %s.\n", keyVar)
	}
}

// portAnswers reports whether something listens on the local port.
func portAnswers(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
