package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"memdoor/gateway"
	"memdoor/gateway/providers"
	"memdoor/gateway/telemetry"
	"memdoor/pkg/shared"

	"github.com/spf13/cobra"
)

var (
	gatewayPort int
	gatewayHost string
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "The gateway in the foreground; install, status and uninstall run it as a service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Pre-bind probe: if another memdoor gateway is already serving
		// on this host:port and healthy, the install / boot script
		// should treat that as success — not an error. Pre-fix this
		// command went straight to ListenAndServe, hit EADDRINUSE, and
		// exited 1 even though `memdoor health` would have reported
		// healthy. The installer then surfaced it as an install
		// failure.
		if running, info := probeGatewayHealth(gatewayHost, gatewayPort); running {
			fmt.Printf("Gateway already running on %s:%d — leaving it alone.\n",
				gatewayHost, gatewayPort)
			if info != "" {
				fmt.Println(info)
			}
			fmt.Println("  Check it with:  memdoor health")
			fmt.Printf("  Stop it with:   kill $(lsof -ti:%d)\n", gatewayPort)
			return nil
		}
		keepCrashOutput()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle shutdown signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\nShutting down...")
			cancel()
		}()

		// Pass the linker-injected Version to the telemetry layer
		// before the gateway boots — the storage decorator in
		// gateway/server.go reads telemetry.GatewayVersion when it
		// installs the wrapper. Cycle-free: cmd → telemetry; gateway
		// → telemetry; cmd → gateway; no telemetry → cmd or gateway → cmd.
		telemetry.GatewayVersion = Version
		providers.SetAppVersion(Version)

		// One-line banner so the user sees the URL immediately and most
		// terminals auto-link it. Below this, slog prints structured
		// startup logs from gateway.Start.
		fmt.Printf("Memdoor gateway → http://%s:%d\n", gateway.DialHost(gatewayHost), gatewayPort)
		if !isLoopbackHost(gatewayHost) {
			// The network is the default (Greg, 2026-10-23: "default to be
			// the network"): a phone reaches the gateway directly. It
			// speaks plain HTTP, every door takes a token, and the first
			// account is made from this machine alone; say where it is
			// reachable and what keeps it to this machine.
			fmt.Printf("  also on this network, for your phone: %s\n", networkURLs(gatewayHost, gatewayPort))
			fmt.Printf("  plain HTTP, every door token-gated: trust that network, or MEMDOOR_HOST=localhost keeps it to this machine\n")
		}
		// This process hosts the phone's links (remote_host.go): a window
		// hands /remote over, and the link outlives the window.
		gateway.SetRemoteHost(newGatewayRemoteHost(gatewayHost, gatewayPort))
		// This process asks peers for @<peer>:<agent> (peers.go, experimental).
		gateway.SetPeerCaller(peerAsk)
		err := gateway.Start(ctx, gatewayHost, gatewayPort, "", verbose)
		// Mid-flight EADDRINUSE: rare (probe above is the common case)
		// but possible if a stale gateway started between the probe
		// and our bind. Surface the same friendly message so the
		// installer doesn't choke; differentiate "port held by
		// something else entirely" from "stale memdoor" by re-probing
		// health on the explicit fall-through path.
		if err != nil && isAddrInUseErr(err) {
			if running, _ := probeGatewayHealth(gatewayHost, gatewayPort); running {
				fmt.Printf("Gateway already running on %s:%d — nothing to do.\n",
					gatewayHost, gatewayPort)
				return nil
			}
			return fmt.Errorf(
				"port %d on %s is in use but doesn't respond to /health — "+
					"another process is holding the address. Find it with: lsof -iTCP:%d -sTCP:LISTEN",
				gatewayPort, gatewayHost, gatewayPort)
		}
		return err
	},
}

// probeGatewayHealth does a fast HTTP GET against /health and reports
// whether a real memdoor gateway is answering. Returns the response
// summary so the caller can surface version/uptime. 1s timeout — long
// enough for a healthy local gateway, short enough that an empty port
// returns "no" within the install script's patience.
func probeGatewayHealth(host string, port int) (bool, string) {
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("http://%s:%d/health", gateway.DialHost(host), port)
	resp, err := client.Get(url)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, ""
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, ""
	}
	// Be defensive: any HTTP server can answer 200, but only memdoor
	// returns these fields. Without the check, e.g. an unrelated
	// localhost server on the port would falsely register as "healthy
	// memdoor." Require at least one memdoor-shaped field.
	if _, ok := body["version"]; !ok {
		if _, ok := body["agents"]; !ok {
			return false, ""
		}
	}
	bits := []string{}
	if v, ok := body["version"]; ok {
		bits = append(bits, fmt.Sprintf("version=%v", v))
	}
	if v, ok := body["uptime"]; ok {
		bits = append(bits, fmt.Sprintf("uptime=%v", v))
	}
	if len(bits) == 0 {
		return true, ""
	}
	return true, "  " + strings.Join(bits, "  ")
}

