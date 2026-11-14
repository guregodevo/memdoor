package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// A run's report: what a person keeps from a run of team-solve, read off the
// run's folder (.memdoor/runs/<workflow>-<partition>/) with no gateway: the
// task, which attempt the pick took, the review and the judge with their
// reasons, what the Check printed, keep's line (whether the team beat its
// own single call) and the diff. `workflow runs` lists the runs of this
// project with their end; `workflow report [run]` prints one; a run followed
// with --wait prints it as it ends (#175).

type runReport struct {
	Dir      string // the run's folder
	Name     string // its folder name, <workflow>-<partition>
	Started  time.Time
	Prompt   string
	Pick     string // pick.txt's line, when the run had attempts to pick from
	Statuses map[string]string
	Review   string
	Verdict  string
	Keep     string
	Check    string // the last Check log's tail
	Diff     string
}

// runFolders lists .memdoor/runs/* of a project, newest first.
func runFolders(project string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(project, ".memdoor", "runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		// A run's folder holds its files (task.md, keep.txt, the logs). .memdoor/runs/<workflow>/ is not
		// one: it is the tree the engine keeps each step's output in, <group>/<task>/<partition>, and
		// holds only directories. Written to last, it used to sort first and the report was empty.
		if dir := filepath.Join(project, ".memdoor", "runs", e.Name()); e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !onlyDirectories(dir) {
			dirs = append(dirs, dir)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return runStarted(dirs[i]).After(runStarted(dirs[j])) })
	return dirs, nil
}

// onlyDirectories says a folder holds something and none of it is a file: the engine's step-output tree.
// An empty folder is a run that has written nothing yet.
func onlyDirectories(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return false
		}
	}
	return true
}

// runStarted reads the partition off the folder name (<workflow>-2006-01-02T150405),
// else the folder's modification time.
func runStarted(dir string) time.Time {
	name := filepath.Base(dir)
	if i := strings.LastIndex(name, "-20"); i >= 0 {
		if t, err := time.ParseInLocation("2006-01-02T150405", name[i+1:], time.Local); err == nil {
			return t
		}
	}
	if st, err := os.Stat(dir); err == nil {
		return st.ModTime()
	}
	return time.Time{}
}

func readFile(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(b), "\n")
}

