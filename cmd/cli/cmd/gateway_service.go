package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"memdoor/pkg/shared"
)

// THE DAEMON THAT OWNS THE WORK (Greg, 2026-10-23: "the deamon that owns the
// work is the way to go"). A gateway started by the first run lives as long
// as the login session that started it: a reboot, a logout or a closed lid
// on a laptop ends it, and nothing brings it back. On a box that stays on, a
// VPS, a Mac mini, a home server, the gateway belongs to the service manager
// instead: systemd's user instance on Linux, launchd's user agents on macOS.
// `memdoor gateway install` writes that unit and starts it; from then on the
// box boots, the gateway is up, and the laptop and the phone are windows onto
// it. No root: a user unit under ~/.config, a LaunchAgent under ~/Library.
//
// Keys: a key kept by `memdoor connect` is on disk and reaches the service.
// A key that lived only in a shell does not; on Linux the unit reads
// ~/.memdoor/env when it exists (KEY=value lines), on macOS `memdoor connect`
// is the way.

const (
	systemdUnitName = "memdoor.service"
	launchdLabel    = "ai.memdoor.gateway"
	serviceEnvFile  = "env" // ~/.memdoor/env, read by the Linux unit when present
)

// serviceSpec is everything a unit file needs.
type serviceSpec struct {
	Binary string
	Port   int
	Host   string
	Home   string
	Log    string
	Label  string
}

// servicePath is where this platform's unit lives, and the manager's name.
func servicePath() (path, manager string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), "systemd", nil
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), "launchd", nil
	default:
		return "", "", fmt.Errorf("no service manager on %s yet: start `memdoor gateway` from a Scheduled Task or your init system", runtime.GOOS)
	}
}

// serviceInstalled answers whether a unit is on disk.
func serviceInstalled() bool {
	p, _, err := servicePath()
	if err != nil {
		return false
	}
	_, statErr := os.Stat(p)
	return statErr == nil
}

// systemdUnit renders the user unit.
func systemdUnit(s serviceSpec) string {
	return fmt.Sprintf(`[Unit]
Description=Memdoor gateway (the daemon that owns the work)
Documentation=https://memdoor.ai/docs/gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s gateway --port %d --host %s
Restart=always
RestartSec=3
Environment=HOME=%s
Environment=PATH=%s/.local/bin:/usr/local/bin:/usr/bin:/bin
EnvironmentFile=-%s/.memdoor/%s
WorkingDirectory=%s

[Install]
WantedBy=default.target
`, s.Binary, s.Port, s.Host, s.Home, s.Home, s.Home, serviceEnvFile, s.Home)
}

