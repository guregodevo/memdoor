package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/savings"
)

// jlogs, jread — the decision model behind tools agents already use. Each
// reuses an existing source (agent_log's query, a file read) and returns only
// what the relevance judge keeps, with probabilities; each degrades to its
// unjudged output when the decision model is unavailable.

const jevMaxItems = 400

// ---- jlogs ------------------------------------------------------------------

type JlogsInput struct {
	Task string `json:"task" jsonschema_description:"What you are investigating, one or two specific sentences (e.g. 'why did the coder's last turn stop without a reply')."`
	AgentLogParams
	Keep int `json:"keep,omitempty" jsonschema_description:"Most events to return (default 20)."`
}

var JlogsDefinition = ToolDefinition{
	Name: "jlogs",
	Description: "Query the gateway's structured logs like agent_log, but get back only the events that matter for what you " +
		"are investigating, in time order, each with a relevance probability. Use it instead of agent_log when a window " +
		"holds more than a screenful of events. Filters (since, level, component, regex, session) narrow the candidates first.",
	InputSchema: GenerateSchema[JlogsInput](),
	Function:    Jlogs,
}

func Jlogs(input json.RawMessage) (string, error) {
	var in JlogsInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(in.Task) == "" {
		return "", fmt.Errorf("task is required: relevance is judged against it")
	}
	lt, err := NewAgentLogTool("")
	if err != nil {
		return "", err
	}
	limit := jevMaxItems
	if in.Limit == nil || *in.Limit < limit {
		in.Limit = &limit
	}
	evs, err := lt.executeQuery(context.Background(), in.AgentLogParams)
	if err != nil {
		return "", err
	}
	if len(evs) == 0 {
		return "No log events match the filters.", nil
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].Timestamp.Before(evs[j].Timestamp) })
	items := make([]string, len(evs))
	for i, e := range evs {
		items[i] = renderLogEvent(e)
	}
	keep := in.Keep
	if keep <= 0 {
		keep = 20
	}
	ps, jerr := judgeRelevance(context.Background(), decisionService(), relevanceSpec{
		Task:     in.Task,
		Framing:  "Each question shows one event from the system's logs. Judge whether it helps answer the investigation.",
		Question: "Does this log event matter for the investigation?",
		True:     "It shows the cause, a symptom, or a step of what is being investigated.",
		False:    "It is routine activity unrelated to the investigation.",
		Purpose:  "jlogs",
	}, items)
	out, kept := renderJudged(fmt.Sprintf("%d log events", len(items)), items, ps, jerr, keep)
	if kept != nil {
		raw, keptBytes := 0, 0
		for _, it := range items {
			raw += len(it)
		}
		for _, i := range kept {
			keptBytes += len(items[i])
		}
		savings.Record(savings.Entry{Kind: savings.KindJudgedRead, Tool: "jlogs", RawBytes: raw, KeptBytes: keptBytes})
	}
	return out, nil
}

// A rendered event is capped: judged jlogs output averaged 11 KB and reached
// 79 KB (2026-09-26) because single events carry a whole system-prompt head
// in the message and a dozen keys. The judge picks the right events; the cap
// keeps each one to what identifies it.
const (
	logEventMsgMax  = 300
	logEventLineMax = 800
)

func renderLogEvent(e *logs.Event) string {
	var b strings.Builder
	msg := e.Message
	if len(msg) > logEventMsgMax {
		msg = msg[:logEventMsgMax] + "…"
	}
	fmt.Fprintf(&b, "%s %s [%s] %s", e.Timestamp.Format("15:04:05.000"), e.Level, e.Component, msg)
	keys := make([]string, 0, len(e.Data))
	for k := range e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fmt.Sprint(e.Data[k])
		if len(v) > 200 {
			v = v[:200] + "…"
		}
		fmt.Fprintf(&b, " %s=%s", k, v)
	}
	if e.Error != nil {
		fmt.Fprintf(&b, " error=%s", e.Error.Message)
	}
	out := b.String()
	if len(out) > logEventLineMax {
		out = out[:logEventLineMax] + "…"
	}
	return out
}

