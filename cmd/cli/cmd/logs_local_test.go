package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"memdoor/gateway/logs"
)

func capture(t *testing.T, run func() error) (stdout, stderr string, err error) {
	t.Helper()
	or, ow, _ := os.Pipe()
	er, ew, _ := os.Pipe()
	so, se := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = ow, ew
	err = run()
	os.Stdout, os.Stderr = so, se
	_ = ow.Close()
	_ = ew.Close()
	o, _ := io.ReadAll(or)
	e, _ := io.ReadAll(er)
	return string(o), string(e), err
}

// A gateway on this machine that refuses the sign-in ("authentication
// required": another HOME's gateway on the port, a stale token) does not stop
// the read: the log is this machine's file, read here.
func TestLogsReadThisMachinesFileWhenTheGatewayRefuses(t *testing.T) {
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	t.Setenv("HOME", t.TempDir())
	st, err := logs.NewSQLiteStorage(logs.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	warn := logs.NewEvent(logs.EventStateChange, "Remote", "provider sent only keep-alive comments before any text; asking again")
	warn.Level = logs.LevelWarn
	failed := logs.NewEvent(logs.EventError, "Workflow", "title-check timed out after 20m")
	failed.Level = logs.LevelError
	if err := st.WriteEvents(context.Background(), []*logs.Event{warn, failed}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
	}))
	defer gw.Close()
	gatewayAddr = gw.URL

	logsRegex, logsSince = "keep-alive", "1h"
	t.Cleanup(func() { logsRegex, logsSince = "", "" })
	out, errOut, err := capture(t, func() error { return logsQueryCmd.RunE(logsQueryCmd, nil) })
	if err != nil || !strings.Contains(out, "keep-alive comments") || strings.Contains(out, "title-check") || !strings.Contains(out, "(1 events)") {
		t.Fatalf("query: err=%v\n%s", err, out)
	}
	if !strings.Contains(errOut, "authentication required") || !strings.Contains(errOut, "events.db") {
		t.Fatalf("stderr says where the lines came from: %q", errOut)
	}

	logsRegex = ""
	out, _, err = capture(t, func() error { return logsErrorsCmd.RunE(logsErrorsCmd, nil) })
	if err != nil || !strings.Contains(out, "title-check timed out") {
		t.Fatalf("errors: err=%v\n%s", err, out)
	}

	// No gateway at all on this machine: the same file.
	gw.Close()
	out, _, err = capture(t, func() error { return logsErrorsCmd.RunE(logsErrorsCmd, nil) })
	if err != nil || !strings.Contains(out, "title-check timed out") {
		t.Fatalf("no gateway: err=%v\n%s", err, out)
	}
}

// A gateway on another machine keeps its log there: its error stands, and
// this machine's file is not read in its place.
func TestLogsFromAnotherMachineKeepItsError(t *testing.T) {
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	t.Setenv("HOME", t.TempDir())
	st, err := logs.NewSQLiteStorage(logs.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	gatewayAddr = "http://box.invalid:18789"
	_, _, err = capture(t, func() error { return logsErrorsCmd.RunE(logsErrorsCmd, nil) })
	if err == nil || strings.Contains(err.Error(), "events.db") {
		t.Fatalf("a remote gateway's error stands: %v", err)
	}
}
