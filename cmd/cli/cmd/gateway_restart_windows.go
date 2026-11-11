//go:build windows

package cmd

import (
	"errors"
	"syscall"
)

// Windows has no SIGUSR2 and no exec: the installer stops the gateway and
// starts it again, as before. SIGTERM stands in so the signal list compiles.
const restartSignal = syscall.SIGTERM

const canRestartInPlace = false

func execSelf(string) error { return errors.New("restart in place is not available on Windows") }
