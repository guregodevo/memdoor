package billingsvc

import (
	"os"
	"testing"

	"memdoor/gateway/logs"
)

// The billing service logs through the global EventLogger, so any test that
// reaches a code path with a log line panics without one. It had no TestMain,
// which meant those paths simply could not be tested — and the watchdog
// decisions are exactly the paths worth testing.
func TestMain(m *testing.M) {
	if err := logs.InitGlobalLoggerDefault(false); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
