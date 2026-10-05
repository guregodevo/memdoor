//go:build windows

package cmd

// terminalResized: Windows has no SIGWINCH; the size set when joining stays.
func terminalResized() <-chan struct{} { return nil }
