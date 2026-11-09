package ui

import (
	"os"
	"testing"
)

// The suite never reaches the cmux the tests run in: a test that goes
// through Update would draw its status in the developer's own sidebar.
// A test that wants cmux points these at a fake (cmux_test.go fakeCmux).
func TestMain(m *testing.M) {
	_ = os.Unsetenv("CMUX_BUNDLED_CLI_PATH")
	_ = os.Unsetenv("CMUX_WORKSPACE_ID")
	os.Exit(m.Run())
}