// ---- jread ------------------------------------------------------------------

type JreadInput struct {
	Task string `json:"task" jsonschema_description:"What you need from the file, one or two specific sentences."`
	Path string `json:"path" jsonschema_description:"File to read, relative to your working directory or absolute."`
	Keep int    `json:"keep,omitempty" jsonschema_description:"Most sections to return (default 8)."`
}

var JreadDefinition = ToolDefinition{
	Name: "jread",
	Description: "Read a long file (code, transcript, document, config) and get back only the sections that matter for your " +
		"task, with line numbers and a relevance probability each, in file order. Use it instead of read_file on files " +
		"of more than a few hundred lines; use read_file when you need the whole file or know the lines.",
	InputSchema: GenerateSchema[JreadInput](),
	Function:    Jread,
}

const jreadSectionLines = 40

func Jread(input json.RawMessage) (string, error) {
	var in JreadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(in.Task) == "" || strings.TrimSpace(in.Path) == "" {
		return "", fmt.Errorf("task and path are required")
	}
	raw, err := os.ReadFile(in.Path)
	if err != nil {
		return "", err
	}
	keep := in.Keep
	if keep <= 0 {
		keep = 8
	}
	out, err := judgedSections("jread", in.Path, string(raw), in.Task, keep)
	if errors.Is(err, errJudgeNoSaving) {
		// Judging kept nearly all of it: the file is the better answer, whole
		// and exact, and it costs no more than the sections would have.
		return fmt.Sprintf("%s (%d bytes) — the task needs nearly all of it, so here it is whole:\n%s",
			in.Path, len(raw), string(raw)), nil
	}
	return out, err
}

// errJudgeNoSaving says the judged sections cover so much of the text that
// returning them saves nothing and costs fidelity. Every caller answers it
// with the whole text (read_file and web_fetch already fall back on any
// error): a judged read is only worth it when it actually removes
// bytes, and a partial view of a file the turn is about to patch is worse
// than the file (measured 2026-09-27: a 4 KB edit re-read the file it had
// already been given).
var errJudgeNoSaving = errors.New("judging kept nearly all of it")

// judgedSections splits text into sections and returns the ones the task
// needs, in file order with line numbers. Shared by jread and read_file.
func judgedSections(tool, path, text, task string, keep int) (string, error) {
	lines := strings.Split(text, "\n")
	secs := fileSections(lines)
	// A file read shows the numbers an edit names (hashline.go).
	anchored := HashlineOn() && (tool == "read_file" || tool == "jread")
	cut := map[int]bool{}
	if len(secs) > jevMaxItems {
		return "", fmt.Errorf("%s has %d sections; too long to judge whole — use jgrep on it instead", path, len(secs))
	}
	items := make([]string, len(secs))
	for i, sc := range secs {
		var b strings.Builder
		fmt.Fprintf(&b, "%s:%d-%d\n", path, sc[0]+1, sc[1]+1)
		for n := sc[0]; n <= sc[1]; n++ {
			l := lines[n]
			if len(l) > sectionLineMax {
				l = l[:sectionLineMax] + "…"
				cut[n] = true
			}
			if anchored {
				fmt.Fprintf(&b, "%d:%s\n", n+1, l)
				continue
			}
			fmt.Fprintf(&b, "%5d  %s\n", n+1, l)
		}
		items[i] = b.String()
	}
	ps, jerr := judgeRelevance(context.Background(), decisionService(), relevanceSpec{
		Task:     task,
		Framing:  "Each question shows one section of a file. Judge whether the agent needs to read it for the task.",
		Question: "Does the agent need this section for the task?",
		True:     "The section holds something the task needs to read, use or change.",
		False:    "The section is unrelated to the task.",
		Purpose:  "jread",
	}, items)
	// keep is the caller's cap, unchanged: a tighter cap was tried on
	// 2026-09-27 and dropped — it would have dropped sections the judge had
	// already scored above the bar, and the day's saving came from elsewhere.
	out, kept := renderJudged(fmt.Sprintf("%d sections of %s (%d lines)", len(items), path, len(lines)),
		items, ps, jerr, keep)
	if kept != nil {
		lk := 0
		keptBytes := 0
		for _, i := range kept {
			lk += secs[i][1] - secs[i][0] + 1
			keptBytes += len(items[i])
		}
		if lk*10 >= len(lines)*7 {
			return "", errJudgeNoSaving
		}
		if anchored {
			all := addressableLines(text)
			seen := make([]bool, len(all))
			for _, i := range kept {
				for n := secs[i][0]; n <= secs[i][1] && n < len(seen); n++ {
					seen[n] = !cut[n]
				}
			}
			tag := FileTag(text)
			recordSnapshot(path, tag, all, seen)
			out = hashlineHeader(path, tag) + "\n" + out
		}
		// The receipt (pkg/savings): the judge read the whole text; the model
		// that writes code reads the sections it needs.
		savings.Record(savings.Entry{Kind: savings.KindJudgedRead, Tool: tool, RawBytes: len(text), KeptBytes: keptBytes})
	}
	return out, nil
}

