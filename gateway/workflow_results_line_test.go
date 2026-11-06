package gateway

import "testing"

// A run's files in one folder are named once with it; files in several
// folders keep their paths.
func TestTheResultsLineNamesTheFolderOnce(t *testing.T) {
	for _, c := range []struct {
		files []string
		want  string
	}{
		{[]string{".memdoor/runs/team-solve-p/context.md", ".memdoor/runs/team-solve-p/review.md"}, "results in .memdoor/runs/team-solve-p/: context.md · review.md"},
		{[]string{"reports/a.json", "FINAL-p1.md"}, "results: reports/a.json · FINAL-p1.md"},
		{[]string{"a.md", "b.md"}, "results: a.md · b.md"},
	} {
		if got := resultsLine(c.files); got != c.want {
			t.Errorf("%v: %q, want %q", c.files, got, c.want)
		}
	}
}
