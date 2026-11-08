package cmd

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Setup's last question, declined: the command for later, and nothing run.
func TestDecliningTheSampleRunNamesTheCommandForLater(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-test")
	var out bytes.Buffer
	if err := offerSampleRun(bufio.NewReader(strings.NewReader("n\n")), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Run the team on a sample problem now?") || !strings.Contains(out.String(), "Later: memdoor workflow try") {
		t.Fatalf("declined: the question and the command for later:\n%s", out.String())
	}
}

// Each try gets a fresh, empty folder of its own under ~/.memdoor/sample, a
// git repository when git is here: the workflow copies the sample problem
// into an empty folder and keep needs a base to put the first attempt back.
func TestSampleDirIsFreshAndUnderTheHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := sampleDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, filepath.Join(home, ".memdoor", "sample")) {
		t.Fatalf("dir = %s", dir)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != ".git" {
			t.Fatalf("not empty: %s", e.Name())
		}
	}
}