// section is the text under "## <name>" in a task file, up to the next heading.
func section(text, name string) string {
	var out []string
	in := false
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = strings.TrimSpace(strings.TrimPrefix(l, "## ")) == name
			continue
		}
		if in && !strings.HasPrefix(l, "<!--") {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func readRunReport(dir string) runReport {
	r := runReport{Dir: dir, Name: filepath.Base(dir), Started: runStarted(dir), Statuses: map[string]string{}}
	r.Prompt = section(readFile(dir, "task.md"), "Prompt")
	if r.Prompt == "" {
		r.Prompt = readFile(dir, "prompt.md")
	}
	r.Pick = readFile(dir, "pick.txt")
	for _, f := range []string{"direct-status.txt", "test-status.txt", "final-status.txt", "check-kind.txt"} {
		if v := readFile(dir, f); v != "" {
			r.Statuses[strings.TrimSuffix(f, ".txt")] = strings.TrimSpace(v)
		}
	}
	r.Review = readFile(dir, "review.md")
	r.Verdict = readFile(dir, "verdict.md")
	r.Keep = readFile(dir, "keep.txt")
	for _, f := range []string{"final-check.log", "test-output.log", "direct-check.log"} {
		if v := readFile(dir, f); v != "" {
			r.Check = tailLines(v, 20)
			break
		}
	}
	r.Diff = readFile(dir, "changes.diff")
	return r
}

// verdictLine is the judge's verdict when it ran, else the review's, else none.
func (r runReport) verdictLine() string {
	for _, v := range []string{r.Verdict, r.Review} {
		if first := strings.SplitN(v, "\n", 2)[0]; strings.HasPrefix(first, "verdict: ") {
			return strings.TrimPrefix(first, "verdict: ")
		}
	}
	return "none"
}

// line is the run in one line, for the list.
func (r runReport) line() string {
	keep := strings.SplitN(r.Keep, " (", 2)[0]
	if keep == "" {
		keep = "no keep: the run did not reach it"
	}
	files := strings.Count(r.Diff, "\ndiff --git ")
	if strings.HasPrefix(r.Diff, "diff --git ") {
		files++
	}
	return fmt.Sprintf("%s  %-19s  verdict %-5s %s · %d file(s) changed", r.Name, r.Started.Format("2006-01-02 15:04:05"), r.verdictLine(), keep, files)
}

func (r runReport) write(w io.Writer) {
	fmt.Fprintf(w, "# %s, %s\n\n", r.Name, r.Started.Format("2006-01-02 15:04:05"))
	if r.Prompt != "" {
		fmt.Fprintf(w, "## The task\n\n%s\n\n", r.Prompt)
	}
	if r.Pick != "" {
		fmt.Fprintf(w, "## The attempts\n\n%s\n\n", r.Pick)
	}
	if len(r.Statuses) > 0 {
		keys := make([]string, 0, len(r.Statuses))
		for k := range r.Statuses {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(w, "## The Check\n\n")
		for _, k := range keys {
			fmt.Fprintf(w, "%s: %s\n", k, r.Statuses[k])
		}
		if r.Check != "" {
			fmt.Fprintf(w, "\n```\n%s\n```\n", r.Check)
		}
		fmt.Fprintln(w)
	}
	if r.Review != "" {
		fmt.Fprintf(w, "## The review\n\n%s\n\n", r.Review)
	}
	if r.Verdict != "" {
		fmt.Fprintf(w, "## The judge\n\n%s\n\n", r.Verdict)
	}
	if r.Keep != "" {
		fmt.Fprintf(w, "## What stayed\n\n%s\n\n", r.Keep)
	}
	if r.Diff != "" {
		fmt.Fprintf(w, "## The diff\n\n```diff\n%s\n```\n", r.Diff)
	}
}

var workflowRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "The runs of this project, newest first, each with its end: the verdict, what stayed, the files changed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(projectDirFlag)
		if err != nil {
			return err
		}
		dirs, err := runFolders(dir)
		if err != nil {
			return err
		}
		if len(dirs) == 0 {
			fmt.Println("No runs in this project yet (.memdoor/runs/).")
			return nil
		}
		for _, d := range dirs {
			fmt.Println(readRunReport(d).line())
		}
		fmt.Println("\n  memdoor workflow report [run]      the report of one, the newest without a name")
		return nil
	},
}

var workflowReportCmd = &cobra.Command{
	Use:   "report [run]",
	Short: "A run's report, read off its folder: the task, the attempts, the Check, the review, the judge, what stayed, the diff",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(projectDirFlag)
		if err != nil {
			return err
		}
		var run string
		if len(args) == 1 {
			run = filepath.Join(dir, ".memdoor", "runs", filepath.Base(args[0]))
			if _, err := os.Stat(run); err != nil {
				return fmt.Errorf("no run %s in this project: memdoor workflow runs lists them", args[0])
			}
		} else {
			dirs, err := runFolders(dir)
			if err != nil {
				return err
			}
			if len(dirs) == 0 {
				return fmt.Errorf("no runs in this project yet (.memdoor/runs/)")
			}
			run = dirs[0]
		}
		readRunReport(run).write(os.Stdout)
		return nil
	},
}

// reportRunEnd prints a run's report on stderr as --wait ends, when the run
// left a folder (team-solve does): the person who followed it from a shell
// sees the verdict and what stayed, not only the exit code.
func reportRunEnd(st workflowRunStatus) {
	if st.Dir == "" || st.Partition == "" {
		return
	}
	run := filepath.Join(st.Dir, ".memdoor", "runs", st.Workflow+"-"+st.Partition)
	if _, err := os.Stat(run); err != nil {
		return
	}
	r := readRunReport(run)
	r.Diff = "" // the diff is the run's own output, on stdout already
	fmt.Fprintln(os.Stderr)
	r.write(os.Stderr)
	fmt.Fprintf(os.Stderr, "memdoor workflow report %s      the report again, with the diff\n", r.Name)
}

func init() {
	workflowCmd.AddCommand(workflowRunsCmd, workflowReportCmd)
}
