package gateway

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/savings"
)

// A large instruction file keeps the sections the task needs and lists the
// rest; a small one, or a judge that keeps nearly everything, falls back to
// the whole file (""), so judging only ever narrows. A failed judge is the
// exact outline (the next test).
func TestProjectInstructionsAreJudged(t *testing.T) {
	savings.SetLedger(filepath.Join(t.TempDir(), "savings.jsonl")) // never the person's own receipt
	t.Cleanup(func() { savings.SetLedger("") })
	var b strings.Builder
	b.WriteString("# Project\n\nIntro line.\n\n")
	for _, h := range []string{"Build", "Testing", "Deploy", "Billing", "Style"} {
		b.WriteString("## " + h + "\n\n" + strings.Repeat(h+" detail. ", 180) + "\n\n```sh\n## not a heading inside a fence\n```\n\n")
	}
	text := b.String()
	keepTesting := func(_ context.Context, _ string, secs []string) ([]float64, error) {
		ps := make([]float64, len(secs))
		for i, s := range secs {
			if strings.HasPrefix(s, "## Testing") {
				ps[i] = 0.9
			}
		}
		return ps, nil
	}
	out := judgeProjectInstructions(context.Background(), "CLAUDE.md", text, "add a test", keepTesting)
	if !strings.Contains(out, "## Testing") || strings.Contains(out, "Billing detail") {
		t.Fatalf("must keep Testing and drop Billing:\n%.400s", out)
	}
	if !strings.Contains(out, "Other sections of CLAUDE.md: Build; Deploy; Billing; Style.") {
		t.Fatalf("the dropped titles must be listed:\n%s", out[len(out)-200:])
	}
	if !strings.Contains(out, "Intro line.") {
		t.Fatal("the preamble always stays")
	}
	if len(out) > len(text)/3 {
		t.Fatalf("expected a real cut, kept %d of %d", len(out), len(text))
	}

	all := func(_ context.Context, _ string, s []string) ([]float64, error) {
		ps := make([]float64, len(s))
		for i := range ps {
			ps[i] = 0.9
		}
		return ps, nil
	}
	for name, got := range map[string]string{
		"small file":      judgeProjectInstructions(context.Background(), "CLAUDE.md", "# tiny\n## a\nx\n## b\ny\n## c\nz", "t", keepTesting),
		"kept nearly all": judgeProjectInstructions(context.Background(), "CLAUDE.md", text, "t", all),
	} {
		if got != "" {
			t.Errorf("%s must fall back to the whole file", name)
		}
	}
}

func TestProjectInstructionsOutlineWhenNoJudge(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Project\n\nIntro line.\n\n")
	pad := strings.Repeat("rules detail. ", 300) // long single line, no extra headings
	b.WriteString("## Build\n\nbuild rules. " + pad + "\n\n## Testing\n\ntest rules. " + pad + "\n\n## Deploy\n\ndeploy rules. " + pad + "\n")
	text := b.String()
	fails := func(context.Context, string, []string) ([]float64, error) {
		return nil, errors.New("no decision model")
	}
	out := judgeProjectInstructions(context.Background(), "AGENTS.md", text, "any task at all", fails)
	if !strings.Contains(out, "Intro line.") {
		t.Fatal("the preamble stays:\n" + out)
	}
	if !strings.Contains(out, "- line 5: Build") || !strings.Contains(out, "- line 9: Testing") || !strings.Contains(out, "- line 13: Deploy") {
		t.Fatalf("every heading with its line number:\n%s", out)
	}
	if !strings.Contains(out, "read_file") {
		t.Fatal("the model must be told to read_file the sections:\n" + out)
	}
	if strings.Contains(out, "build rules.") {
		t.Fatal("no section body travels in the outline:\n" + out)
	}
	// A fenced block's "## " is not a heading, and a fence straddling lines
	// still counts the line numbers right.
	fenced := "# Project\n\n## Real\n\n" + pad + "\n\n## Middle\n\n" + pad + "\n\n```sh\n## not a heading\n```\n\n## After\n\n" + pad + "\n"
	out = judgeProjectInstructions(context.Background(), "AGENTS.md", fenced, "t", fails)
	if strings.Contains(out, "not a heading") {
		t.Fatal("a heading inside a fence is not a section:\n" + out)
	}
	if !strings.Contains(out, "- line 3: Real") || !strings.Contains(out, "- line 15: After") {
		t.Fatalf("line numbers hold across fences:\n%s", out)
	}
}

func TestOutlineForLineNumbers(t *testing.T) {
	text := "# Project\n\nintro\n\n## Alpha\n\nalpha body\n\n## Beta\n\nbeta body\n"
	out := outlineFor("AGENTS.md", text)
	if !strings.Contains(out, "- line 5: Alpha") || !strings.Contains(out, "- line 9: Beta") {
		t.Fatalf("line numbers: %s", out)
	}
	if !strings.Contains(out, "intro") {
		t.Fatalf("preamble: %s", out)
	}
	if _, sections := splitHeadings(text); len(sections) < 2 {
		t.Fatal("splitHeadings must find the sections")
	}
}