// sectionLineMax cuts a single line inside a judged section (minified data,
// base64) so one line cannot carry a whole file.
const sectionLineMax = 1000

// fileSections splits at blank lines into blocks, merging small blocks and
// cutting long ones so each section is at most jreadSectionLines lines.
// Returns 0-based [start, end] pairs.
func fileSections(lines []string) [][2]int {
	var out [][2]int
	start := -1
	flush := func(end int) {
		if start >= 0 && end >= start {
			for s := start; s <= end; s += jreadSectionLines {
				out = append(out, [2]int{s, min(end, s+jreadSectionLines-1)})
			}
		}
		start = -1
	}
	for i, l := range lines {
		blank := strings.TrimSpace(l) == ""
		switch {
		case !blank && start < 0:
			start = i
		case blank && start >= 0:
			if n := len(out); n > 0 && i-1-out[n-1][0] < jreadSectionLines && start-out[n-1][1] <= 2 && i-1-start < jreadSectionLines/2 {
				out[n-1][1] = i - 1 // merge a short block into the previous section
				start = -1
				continue
			}
			flush(i - 1)
		}
	}
	flush(len(lines) - 1)
	return out
}

// renderJudged is the one output shape of the jev tools: the kept items with
// their probability, the best `keep` of them in source order, or the
// unjudged head of the list with the reason when judging was unavailable.
// It also returns the kept indices, for a caller that needs to know how much
// of the source survived.
func renderJudged(what string, items []string, ps []float64, jerr error, keep int) (string, []int) {
	var b strings.Builder
	if jerr != nil {
		fmt.Fprintf(&b, "UNJUDGED (%s) — first %d of %d:\n", jerr, min(keep, len(items)), len(items))
		for _, it := range items[:min(keep, len(items))] {
			fmt.Fprintf(&b, "\n%s", strings.TrimRight(it, "\n"))
		}
		return b.String(), nil
	}
	idx := make([]int, 0, len(items))
	for i := range items {
		if ps[i] >= jevKeep {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, c int) bool { return ps[idx[a]] > ps[idx[c]] })
	if len(idx) > keep {
		idx = idx[:keep]
	}
	sort.Ints(idx)
	fmt.Fprintf(&b, "kept %d of %s:\n", len(idx), what)
	if len(idx) == 0 {
		b.WriteString("\nNothing matched (all below 0.5).")
	}
	for _, i := range idx {
		fmt.Fprintf(&b, "\n(p=%.2f) %s", ps[i], strings.TrimRight(items[i], "\n"))
	}
	return b.String(), idx
}
