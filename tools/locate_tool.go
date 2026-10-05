package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"memdoor/pkg/coding/bm25"
	"memdoor/pkg/savings"
)

// LOCATE, LANGUAGE-AGNOSTIC (Greg, 2026-09-28: "we need the locate tool
// because it always take time to read all files and search relevant files",
// "should be language agnostic"). It indexed .go files only, through a Go
// syntax graph, so in any other project it found nothing and the coder read
// its way around instead — 4 locate calls against 771 bash calls in a day.
//
// Now: every text file the repo tracks (git ls-files when it is a repo, a
// walk otherwise) is split into sections; BM25 ranks the sections against the
// query; the decision model judges the top ones against it; and locate
// returns the files that matter, each with its best section and line numbers.
// Without a decision model the BM25 ranking stands.

// LocateInput is the locate tool's input. Cwd is injected by the coder-workdir
// confinement (the repo root), never sent by the model.
type LocateInput struct {
	Query string `json:"query" jsonschema_description:"What you are looking for: the behavior, bug, feature or identifiers involved, in a sentence. Returns the files and sections of the repo where it lives."`
	File  string `json:"file,omitempty" jsonschema_description:"Optional. A file from a previous locate result: returns that file's sections ranked by the query, with line numbers."`
	Cwd   string `json:"-"`
}

var LocateInputSchema = GenerateSchema[LocateInput]()

// LocateDefinition is the agent-facing localization tool.
var LocateDefinition = ToolDefinition{
	Name: "locate",
	Description: "Find where something lives in the repository when you do not know the file — any language. Give it the behavior, " +
		"bug or feature in a sentence; it searches every source file and returns the files that matter, ranked, each with " +
		"its most relevant section and line numbers. Call it BEFORE reading files one by one or grepping around. Not needed " +
		"when the task names the file.",
	InputSchema: LocateInputSchema,
	Function:    Locate,
}

const (
	locateCandidates  = 40        // BM25 sections sent to the judge
	locateFiles       = 8         // files returned
	locateExcerpts    = 3         // sections shown with their code
	locateFileMaxSize = 512 << 10 // larger files are data, not source
	locateMaxFiles    = 20000     // a repo past this is indexed by its first files
)

// locateSection is one indexed slice of a file.
type locateSection struct {
	File       string
	Start, End int // 1-based, inclusive
	Text       string
}

// repoIndex is a repo's sections and their BM25 index, cached until any
// indexed file changes (the stamp: file count, newest mtime, total size).
type repoIndex struct {
	stamp    string
	sections []locateSection
	byID     map[string]int
	index    *bm25.Index
}

var (
	repoIndexMu sync.Mutex
	repoIndexes = map[string]*repoIndex{}
)

// repoFiles lists the repo's candidate files, relative to dir: what git
// tracks plus untracked files it does not ignore, or a walk that skips the
// usual build and dependency folders when dir is not a repository.
func repoFiles(dir string) []string {
	cmd := exec.Command("git", "-C", dir, "ls-files", "-co", "--exclude-standard")
	if out, err := cmd.Output(); err == nil {
		var files []string
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if f != "" && !locateSkipped(f) {
				files = append(files, f)
			}
		}
		return files
	}
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			n := d.Name()
			if path != dir && (strings.HasPrefix(n, ".") || n == "node_modules" || n == "vendor" || n == "dist" || n == "build" || n == "target" || n == "__pycache__") {
				return filepath.SkipDir
			}
			return nil
		}
		if !locateSkipped(rel) {
			files = append(files, rel)
		}
		return nil
	})
	return files
}

// locateSkipped drops files that are never where a change belongs: lock
// files, minified bundles, what a patch or an editor left behind, and
// media/archives by extension.
func locateSkipped(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, "-lock.json") || strings.HasSuffix(base, ".sum") ||
		strings.Contains(base, ".min.") || strings.HasSuffix(base, "~") {
		return true
	}
	switch filepath.Ext(base) {
	case ".orig", ".rej", ".bak", ".swp", ".snap":
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".tar", ".mp4", ".mov", ".mp3", ".wav",
		".woff", ".woff2", ".ttf", ".otf", ".a", ".so", ".dylib", ".exe", ".bin", ".wasm", ".cast":
		return true
	}
	return false
}

// locateTestData are the folders that hold test data in any language:
// copies of code, recorded output, snapshots. They read like the real thing,
// so they outrank it on words alone (live 2026-09-29: a fixture's old copy of
// a file came back first for a question about the file).
var locateTestData = map[string]bool{"testdata": true, "fixtures": true, "__fixtures__": true, "__snapshots__": true}

