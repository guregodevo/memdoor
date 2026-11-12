// Package testenv is the machine a test pretends to be on: a home directory of
// its own, a log store that is closed again, and the file permissions the
// platform actually keeps (#283).
//
// A test used to set HOME alone. That isolates it on Unix; on Windows
// os.UserHomeDir reads USERPROFILE, so such a test wrote into the runner's real
// home — and the state one test left behind failed the next one in the package.
package testenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"memdoor/gateway/logs"
)

// Logger points the global event logger at a store of this test's own, under a
// temporary directory, and closes it when the test ends.
//
// The close matters on Windows: a directory holding an open events.db cannot be
// removed, so every test that left the logger running failed its own cleanup
// ("TempDir RemoveAll cleanup: ... events.db ... used by another process"). The
// store is closed before the directory goes because this cleanup is registered
// after t.TempDir's, and cleanups run last-registered-first.
func Logger(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := logs.InitGlobalLogger(dir, false); err != nil {
		t.Fatalf("InitGlobalLogger: %v", err)
	}
	t.Cleanup(func() {
		_ = logs.CloseGlobalLogger()
		// Later tests in this package expect a logger (logs.New panics without
		// one). InitGlobalLoggerDefault under `go test` writes to a temp
		// directory of its own, which no test cleans up — so nothing holds the
		// one that is being removed.
		_ = logs.InitGlobalLoggerDefault(false)
	})
}

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

// Abs is a POSIX absolute path as an absolute path on this OS: the same text on
// Unix, the same path on the drive of the temporary directory on Windows, where
// "/Users/dev" has no drive and filepath.IsAbs says it is not absolute.
func Abs(posix string) string {
	if runtime.GOOS != "windows" {
		return posix
	}
	return filepath.VolumeName(os.TempDir()) + filepath.FromSlash(posix)
}
