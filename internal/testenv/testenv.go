// Package testenv is the machine a test pretends to be on: a home directory of
// its own, and the file permissions the platform actually keeps (#283).
//
// A test used to set HOME alone. That isolates it on Unix; on Windows
// os.UserHomeDir reads USERPROFILE, so such a test wrote into the runner's real
// home — and the state one test left behind failed the next one in the package.
package testenv

import (
	"os"
	"runtime"
	"testing"
)

// Home points the home directory at a fresh temporary directory for the length
// of the test, and returns it. os.UserHomeDir and anything built on
// shared.MemdoorHome then land there on every OS.
func Home(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	SetHome(t, home)
	return home
}

// SetHome points the home directory at dir for the length of the test: HOME,
// which Unix reads, and USERPROFILE, which is what Windows reads.
func SetHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

// OwnerOnly fails unless path exists and only its owner can read it.
//
// On Windows there are no POSIX mode bits to check: os.Chmod there sets the
// read-only flag and Stat always reports -rw-rw-rw-, whatever the file was
// created with. What keeps a file in the profile private is the profile's ACL,
// so there the check is that the file was written at all.
func OwnerOnly(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("not written: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("must be owner-only, got %v", fi.Mode())
	}
}