// isAddrInUseErr reports whether the error chain contains a bind-time
// "address already in use" syscall error. Pre-fix we matched on
// strings.Contains; net.OpError + syscall.EADDRINUSE gives a typed
// path that survives Go stdlib error-text changes between releases.
func isAddrInUseErr(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.EADDRINUSE) {
			return true
		}
	}
	return errors.Is(err, syscall.EADDRINUSE)
}

// THE GATEWAY AS A SERVICE (gateway_service.go): on a box that stays on, the
// service manager owns it; the laptop and the phone are windows onto it.
var gatewayInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Run the gateway as a service on this machine: up at boot, restarted if it stops, no root (systemd --user, launchd)",
	Long: `Writes a user-level service for this binary and starts it:
Linux   ~/.config/systemd/user/memdoor.service   (systemctl --user)
macOS   ~/Library/LaunchAgents/ai.memdoor.gateway.plist   (launchctl)

For a box that stays on, a VPS, a Mac mini, a home server: the gateway comes up
at boot and stays up, so a run keeps going when no window is open and no laptop
is awake. A key kept by ` + "`memdoor connect`" + ` is on disk and reaches the service;
on Linux the unit also reads ~/.memdoor/env (KEY=value lines) when it exists.

Examples:
  memdoor gateway install                      # :18789 on this machine and its network, for your laptop and your phone
  memdoor gateway install --host localhost     # this machine alone; the ssh door (--gateway ssh://box) still reaches it
  memdoor gateway install --print              # show the unit, write nothing
  memdoor --gateway <host>:18789 run "…"       # from another machine, in that box's project`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		print, _ := cmd.Flags().GetBool("print")
		return installService(gatewayPort, gatewayHost, print)
	},
}

var gatewayUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop the gateway service and remove its unit (the data stays)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return uninstallService()
	},
}

// The exit code is the answer too: 0 when the gateway answers, 1 when not,
// so a script waits for its gateway with it (the CI job that hands a run to
// a phone: `memdoor gateway &`, then `until memdoor gateway status; do sleep
// 1; done`).
var gatewayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Is the gateway up, which version, and who manages it (exit 1 when it does not answer)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var st struct {
			Version string `json:"version"`
			PID     int    `json:"pid"`
		}
		var down error
		if err := c.GetJSON("/api/status", &st); err != nil {
			down = fmt.Errorf("gateway    not answering at %s", gatewayAddr)
		} else {
			fmt.Printf("gateway    %s answering at %s (pid %d)\n", st.Version, gatewayAddr, st.PID)
		}
		switch state := serviceState(); {
		case state == "":
			if _, err := os.Stat(shared.MemdoorHome(gatewayPidFile)); err == nil {
				fmt.Println("managed by this machine's first run (gateway.pid); ends with the login session — `memdoor gateway install` makes it a service")
			} else {
				fmt.Println("service    none installed (memdoor gateway install)")
			}
		default:
			path, manager, _ := servicePath()
			fmt.Printf("service    %s, %s (%s)\n", state, manager, path)
		}
		return down
	},
}

func init() {
	gatewayCmd.Flags().IntVar(&gatewayPort, "port", 18789, "The port it listens on")
	gatewayCmd.Flags().StringVar(&gatewayHost, "host", envOr("MEMDOOR_HOST", defaultBindHost), "The address it binds (or MEMDOOR_HOST): 0.0.0.0 for this machine and its network, so a phone reaches it; localhost for this machine only")
	gatewayInstallCmd.Flags().IntVar(&gatewayPort, "port", 18789, "the port the service listens on")
	gatewayInstallCmd.Flags().StringVar(&gatewayHost, "host", envOr("MEMDOOR_HOST", defaultBindHost), "the address it binds (or MEMDOOR_HOST): 0.0.0.0 for this machine and its network, localhost for this machine only")
	gatewayInstallCmd.Flags().Bool("print", false, "print the unit instead of installing it")
	gatewayCmd.AddCommand(gatewayInstallCmd, gatewayUninstallCmd, gatewayStatusCmd)
	rootCmd.AddCommand(gatewayCmd)
}

// keepCrashOutput makes a crash leave its last words on disk whoever
// started the gateway and wherever its stderr went: the Mac app discards
// stderr, and a restart from a shell overwrote it — three silent deaths
// on 2026-09-13 left nothing to read. The Go runtime writes an unrecovered
// panic or fatal error (a Metal allocation failure ends here too) to this
// file as well as to stderr; the file is append-only and only grows on a
// crash.
func keepCrashOutput() {
	dir := shared.MemdoorHome("logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "\n=== gateway %s started %s (pid %d): a crash below is this run's ===\n", Version, time.Now().Format(time.RFC3339), os.Getpid())
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
}

// defaultBindHost is where the gateway listens unless told otherwise: this
// machine and its network, so a phone has a door of its own (2026-10-23).
const defaultBindHost = "0.0.0.0"

// networkURLs names the gateway on the network, for the start-up line.
func networkURLs(host string, port int) string {
	addrs := gateway.ListenAddresses(host)
	if len(addrs) == 0 {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, fmt.Sprintf("http://%s:%d", a, port))
	}
	return strings.Join(out, "  ")
}
