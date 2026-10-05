package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"memdoor/pkg/decision"
	"memdoor/pkg/savings"
)

// jgrep — grep, then judged. The deterministic half finds every match and
// groups it into small hunks with context; the decision model answers one
// boolean per hunk ("does this matter for the task?") in batched calls; the
// agent gets the few hunks that matter, ranked, with their probabilities,
// instead of reading every match and spending a turn on the noise.
//
// When the decision model is unavailable jgrep degrades to plain grep (the
// first matches, unranked) and says so: a missing model costs precision,
// never the search.

const (
	JgrepName          = "jgrep"
	jgrepMaxCandidates = 192  // hunks judged per call (8 parallel batches)
	jgrepMaxCollected  = 4000 // hunks collected before pre-ranking; more means the pattern is too broad
	jgrepKeepDefault   = 10
	jgrepContext       = 3
	jgrepKeepThreshold = 0.5
)

type JgrepInput struct {
	Task    string `json:"task" jsonschema_description:"What you are trying to do, one or two sentences. Relevance is judged against this, so be specific (e.g. 'find where a paused turn is resumed after a gateway restart')."`
	Pattern string `json:"pattern" jsonschema_description:"Regular expression to search for (Go RE2). Broad is fine: jgrep filters the matches."`
	Path    string `json:"path,omitempty" jsonschema_description:"Directory or file to search, relative to your working directory. Default: the working directory."`
	Glob    string `json:"glob,omitempty" jsonschema_description:"Filename glob to restrict the search (e.g. '*.go')."`
	I       bool   `json:"i,omitempty" jsonschema_description:"Case-insensitive match."`
	Keep    int    `json:"keep,omitempty" jsonschema_description:"Most hunks to return (default 10)."`
}

var JgrepInputSchema = GenerateSchema[JgrepInput]()

var JgrepDefinition = ToolDefinition{
	Name: JgrepName,
	Description: "Search code or text like grep, but get back only the matches that matter for your task, ranked, with a " +
		"relevance probability each. Use it instead of grep when a pattern is common (a function name, an error string, " +
		"a config key) and you would otherwise read dozens of matches. Give the task in one or two specific sentences. " +
		"Returns each kept hunk as file:line with a few lines of context, plus how many candidates were dropped.",
	InputSchema: JgrepInputSchema,
	Function:    Jgrep,
}

