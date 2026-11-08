//go:build ignore

// This test host lets the real approval gate and its tests compile without
// importing the entire gateway, whose embedded frontend requires web/dist.
// It supplies only the host fields and formatting helpers used by approval.go.
// The event emitter, question broker, approval gate, and policy are production
// code. This does not replace full gateway integration tests.
//
// From the repository root:
//
//	go test -race gateway/approval.go gateway/workflow_approval.go \
//	  gateway/question_broker.go gateway/util.go \
//	  gateway/approval_workflow_test.go gateway/workflow_approval_test.go \
//	  gateway/approval_standalone_test.go
package gateway

import (
	"os"
	"strings"
	"testing"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
)

type AgentRuntime struct {
	events           *infra.EventEmitter
	questions        *questionBroker
	approvals        approvals
	workspaceSetting func(string) string
}

type Session struct{ ID string }

const workflowToolName = "workflow"

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max]
	}
	return s
}

func truncateForLog(s string, max int) string { return oneLine(s, max) }

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "memdoor-approval-tests-")
	if err != nil {
		panic(err)
	}
	if err := logs.InitGlobalLogger(dir, false); err != nil {
		os.RemoveAll(dir)
		panic(err)
	}
	code := m.Run()
	logs.CloseGlobalLogger()
	os.RemoveAll(dir)
	os.Exit(code)
}