// inTestData reports whether rel sits under a test-data folder.
func inTestData(rel string) bool {
	dirs := strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/")
	for _, d := range dirs {
		if locateTestData[strings.ToLower(d)] {
			return true
		}
	}
	return false
}

// repoStamp is the cache key: cheap stats over the listed files.
func repoStamp(dir string, files []string) string {
	var newest time.Time
	var total int64
	for _, f := range files {
		if info, err := os.Stat(filepath.Join(dir, f)); err == nil {
			total += info.Size()
			if m := info.ModTime(); m.After(newest) {
				newest = m
			}
		}
	}
	return fmt.Sprintf("%d:%d:%d", len(files), newest.UnixNano(), total)
}

// indexFor returns the repo's index, rebuilt when any indexed file changed —
// so the coder locates against its own edits.
func indexFor(dir string) *repoIndex {
	files := repoFiles(dir)
	if len(files) > locateMaxFiles {
		files = files[:locateMaxFiles]
	}
	stamp := repoStamp(dir, files)
	repoIndexMu.Lock()
	defer repoIndexMu.Unlock()
	if ri, ok := repoIndexes[dir]; ok && ri.stamp == stamp {
		return ri
	}
	ri := &repoIndex{stamp: stamp, byID: map[string]int{}}
	var docs []bm25.Doc
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil || len(b) == 0 || len(b) > locateFileMaxSize || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
			continue
		}
		lines := strings.Split(string(b), "\n")
		for _, sc := range fileSections(lines) {
			sec := locateSection{File: f, Start: sc[0] + 1, End: sc[1] + 1, Text: strings.Join(lines[sc[0]:sc[1]+1], "\n")}
			id := fmt.Sprintf("%s:%d", f, sec.Start)
			ri.byID[id] = len(ri.sections)
			ri.sections = append(ri.sections, sec)
			// The path is part of the text: "auth", "login", "router" in a
			// file's name are often the best evidence of what it holds.
			docs = append(docs, bm25.Doc{ID: id, Text: f + "\n" + sec.Text})
		}
	}
	ri.index = bm25.New(docs)
	repoIndexes[dir] = ri
	return ri
}

// existingFileInQuery returns the first token in the query that names a file
// existing in dir, so locate("store.ts") drills into the real file.
func existingFileInQuery(dir, query string) string {
	for _, tok := range strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == ',' || r == '"' || r == '\'' || r == '`' ||
			r == '(' || r == ')' || r == ':' || r == '\n' || r == '\t'
	}) {
		if !strings.Contains(tok, ".") || strings.Contains(tok, "..") || strings.HasPrefix(tok, "/") {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, tok)); err == nil && !st.IsDir() {
			return tok
		}
	}
	return ""
}

