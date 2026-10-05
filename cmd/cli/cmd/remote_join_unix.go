//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// terminalResized fires when this terminal changes size (SIGWINCH).
func terminalResized() <-chan struct{} {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	out := make(chan struct{}, 1)
	go func() {
		for range sig {
			select {
			case out <- struct{}{}:
			default:
			}
		}
	}()
	return out
}
