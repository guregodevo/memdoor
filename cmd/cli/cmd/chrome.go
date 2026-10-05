package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"memdoor/tools"

	"github.com/spf13/cobra"
)

var chromeTimeout int

var chromeCmd = &cobra.Command{
	Use:   "chrome",
	Short: "Browser automation using DSL",
	Long:  "Browser automation using a simple DSL (Domain-Specific Language) syntax. Connects to Chrome via DevTools Protocol.",
}

var chromeRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute browser script from stdin",
	Long: `Execute a browser automation script from stdin.

Example:
  memdoor chrome run <<'EOF'
  nav http://localhost:5173
  wait 2
  type input[type="email"] user@test.com
  click button[type="submit"]
  wait 3
  snap /tmp/logged-in.png
  EOF`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse DSL from stdin
		parser := tools.NewParser()
		commands, err := parser.Parse(os.Stdin)
		if err != nil {
			return fmt.Errorf("parse error: %w", err)
		}

		if len(commands) == 0 {
			return fmt.Errorf("no commands found in input")
		}

		// Create browser tool
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
		}

		browserTool, err := tools.NewBrowserTool(logger)
		if err != nil {
			return fmt.Errorf("failed to create browser: %w", err)
		}

		browserCtx, browserCancel := context.WithCancel(context.Background())
		defer browserCancel()

		if err := browserTool.Start(browserCtx); err != nil {
			return fmt.Errorf("failed to start browser: %w", err)
		}
		defer func() {
			if stopErr := browserTool.Stop(); stopErr != nil {
				logger.Warn("Failed to stop browser", slog.String("error", stopErr.Error()))
			}
		}()

		// Wait for MCP server to initialize
		time.Sleep(2 * time.Second)

		// Execute commands
		timeout := time.Duration(chromeTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		executor := tools.NewExecutor(browserTool)
		executed := 0

		for i, command := range commands {
			result, err := executor.Execute(ctx, command)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ [%d] %s failed: %v\n", i, command.Type(), err)
				return fmt.Errorf("command %d (%s) failed: %w", i, command.Type(), err)
			}

			executed++
			printCommandResult(i, command, result)
		}

		fmt.Printf("\nScript complete: %d commands executed\n", executed)
		return nil
	},
}

func printCommandResult(index int, cmd tools.Command, result interface{}) {
	switch cmd.Type() {
	case "nav":
		navCmd := cmd.(tools.NavigateCommand)
		fmt.Printf("✓ [%d] Navigated to %s\n", index, navCmd.URL)

	case "wait":
		waitCmd := cmd.(tools.WaitCommand)
		fmt.Printf("✓ [%d] Waited %.1fs\n", index, waitCmd.Seconds)

	case "snap":
		snapCmd := cmd.(tools.ScreenshotCommand)
		info, err := os.Stat(snapCmd.Path)
		if err == nil {
			fmt.Printf("✓ [%d] Screenshot saved to %s (%d bytes)\n", index, snapCmd.Path, info.Size())
		} else {
			fmt.Printf("✓ [%d] Screenshot saved to %s\n", index, snapCmd.Path)
		}

	case "snapshot":
		text := fmt.Sprintf("%v", result)
		fmt.Printf("✓ [%d] DOM snapshot (%d chars)\n", index, len(text))
		fmt.Println(text)

	case "click":
		fmt.Printf("✓ [%d] Clicked: %v\n", index, result)

	case "click-text":
		fmt.Printf("✓ [%d] Clicked text: %v\n", index, result)

	case "type":
		fmt.Printf("✓ [%d] Typed: %v\n", index, result)

	case "exec":
		fmt.Printf("✓ [%d] Eval: %v\n", index, result)

	case "press":
		fmt.Printf("✓ [%d] Pressed: %v\n", index, result)

	case "console", "network":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("✓ [%d] %s:\n%s\n", index, cmd.Type(), string(data))

	case "reload":
		fmt.Printf("✓ [%d] Reloaded\n", index)

	case "hover":
		fmt.Printf("✓ [%d] Hovered: %v\n", index, result)

	case "focus":
		fmt.Printf("✓ [%d] Focused: %v\n", index, result)

	case "send":
		fmt.Printf("✓ [%d] Sent: %v\n", index, result)

	default:
		fmt.Printf("✓ [%d] %s: %v\n", index, cmd.Type(), result)
	}
}

func init() {
	chromeCmd.PersistentFlags().IntVar(&chromeTimeout, "timeout", 30, "Timeout in seconds for operations")
	chromeCmd.AddCommand(chromeRunCmd)
	rootCmd.AddCommand(chromeCmd)
}
