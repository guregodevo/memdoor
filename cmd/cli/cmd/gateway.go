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
	Short: "Start the gateway server",
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
			fmt.Println("  Stop it with:   kill $(lsof -ti:18789)")
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

		// One-line banner so the user sees the URL immediately and most
		// terminals auto-link it. Below this, slog prints structured
		// startup logs from gateway.Start.
		fmt.Printf("Memdoor gateway → http://%s:%d\n", gatewayHost, gatewayPort)
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
	url := fmt.Sprintf("http://%s:%d/health", host, port)
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

func init() {
	gatewayCmd.Flags().IntVar(&gatewayPort, "port", 18789, "Server port")
	gatewayCmd.Flags().StringVar(&gatewayHost, "host", "localhost", "Server host")
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
