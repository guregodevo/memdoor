package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// After a provider is connected the next step is the window, as the installer
// says, not the list of commands (2026-11-18, a first connect on Linux printed
// "→ Next: memdoor --help").
func TestConnectSaysTheWindowIsNext(t *testing.T) {
	root := &cobra.Command{Use: "memdoor"}
	connect := &cobra.Command{Use: "connect"}
	root.AddCommand(connect)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = printNextStep(connect, nil)
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, "→ Next: cd <your-project> && memdoor") || strings.Contains(got, "--help") {
		t.Fatalf("after connect: %q", got)
	}
}