// Hunk is one match (or run of nearby matches) with its context window.
type Hunk struct {
	File      string  `json:"file"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Text      string  `json:"text"`
	P         float64 `json:"p"`
}

// Jgrep is the tool function. Like grep it runs in the agent's real working
// directory, so it behaves the same for any agent whose palette lists it.
func Jgrep(input json.RawMessage) (string, error) {
	var in JgrepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	res, err := RunJgrep(context.Background(), decisionService(), in)
	if err != nil {
		return "", err
	}
	return res.Render(), nil
}

// JgrepResult is what a run found and kept.
type JgrepResult struct {
	Matched    int // hunks the pattern matched
	Candidates int // hunks judged
	Files      int
	Kept       []Hunk
	Ranked     bool   // false = decision model unavailable, Kept are plain matches
	Note       string // why unranked, or that the candidate cap was hit
	Truncated  bool
}

func (r JgrepResult) Render() string {
	var b strings.Builder
	switch {
	case r.Candidates == 0:
		return "No matches."
	case r.Ranked:
		fmt.Fprintf(&b, "jgrep: kept %d of %d candidate hunks in %d files (ranked by relevance to the task).\n", len(r.Kept), r.Candidates, r.Files)
	default:
		fmt.Fprintf(&b, "jgrep: UNRANKED (%s) — first %d of %d hunks in %d files, plain grep order.\n", r.Note, len(r.Kept), r.Candidates, r.Files)
	}
	if r.Truncated {
		fmt.Fprintf(&b, "Note: %d hunks matched; the %d closest to the task by wording were judged. Narrow the pattern, path or glob if the answer is missing.\n", r.Matched, jgrepMaxCandidates)
	}
	if r.Ranked && len(r.Kept) == 0 {
		b.WriteString("None of the matches look relevant to the task (all below 0.5). The code you want may not contain this pattern.\n")
	}
	for _, h := range r.Kept {
		if r.Ranked {
			fmt.Fprintf(&b, "\n== %s:%d-%d  (p=%.2f)\n%s", h.File, h.StartLine, h.EndLine, h.P, h.Text)
		} else {
			fmt.Fprintf(&b, "\n== %s:%d-%d\n%s", h.File, h.StartLine, h.EndLine, h.Text)
		}
	}
	return b.String()
}

// RunJgrep is the whole tool, usable from tests and other tools.
func RunJgrep(ctx context.Context, svc decision.Service, in JgrepInput) (JgrepResult, error) {
	if strings.TrimSpace(in.Pattern) == "" {
		return JgrepResult{}, fmt.Errorf("pattern is required")
	}
	if strings.TrimSpace(in.Task) == "" {
		return JgrepResult{}, fmt.Errorf("task is required: relevance is judged against it")
	}
	keep := in.Keep
	if keep <= 0 {
		keep = jgrepKeepDefault
	}
	re, err := compileGrepPattern(GrepInput{Pattern: in.Pattern, I: in.I})
	if err != nil {
		return JgrepResult{}, err
	}
	hunks, files, err := collectHunks(GrepInput{Pattern: in.Pattern, Path: in.Path, Glob: in.Glob, I: in.I}, re)
	if err != nil {
		return JgrepResult{}, err
	}
	matched := len(hunks)
	truncated := false
	if len(hunks) > jgrepMaxCandidates {
		hunks = prerank(in.Task, hunks)[:jgrepMaxCandidates]
		truncated = true
	}
	res := JgrepResult{Matched: matched, Candidates: len(hunks), Files: files, Truncated: truncated}
	if len(hunks) == 0 {
		return res, nil
	}
	if svc == nil {
		res.Note = "no decision model configured"
		res.Kept = hunks[:min(keep, len(hunks))]
		return res, nil
	}
	if err := judgeHunks(ctx, svc, in.Task, hunks); err != nil {
		res.Note = err.Error()
		res.Kept = hunks[:min(keep, len(hunks))]
		return res, nil
	}
	res.Ranked = true
	sort.SliceStable(hunks, func(i, j int) bool { return hunks[i].P > hunks[j].P })
	for _, h := range hunks {
		if h.P < jgrepKeepThreshold || len(res.Kept) >= keep {
			break
		}
		res.Kept = append(res.Kept, h)
	}
	// The receipt (pkg/savings): the judge read every candidate at its own
	// price; only these reach the model that writes code.
	judged, kept := 0, 0
	for _, h := range hunks {
		judged += len(h.Text)
	}
	for _, h := range res.Kept {
		kept += len(h.Text)
	}
	savings.Record(savings.Entry{Kind: savings.KindJudgedRead, Tool: "jgrep", RawBytes: judged, KeptBytes: kept})
	return res, nil
}

// collectHunks turns grep's matches into context hunks, merging windows that
// touch so one function with three matches is one hunk.
func collectHunks(params GrepInput, re *regexp.Regexp) ([]Hunk, int, error) {
	var hunks []Hunk
	files := 0
	err := grepWalk(params, re, func(path string, lines []string, hits []int) error {
		if len(hits) == 0 {
			return nil
		}
		files++
		hunks = append(hunks, hunksFor(path, lines, hits)...)
		if len(hunks) >= jgrepMaxCollected {
			return filepath.SkipAll
		}
		return nil
	})
	if err == filepath.SkipAll {
		err = nil
	}
	return hunks, files, err
}

func hunksFor(path string, lines []string, hits []int) []Hunk {
	var out []Hunk
	for _, i := range hits {
		lo, hi := max(0, i-jgrepContext), min(len(lines)-1, i+jgrepContext)
		if n := len(out); n > 0 && lo <= out[n-1].EndLine {
			out[n-1].EndLine = hi
			continue
		}
		out = append(out, Hunk{File: filepath.Clean(path), StartLine: lo, EndLine: hi})
	}
	for i := range out {
		var b strings.Builder
		for n := out[i].StartLine; n <= out[i].EndLine; n++ {
			fmt.Fprintf(&b, "%5d  %s\n", n+1, clipLine(lines[n]))
		}
		out[i].Text = b.String()
		out[i].StartLine++
		out[i].EndLine++
	}
	return out
}

// clipLine keeps a hunk to what a person could read. A minified bundle is a
// few enormous lines: live 2026-09-27, one hunk of web/dist's JavaScript made
// the judge answer max_tokens_exceeded, the search came back unranked, and the
// window took hundreds of KB of React. The head of such a line says what it
// is; the rest is noise to a judge and to the coder.
func clipLine(line string) string {
	if len(line) <= jgrepMaxLine {
		return line
	}
	cut := jgrepMaxLine
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return fmt.Sprintf("%s … (%d characters on this line)", line[:cut], len(line))
}

// jgrepMaxLine is a long line of hand-written code, well past any sane width.
const jgrepMaxLine = 400

// prerank orders hunks by how many distinct task words (and their stems)
// appear in the hunk's path and text, rarer words weighing more. It only
// decides WHICH hunks the model judges when there are too many; the model
// decides relevance.
func prerank(task string, hunks []Hunk) []Hunk {
	words := taskWords(task)
	df := map[string]int{}
	lower := make([]string, len(hunks))
	for i, h := range hunks {
		lower[i] = strings.ToLower(h.File + "\n" + h.Text)
		for _, w := range words {
			if strings.Contains(lower[i], w) {
				df[w]++
			}
		}
	}
	score := make([]float64, len(hunks))
	for i := range hunks {
		for _, w := range words {
			if df[w] > 0 && strings.Contains(lower[i], w) {
				score[i] += 1 / float64(df[w])
			}
		}
	}
	idx := make([]int, len(hunks))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return score[idx[a]] > score[idx[b]] })
	out := make([]Hunk, len(hunks))
	for i, j := range idx {
		out[i] = hunks[j]
	}
	return out
}

var jgrepStop = map[string]bool{"the": true, "and": true, "where": true, "find": true, "that": true, "this": true, "with": true, "from": true, "into": true, "when": true, "what": true, "which": true, "should": true, "nobody": true, "same": true, "keeps": true}

// taskWords are the task's words of 4+ letters, minus stop words, cut to a
// 5-letter stem so "announced" finds "announce" and "failing" finds "fail".
func taskWords(task string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(task), func(r rune) bool { return !(r >= 'a' && r <= 'z') }) {
		if len(w) < 4 || jgrepStop[w] {
			continue
		}
		if len(w) > 5 {
			w = w[:5]
		}
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// judgeHunks fills Hunk.P through the shared relevance judge (jev_core.go).
func judgeHunks(ctx context.Context, svc decision.Service, task string, hunks []Hunk) error {
	items := make([]string, len(hunks))
	for i, h := range hunks {
		items[i] = fmt.Sprintf("%s:%d-%d\n%s", h.File, h.StartLine, h.EndLine, h.Text)
	}
	ps, err := judgeRelevance(ctx, svc, relevanceSpec{
		Task:     task,
		Framing:  "Each question shows one search hit from their codebase. Judge whether reading that code is needed to do the task.",
		Question: "Is this code relevant to the task?",
		True:     "The engineer needs to read or change this code to do the task.",
		False:    "The pattern merely appears here; this code is unrelated or incidental to the task.",
		Purpose:  "jgrep",
	}, items)
	if err != nil {
		return err
	}
	for i := range hunks {
		hunks[i].P = ps[i]
	}
	return nil
}
