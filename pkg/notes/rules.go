package notes

import (
	"errors"
	"os"
	"strings"
	"sync"
)

// RuleBook keeps a project's lasting rules in its instructions file
// (AGENTS.md), which the agent reads at every turn and the person reads and
// reviews in git. Notes end with the conversation; a rule told once for
// later ("always run tests with -race") must not. Left to the model, the
// rule went into notes, or the model said it would edit AGENTS.md and
// ended the turn without doing it (live 2026-09-29) — so the harness writes.
type RuleBook interface {
	// Add appends rule to the instructions file at path, under
	// RulesHeading; a rule the file already holds is not added twice.
	Add(path, rule string) error
}

// RulesHeading is the section the kept rules go under.
const RulesHeading = "## Rules kept by Memdoor"

type markdownRuleBook struct{ mu sync.Mutex }

// NewMarkdownRuleBook writes rules into a Markdown instructions file.
func NewMarkdownRuleBook() RuleBook { return &markdownRuleBook{} }

func (b *markdownRuleBook) Add(path, rule string) error {
	rule = strings.TrimSpace(rule)
	if path == "" {
		return errors.New("no instructions file to keep the rule in")
	}
	if rule == "" {
		return errors.New("an empty rule")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(raw)
	if strings.Contains(text, rule) {
		return nil
	}
	return os.WriteFile(path, []byte(withRule(text, rule)), 0o644)
}

// withRule is text with rule at the end of the RulesHeading section, the
// section added at the end when the file has none.
func withRule(text, rule string) string {
	line := "- " + rule + "\n"
	at := strings.Index(text, RulesHeading)
	if at < 0 {
		if text == "" {
			return RulesHeading + "\n\n" + line
		}
		return strings.TrimRight(text, "\n") + "\n\n" + RulesHeading + "\n\n" + line
	}
	end := len(text)
	if next := strings.Index(text[at+len(RulesHeading):], "\n## "); next >= 0 {
		end = at + len(RulesHeading) + next + 1
	}
	section := strings.TrimRight(text[:end], "\n") + "\n" + line
	if end == len(text) {
		return section
	}
	return section + "\n" + text[end:]
}