// listSourceFiles returns the files directly in dir and one level down, so a
// no-match locate shows what the workspace holds instead of implying a new
// file is needed. Also used by apply_patch's missing-file message.
func listSourceFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			if e.Name() == "vendor" || e.Name() == "node_modules" {
				continue
			}
			sub, _ := os.ReadDir(filepath.Join(dir, e.Name()))
			for _, s := range sub {
				if !s.IsDir() && !locateSkipped(s.Name()) {
					out = append(out, e.Name()+"/"+s.Name())
				}
			}
			continue
		}
		if !locateSkipped(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

// Locate finds where the query lives: ranked files, each with its best
// section, or — with file set — that file's sections ranked by the query.
func Locate(input json.RawMessage) (string, error) {
	var params LocateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	// Cwd is json:"-" (kept out of the model-facing schema); read the injected value.
	var conf struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(input, &conf)
	dir := conf.Cwd
	if dir == "" {
		dir = "."
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if params.File == "" {
		params.File = existingFileInQuery(dir, params.Query)
	}
	ri := indexFor(dir)

	// Test data is where the task's code lives only when the task names the
	// file, or when nothing else matches.
	var cands, testData []int
	for _, r := range ri.index.Search(params.Query, locateCandidates*4) {
		i := ri.byID[r.ID]
		if params.File != "" && ri.sections[i].File != filepath.ToSlash(filepath.Clean(params.File)) {
			continue
		}
		if params.File == "" && inTestData(ri.sections[i].File) {
			if len(testData) < locateCandidates {
				testData = append(testData, i)
			}
			continue
		}
		cands = append(cands, i)
		if len(cands) == locateCandidates {
			break
		}
	}
	if len(cands) == 0 {
		cands = testData
	}
	if len(cands) == 0 {
		if params.File != "" {
			return fmt.Sprintf("Nothing in %s matches %q. read_file %s to see it whole.", params.File, params.Query, params.File), nil
		}
		if files := listSourceFiles(dir); len(files) > 0 {
			return "Nothing in the repository matches that query. The workspace holds:\n" + strings.Join(files, "\n") +
				"\n\nRephrase with the words the code would use, or read_file the file that fits. Only create a NEW file if none do.", nil
		}
		return "The workspace has no source files — the change needs a NEW file.", nil
	}

	// Judged against the query; the BM25 order stands when there is no
	// decision model or it cannot answer.
	scores := make([]float64, len(cands))
	for k := range cands {
		scores[k] = 1 - float64(k)/float64(len(cands)+1)
	}
	judged := false
	if svc := decisionService(); svc != nil {
		items := make([]string, len(cands))
		for k, i := range cands {
			items[k] = numberedSection(ri.sections[i])
		}
		ps, jerr := judgeRelevance(context.Background(), svc, relevanceSpec{
			Task:     params.Query,
			Framing:  "Each question shows one section of a file in the repository. Judge whether the agent needs this section to find where the task's code lives.",
			Question: "Is this section where the task's code lives, or does the agent need it for the task?",
			True:     "The section holds code or text the task is about, or that it must read or change.",
			False:    "The section is unrelated to the task.",
			Purpose:  "locate",
		}, items)
		if jerr == nil {
			scores, judged = ps, true
		}
	}

	order := make([]int, len(cands))
	for k := range order {
		order[k] = k
	}
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	if judged {
		kept := order[:0]
		for _, k := range order {
			if scores[k] >= jevKeep {
				kept = append(kept, k)
			}
		}
		order = kept
		if len(order) == 0 {
			return fmt.Sprintf("None of the %d closest sections is about %q (judged). Rephrase with the words the code would use.", len(cands), params.Query), nil
		}
	}

	var b strings.Builder
	if params.File != "" {
		fmt.Fprintf(&b, "Sections of %s for %q, most relevant first:\n", params.File, params.Query)
		for n, k := range order {
			if n == locateExcerpts {
				break
			}
			fmt.Fprintf(&b, "\n== %s (p=%.2f)\n%s", sectionRef(ri.sections[cands[k]]), scores[k], numberedSection(ri.sections[cands[k]]))
		}
		return b.String(), nil
	}

	// One line per file, at its best section; the top sections with code.
	seen := map[string]bool{}
	var files []int
	for _, k := range order {
		if f := ri.sections[cands[k]].File; !seen[f] {
			seen[f] = true
			files = append(files, k)
		}
		if len(files) == locateFiles {
			break
		}
	}
	how := "keyword-ranked"
	if judged {
		how = fmt.Sprintf("judged: %d of %d sections kept", len(order), len(cands))
	}
	fmt.Fprintf(&b, "Where %q lives (%s), most relevant first:\n", params.Query, how)
	for n, k := range files {
		fmt.Fprintf(&b, "%d. %s  p=%.2f\n", n+1, sectionRef(ri.sections[cands[k]]), scores[k])
	}
	shown, keptBytes := 0, 0
	for _, k := range order {
		if shown == locateExcerpts {
			break
		}
		sec := numberedSection(ri.sections[cands[k]])
		keptBytes += len(sec)
		fmt.Fprintf(&b, "\n== %s (p=%.2f)\n%s", sectionRef(ri.sections[cands[k]]), scores[k], sec)
		shown++
	}
	b.WriteString("\nread_file or jread a file to see more; locate again with file set to rank one file's sections.")
	if judged {
		raw := 0
		for _, i := range cands {
			raw += len(ri.sections[i].Text)
		}
		savings.Record(savings.Entry{Kind: savings.KindJudgedRead, Tool: "locate", RawBytes: raw, KeptBytes: keptBytes})
	}
	return b.String(), nil
}

func sectionRef(s locateSection) string { return fmt.Sprintf("%s:%d-%d", s.File, s.Start, s.End) }

// numberedSection is a section with its line numbers, long lines clipped.
func numberedSection(s locateSection) string {
	var b strings.Builder
	for n, l := range strings.Split(s.Text, "\n") {
		if len(l) > sectionLineMax {
			l = l[:sectionLineMax] + "…"
		}
		fmt.Fprintf(&b, "%5d  %s\n", s.Start+n, l)
	}
	return b.String()
}
