//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// restartSignal asks a running gateway to restart in place: it shuts down as
// on SIGTERM, then executes the binary at its own path again with its own
// arguments and environment. The installer swaps the file and sends it, so
// the keys the gateway was started with stay (2026-11-11: a /update restart
// from the window's shell dropped AIMLAPI_API_KEY).
const restartSignal = syscall.SIGUSR2

// canRestartInPlace: exec exists here.
const canRestartInPlace = true

// execSelf replaces this process with the binary at exe. The pid stays, so
// ~/.memdoor/gateway.pid still names the gateway.
func execSelf(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
