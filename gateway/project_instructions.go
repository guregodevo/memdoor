package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"memdoor/gateway/logs"
	"memdoor/gateway/prompts"
	"memdoor/pkg/savings"
	"memdoor/pkg/workflow"
	"memdoor/tools"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// A PROJECT'S INSTRUCTIONS, JUDGED AGAINST THE TASK (2026-09-28, battle test).
//
// The project's AGENTS.md went into the system prompt whole, on
// every call: this repository's is 20 KB, about 5,000 tokens, which with the
// coder's own prompt and the tool schemas made roughly 80% of what a code
// question was billed. Most of it is about work the task never touches.
//
// So a large file is split at its "## " headings and the decision model
// keeps the sections the conversation's first task needs; the other titles
// are listed, and the coder can jread one when the work turns out to need it.
// The choice is made once per session and kept, so the system prompt stays
// byte-identical from call to call and the provider's cache keeps hitting.
//
// Without a decision model (free, or decisions off) the prompt carries the
// text before the first "## " heading plus an exact outline: every "## "
// heading with its line number in the file, and the instruction to read_file
// the sections the work needs. Nothing is guessed — the model picks, from
// titles it can see. Judging only ever narrows, it never loses the
// instructions.

// projectJudgeMin is the size under which a file goes in whole: judging a
// small file saves less than it costs to ask.
const projectJudgeMin = 6000

var projectSelected sync.Map // session|path|size|mtime → rendered section

// projectRulesFile is the instructions file a kept rule goes into: the one
// projectInstructionsFor reads (the first that exists), else a new AGENTS.md.
func projectRulesFile(dir string) string {
	if dir == "" {
		return ""
	}
	for _, name := range prompts.ProjectInstructionFiles {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	// No instruction file of the project's own: keep ours under .memdoor/,
	// ignored by git, never a new file in the person's tree.
	workflow.EnsureIgnored(dir)
	return filepath.Join(dir, prompts.KeptRulesFile)
}

// projectInstructionsFor returns the judged section for this session, or ""
// when the builder should read the file whole itself.
func projectInstructionsFor(ctx context.Context, dir string) string {
	if dir == "" {
		return ""
	}
	sess, _ := ctx.Value(ctxSession).(*Session)
	task, _ := ctx.Value(turnTaskKey{}).(string)
	if sess == nil || strings.TrimSpace(task) == "" {
		return ""
	}
	for _, name := range prompts.ProjectInstructionFiles {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d|%d", sess.ID, path, info.Size(), info.ModTime().UnixNano())
		if v, ok := projectSelected.Load(key); ok {
			return v.(string)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		out := judgeProjectInstructions(ctx, name, string(raw), clipHead(task, turnTaskMax), tools.JudgeProjectSections)
		projectSelected.Store(key, out)
		return out
	}
	return ""
}

type sectionJudge func(ctx context.Context, task string, sections []string) ([]float64, error)

// judgeProjectInstructions renders the kept sections and the list of the
// others, or returns "" to fall back to the whole file. A failed judge (no
// decision model, an outage) narrows with an exact outline (outlineFor)
// instead of sending the whole file.
func judgeProjectInstructions(ctx context.Context, name, text, task string, judge sectionJudge) string {
	if len(text) < projectJudgeMin {
		return ""
	}
	preamble, sections := splitHeadings(text)
	if len(sections) < 3 {
		return ""
	}
	ps, err := judge(ctx, task, sections)
	judged := err == nil && len(ps) == len(sections)
	if !judged {
		return outlineFor(name, text)
	}
	var kept, others []string
	keptBytes := len(preamble)
	for i, s := range sections {
		if ps[i] >= 0.5 {
			kept = append(kept, strings.TrimSpace(s))
			keptBytes += len(s)
		} else {
			others = append(others, headingOf(s))
		}
	}
	if keptBytes*10 >= len(text)*7 {
		return "" // it needed nearly all of it
	}
	savings.Record(savings.Entry{Kind: savings.KindJudgedRead, Tool: "project_instructions", RawBytes: len(text), KeptBytes: keptBytes})
	var b strings.Builder
	tail := fmt.Sprintf("jread %s for one if the work turns out to need it", name)
	if !judged {
		tail = fmt.Sprintf("read %s (with read_file) if the work turns out to need one", name)
	}
	fmt.Fprintf(&b, "# Project instructions (%s)\n\nThe project you are working in wrote these for you. Follow them. "+
		"Only the sections this conversation's task needs are here; the others are listed at the end — "+
		"%s.\n\n", name, tail)
	if p := strings.TrimSpace(preamble); p != "" {
		b.WriteString(p + "\n\n")
	}
	for _, s := range kept {
		b.WriteString(s + "\n\n")
	}
	if len(others) > 0 {
		b.WriteString("Other sections of " + name + ": " + strings.Join(others, "; ") + ".")
	}
	logProjectSelection(name, kept, others)
	return strings.TrimSpace(b.String())
}

// splitHeadings cuts markdown at "## " headings outside code fences: the text
// before the first one, then one string per section.
func splitHeadings(text string) (preamble string, sections []string) {
	var cur strings.Builder
	inFence, started := false, false
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			if started {
				sections = append(sections, cur.String())
			} else {
				preamble = cur.String()
			}
			cur.Reset()
			started = true
		}
		cur.WriteString(line)
	}
	if started {
		sections = append(sections, cur.String())
	} else {
		preamble = cur.String()
	}
	return preamble, sections
}

func headingOf(section string) string {
	first, _, _ := strings.Cut(section, "\n")
	return strings.TrimSpace(strings.TrimPrefix(first, "## "))
}

// outlineFor is the exact no-decision-model fallback: the text before the
// file's first "## " heading, then every "## " heading with its 1-based line
// number, and the instruction to read_file the sections the work needs. No
// words are scored; the model picks from the outline itself.
func outlineFor(name, text string) string {
	var b strings.Builder
	preamble, sections := splitHeadings(text)
	if len(sections) == 0 {
		return ""
	}
	fmt.Fprintf(&b, "# Project instructions (%s)\n\nThe project you are working in wrote these for you. Follow them. "+
		"The headings below are this file's sections; read_file the file and read the sections the work needs.\n\n",
		name)
	if p := strings.TrimSpace(preamble); p != "" {
		b.WriteString(p + "\n\n")
	}
	line := strings.Count(preamble, "\n") + 1
	var titles []string
	for _, s := range sections {
		titles = append(titles, fmt.Sprintf("- line %d: %s", line, headingOf(s)))
		line += strings.Count(s, "\n")
	}
	b.WriteString("Sections of " + name + ":\n" + strings.Join(titles, "\n"))
	return b.String()
}

// logProjectSelection records which sections of the instructions file the
// decision model kept for the turn and which it cut, so ./memdoor logs query
// shows it and a dropped rule is never silent.
func logProjectSelection(name string, kept, others []string) {
	keptTitles := make([]string, len(kept))
	for i, s := range kept {
		keptTitles[i] = headingOf(s)
	}
	log := logs.New("ProjectInstructions")
	log.Info("Judged project instructions",
		slog.String("file", name),
		slog.Int("kept", len(kept)),
		slog.Any("kept_sections", keptTitles),
		slog.Any("cut_sections", others))
}
