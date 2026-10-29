package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// githubReportCap bounds each Markdown file in the job summary (GitHub's own
// limit is 1 MiB for the whole summary).
const githubReportCap = 200 << 10

// reportToGitHub writes a finished run into the GitHub Actions step running
// it: the job summary (the outcome, every task's state, then each Markdown
// file the run produced, as written: a review, a fix's report, the notes) and
// the step outputs run-id, exit-code and results. Outside Actions the two
// files are unset and nothing is written. A write that fails is said on
// stderr and never changes the run's exit code.
func reportToGitHub(st workflowRunStatus, code int) {
	if p := os.Getenv("GITHUB_STEP_SUMMARY"); p != "" {
		appendTo(p, func(w io.Writer) { writeGitHubSummary(w, st, code) })
	}
	if p := os.Getenv("GITHUB_OUTPUT"); p != "" {
		appendTo(p, func(w io.Writer) {
			fmt.Fprintf(w, "run-id=%s\nexit-code=%d\n", st.ID, code)
			fmt.Fprintf(w, "results<<MEMDOOR_RESULTS_EOF\n%s\nMEMDOOR_RESULTS_EOF\n", strings.Join(st.Results, "\n"))
		})
	}
}

func writeGitHubSummary(w io.Writer, st workflowRunStatus, code int) {
	fmt.Fprintf(w, "### Memdoor workflow `%s`\n", st.Workflow)
	fmt.Fprintln(w, map[int]string{
		workflowExitDone:    "✓ done",
		workflowExitStopped: "■ stopped",
		workflowExitFailed:  "✗ failed",
		workflowExitGate:    "⏸ stopped at a gate: nobody in this job can approve it",
	}[code])
	fmt.Fprintln(w, "```")
	fprintWorkflowStatus(w, st)
	fmt.Fprintln(w, "```")
	for _, f := range st.Results {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		if !filepath.IsAbs(f) && st.Dir != "" {
			f = filepath.Join(st.Dir, f)
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if len(b) > githubReportCap {
			b = b[:githubReportCap]
		}
		fmt.Fprintf(w, "\n---\n\n%s\n", b)
	}
}

func appendTo(path string, write func(io.Writer)) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "  GitHub step report:", err)
		return
	}
	defer f.Close()
	write(f)
}
