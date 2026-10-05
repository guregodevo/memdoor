package compaction

import (
	"os"
	"testing"

	"memdoor/gateway/logs"
)

func TestMain(m *testing.M) {
	logs.InitGlobalLoggerDefault(false)
	os.Exit(m.Run())
}