// launchdPlist renders the LaunchAgent.
func launchdPlist(s serviceSpec) string {
	// No DOCTYPE line: launchd and plutil read the plist without it, and the
	// line would name Apple's DTD host, which the egress check (gateway/
	// egress_test.go) would then list as a destination of this binary.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>gateway</string>
		<string>--port</string>
		<string>%d</string>
		<string>--host</string>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>HOME</key>
		<string>%s</string>
		<key>PATH</key>
		<string>%s/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
	</dict>
</dict>
</plist>
`, s.Label, s.Binary, s.Port, s.Host, s.Home, s.Log, s.Log, s.Home, s.Home)
}

// renderService is the unit for this platform.
func renderService(s serviceSpec) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return systemdUnit(s), nil
	case "darwin":
		return launchdPlist(s), nil
	}
	_, _, err := servicePath()
	return "", err
}

// currentServiceSpec is the spec for this binary, port and host.
func currentServiceSpec(port int, host string) (serviceSpec, error) {
	binary, err := os.Executable()
	if err != nil {
		return serviceSpec{}, err
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return serviceSpec{}, err
	}
	return serviceSpec{Binary: binary, Port: port, Host: host, Home: home, Log: shared.MemdoorHome("gateway.log"), Label: launchdLabel}, nil
}

func launchdTarget() string {
	uid := os.Getuid()
	if uid < 0 {
		if u, err := user.Current(); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		}
	}
	return fmt.Sprintf("gui/%d", uid)
}

// run executes a service manager command and returns its combined output.
func runManager(name string, args ...string) (string, error) {
	var out bytes.Buffer
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr = &out, &out
	err := c.Run()
	return strings.TrimSpace(out.String()), err
}

// installService writes the unit and starts it. With print, the unit goes to
// stdout and nothing is written.
func installService(port int, host string, print bool) error {
	spec, err := currentServiceSpec(port, host)
	if err != nil {
		return err
	}
	text, err := renderService(spec)
	if err != nil {
		return err
	}
	if print {
		fmt.Print(text)
		return nil
	}
	path, manager, err := servicePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(shared.MemdoorHome(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Printf("  ✓ wrote %s\n", path)
	// A gateway the first run started on this port would hold it against the
	// service; it is Memdoor's own, so it is stopped first.
	stopPidFileGateway()
	switch manager {
	case "systemd":
		if out, err := runManager("systemctl", "--user", "daemon-reload"); err != nil {
			return fmt.Errorf("systemctl --user daemon-reload: %v\n%s", err, out)
		}
		if out, err := runManager("systemctl", "--user", "enable", "--now", systemdUnitName); err != nil {
			return fmt.Errorf("systemctl --user enable --now %s: %v\n%s", systemdUnitName, err, out)
		}
		fmt.Printf("  ✓ enabled and started %s (systemd --user)\n", systemdUnitName)
		if u, err := user.Current(); err == nil {
			if out, _ := runManager("loginctl", "show-user", u.Username, "-p", "Linger"); strings.Contains(out, "Linger=no") {
				fmt.Printf("  → to keep it running when you are not logged in:  loginctl enable-linger %s\n", u.Username)
			}
		}
	case "launchd":
		_, _ = runManager("launchctl", "bootout", launchdTarget()+"/"+launchdLabel)
		if out, err := runManager("launchctl", "bootstrap", launchdTarget(), path); err != nil {
			return fmt.Errorf("launchctl bootstrap: %v\n%s", err, out)
		}
		fmt.Printf("  ✓ loaded %s (launchd, runs at login, restarts if it stops)\n", launchdLabel)
	}
	if waitForGateway(20 * time.Second) {
		fmt.Printf("  ✓ gateway answering on %s:%d\n", host, port)
	} else {
		fmt.Printf("  ⚠ the gateway has not answered on %s:%d yet — see %s\n", host, port, shared.MemdoorHome("gateway.log"))
	}
	fmt.Println()
	fmt.Println("Next:")
	fmt.Println("  memdoor connect <provider>      # a key kept on disk reaches the service (a shell export does not)")
	fmt.Println("  memdoor gateway status          # is it up, and who manages it")
	if host != "localhost" && host != "127.0.0.1" {
		fmt.Printf("  memdoor --gateway %s:%d run \"…\"   # from another machine; keep this box behind ssh, Tailscale or a proxy\n", host, port)
	}
	return nil
}

// uninstallService stops the unit and removes it.
func uninstallService() error {
	path, manager, err := servicePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no service installed (%s)", path)
	}
	switch manager {
	case "systemd":
		_, _ = runManager("systemctl", "--user", "disable", "--now", systemdUnitName)
	case "launchd":
		_, _ = runManager("launchctl", "bootout", launchdTarget()+"/"+launchdLabel)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if manager == "systemd" {
		_, _ = runManager("systemctl", "--user", "daemon-reload")
	}
	fmt.Printf("  ✓ stopped and removed %s\n", path)
	return nil
}

// serviceStart asks the manager to start the installed unit.
func serviceStart() error {
	_, manager, err := servicePath()
	if err != nil {
		return err
	}
	switch manager {
	case "systemd":
		out, err := runManager("systemctl", "--user", "start", systemdUnitName)
		if err != nil {
			return fmt.Errorf("systemctl --user start: %v\n%s", err, out)
		}
	case "launchd":
		out, err := runManager("launchctl", "kickstart", launchdTarget()+"/"+launchdLabel)
		if err != nil {
			return fmt.Errorf("launchctl kickstart: %v\n%s", err, out)
		}
	}
	return nil
}

// serviceState is the manager's word on the unit: "active", "inactive",
// "failed", "loaded" (launchd, running), "not loaded", or "" when nothing
// is installed.
func serviceState() string {
	path, manager, err := servicePath()
	if err != nil {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	switch manager {
	case "systemd":
		out, _ := runManager("systemctl", "--user", "is-active", systemdUnitName)
		if out == "" {
			return "unknown"
		}
		return out
	case "launchd":
		out, err := runManager("launchctl", "print", launchdTarget()+"/"+launchdLabel)
		if err != nil {
			return "not loaded"
		}
		if strings.Contains(out, "state = running") {
			return "running"
		}
		return "loaded"
	}
	return "installed"
}

// stopPidFileGateway ends the gateway the first run started, when its pid
// file names one that is still alive, so the service can take the port.
func stopPidFileGateway() {
	raw, err := os.ReadFile(shared.MemdoorHome(gatewayPidFile))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return
	}
	if p, err := os.FindProcess(pid); err == nil {
		if err := stopProcessGracefully(pid); err == nil {
			fmt.Printf("  ✓ stopped the gateway the first run started (pid %d)\n", pid)
		}
		_ = p.Release()
	}
	_ = os.Remove(shared.MemdoorHome(gatewayPidFile))
}

// waitForGateway polls the configured gateway until it answers or the clock
// runs out.
func waitForGateway(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if gatewayReachable() {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// isLoopbackHost says whether a listen host is reachable only from this
// machine.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}
