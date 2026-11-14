package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func seedWorkflows(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	seedEmbedded(fsys, dir, "Workflow", func(rel string) string { return strings.SplitN(rel, "/", 2)[0] })
}

func readOr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return "(absent)"
	}
	return string(b)
}

var (
	seedV1 = map[string]string{
		"review/steps/diff.yaml":   "diff v1",
		"review/steps/review.yaml": "review v1, requires diff",
		"review/steps/trim.yaml":   "trim v1, requires diff",
		"notes/steps/log.yaml":     "log v1",
	}
	// The release folds diff into another workflow: review and trim change, diff goes, change arrives.
	seedV2 = map[string]string{
		"review/steps/review.yaml": "review v2, requires change",
		"review/steps/trim.yaml":   "trim v2, requires change",
		"change/steps/diff.yaml":   "diff v2",
		"notes/steps/log.yaml":     "log v2",
	}
)

// A workflow a person edited is kept whole when the built-in changes: the step they edited, the steps
// they did not, and the step the new version dropped. Half of each is a DAG that does not load.
func TestSeedKeepsAnEditedWorkflowWhole(t *testing.T) {
	dir := t.TempDir()
	seedWorkflows(t, dir, seedV1)
	edited := filepath.Join(dir, "review", "steps", "review.yaml")
	if err := os.WriteFile(edited, []byte("review v1, requires diff, budget 123456"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedWorkflows(t, dir, seedV2)
	for file, want := range map[string]string{
		"review/steps/review.yaml": "review v1, requires diff, budget 123456",
		"review/steps/trim.yaml":   "trim v1, requires diff",
		"review/steps/diff.yaml":   "diff v1",
		"notes/steps/log.yaml":     "log v2",
		"change/steps/diff.yaml":   "diff v2",
	} {
		if got := readOr(t, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
	// A second boot of the same release changes nothing.
	seedWorkflows(t, dir, seedV2)
	if got := readOr(t, filepath.Join(dir, "review", "steps", "diff.yaml")); got != "diff v1" {
		t.Errorf("a second seed removed the kept step: %q", got)
	}
}

// A workflow nobody edited follows the built-in: changed steps refreshed, a dropped step retired.
func TestSeedStillUpdatesAnUneditedWorkflow(t *testing.T) {
	dir := t.TempDir()
	seedWorkflows(t, dir, seedV1)
	seedWorkflows(t, dir, seedV2)
	for file, want := range map[string]string{
		"review/steps/review.yaml": "review v2, requires change",
		"review/steps/trim.yaml":   "trim v2, requires change",
		"review/steps/diff.yaml":   "(absent)",
		"change/steps/diff.yaml":   "diff v2",
	} {
		if got := readOr(t, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
}

// Taking the built-in back: with the edited copy moved away, the next boot writes the new one whole.
func TestSeedWritesTheBuiltInAgainOnceTheEditedCopyIsMovedAway(t *testing.T) {
	dir := t.TempDir()
	seedWorkflows(t, dir, seedV1)
	os.WriteFile(filepath.Join(dir, "review", "steps", "review.yaml"), []byte("mine"), 0o644)
	seedWorkflows(t, dir, seedV2)
	if err := os.Rename(filepath.Join(dir, "review"), filepath.Join(t.TempDir(), "review-old")); err != nil {
		t.Fatal(err)
	}
	seedWorkflows(t, dir, seedV2)
	for file, want := range map[string]string{
		"review/steps/review.yaml": "review v2, requires change",
		"review/steps/trim.yaml":   "trim v2, requires change",
		"review/steps/diff.yaml":   "(absent)",
	} {
		if got := readOr(t, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
}
