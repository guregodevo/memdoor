package cmd

import (
	"bufio"
	"bytes"
	"memdoor/internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	testenv.SetHome(t, home)
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

// The sample run ends by saying how to run team-solve on a real task and where
// to say how it went; it names no telemetry and sends nothing (#221).
func TestTheSampleRunEndsByAskingHowItWent(t *testing.T) {
	for _, want := range []string{`cd <your-project> && memdoor`, "Shift+Tab to Team Mode", `memdoor workflow run team-solve "<the task>"`, "github.com/guregodevo/memdoor/issues/221", "pass or fail"} {
		if !strings.Contains(sampleNextLine, want) {
			t.Errorf("the closing lines lack %q:\n%s", want, sampleNextLine)
		}
	}
	if strings.Contains(strings.ToLower(sampleNextLine), "telemetry") || strings.Count(sampleNextLine, "\n") > 4 {
		t.Errorf("a short closing, no telemetry words:\n%s", sampleNextLine)
	}
}

// The sample folder starts with a commit: the run takes its clean copy from it
// and keep can put the first attempt back (the sample run hung without it).
func TestSampleDirHasACommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	testenv.SetHome(t, t.TempDir())
	dir, err := sampleDir()
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "rev-parse", "--verify", "-q", "HEAD")
	c.Dir = dir
	if err := c.Run(); err != nil {
		t.Fatalf("the sample folder has no commit: %v", err)
	}
}

// A gateway that is slow to start is waited for, and the line said when it still
// does not answer names the wait it used and that it may still be starting.
func TestTheGatewayWaitIsLongerThanFifteenSecondsAndSaysWhatItWaitedFor(t *testing.T) {
	if gatewayStartWait < 30*time.Second {
		t.Errorf("gatewayStartWait = %v; a fresh start was seen taking over 20 s", gatewayStartWait)
	}
	line := gatewayGiveUpLine()
	if !strings.Contains(line, "40s") || !strings.Contains(line, "may still be starting") {
		t.Errorf("the give-up line: %q", line)
	}
}
