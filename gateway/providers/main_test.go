package providers

import (
	"testing"

	"memdoor/gateway/logs"
	"os"
)

// logs.New panics without an
// initialised global logger. Production initialises it at start-up; a test has
// to say so itself.
func TestMain(m *testing.M) {
	_ = logs.InitGlobalLogger(os.TempDir(), false)
	os.Exit(m.Run())
}
