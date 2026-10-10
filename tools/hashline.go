package tools

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pmezard/go-difflib/difflib"

	"memdoor/pkg/llm"
)

// Line-anchored edits ("hashline"): docs/features/HASHLINE.md.
//
// A read shows the file's tag and numbered lines; an edit names line ranges
// of that read instead of re-typing the code around them. Of the bytes in
// 176 update patches, 35% re-typed existing code, and 19% of apply_patch
// calls failed, mostly on re-typed context that did not match (2026-09-29).

// EditFormatHashline is the edit_format workspace setting that turns it on.
const EditFormatHashline = "hashline"

var editFormat func() string

// SetEditFormat wires the edit_format setting (read per call).
func SetEditFormat(fn func() string) { editFormat = fn }

// HashlineOn reports whether reads are numbered and tagged, and apply_patch
// describes line-anchored edits first. Edits of either form are always
// accepted.
func HashlineOn() bool {
	return editFormat != nil && strings.EqualFold(strings.TrimSpace(editFormat()), EditFormatHashline)
}

// FileTag is a file's 4-hex tag: FNV-1a over its lines with trailing
// whitespace and \r ignored.
func FileTag(text string) string {
	h := fnv.New32a()
	for i, line := range addressableLines(text) {
		if i > 0 {
			_, _ = h.Write([]byte{'\n'})
		}
		_, _ = h.Write([]byte(strings.TrimRight(line, " \t\r")))
	}
	return fmt.Sprintf("%04X", h.Sum32()&0xffff)
}

// addressableLines splits text into the lines an edit can name: a final
// newline ends the last line, it does not start an empty one.
func addressableLines(text string) []string {
	lines, _ := splitFileLines(text)
	return lines
}

func hashlineHeader(path, tag string) string { return "[" + path + "#" + tag + "]" }

// ---- snapshots ------------------------------------------------------------

// snapshot is a file as a read showed it: which lines the model saw.
type snapshot struct {
	lines []string
	seen  []bool
}

const snapshotsKept = 256

var snapshots = struct {
	sync.Mutex
	m     map[string]*snapshot
	order []string
}{m: map[string]*snapshot{}}

func snapshotKey(path, tag string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path + "#" + strings.ToUpper(tag)
}

// recordSnapshot keeps what a read showed; seen nil means every line.
func recordSnapshot(path, tag string, lines []string, seen []bool) {
	if seen == nil {
		seen = make([]bool, len(lines))
		for i := range seen {
			seen[i] = true
		}
	}
	key := snapshotKey(path, tag)
	snapshots.Lock()
	defer snapshots.Unlock()
	if prev, ok := snapshots.m[key]; ok {
		for i := range prev.seen { // a second read of the same text adds what it showed
			if i < len(seen) && seen[i] {
				prev.seen[i] = true
			}
		}
		return
	}
	snapshots.m[key] = &snapshot{lines: lines, seen: seen}
	snapshots.order = append(snapshots.order, key)
	if len(snapshots.order) > snapshotsKept {
		delete(snapshots.m, snapshots.order[0])
		snapshots.order = snapshots.order[1:]
	}
}

func lookupSnapshot(path, tag string) *snapshot {
	snapshots.Lock()
	defer snapshots.Unlock()
	return snapshots.m[snapshotKey(path, tag)]
}

// hashlineRead is a whole (or cut) file as a read shows it when hashline is
// on: the tag, then N:text lines, up to maxBytes of the file.
func hashlineRead(path, text string, maxBytes int) string {
	lines := addressableLines(text)
	tag := FileTag(text)
	var b strings.Builder
	b.WriteString(hashlineHeader(path, tag) + "\n")
	seen := make([]bool, len(lines))
	used := 0
	for i, l := range lines {
		if maxBytes > 0 && used+len(l)+1 > maxBytes {
			fmt.Fprintf(&b, "…[lines %d-%d not shown: %d of %d KB shown; jread with a task, or read_file again for a range]\n",
				i+1, len(lines), used>>10, len(text)>>10)
			break
		}
		fmt.Fprintf(&b, "%d:%s\n", i+1, l)
		seen[i] = true
		used += len(l) + 1
	}
	recordSnapshot(path, tag, lines, seen)
	return strings.TrimRight(b.String(), "\n")
}

// ---- edits ----------------------------------------------------------------

type hlOp struct {
	kind     string // "replace", "delete", "insert"
	from, to int    // 1-based, inclusive (replace/delete); insert: the anchor line
	before   bool   // insert before the anchor
	eof      bool   // insert after $
	guard    string
	body     []string
	patchLn  int
}

type hlSection struct {
	path, tag string
	ops       []hlOp
}

var (
	hlHeaderRe  = regexp.MustCompile(`^\[(.+)#([0-9A-Fa-f]{4})\]\s*$`)
	hlReplaceRe = regexp.MustCompile(`^replace\s+(\d+)(?:\s*-\s*(\d+))?\s+"(.*)"\s*:\s*$`)
	hlDeleteRe  = regexp.MustCompile(`^delete\s+(\d+)(?:\s*-\s*(\d+))?\s+"(.*)"\s*$`)
	hlInsertRe  = regexp.MustCompile(`^insert\s+(before|after)\s+(\d+|\$)\s*:\s*$`)
	hlPrefixRe  = regexp.MustCompile(`^\d+:`)
)

// isHashlinePatch reports whether a patch is line-anchored: its first real
// line is a [path#TAG] header.
func isHashlinePatch(text string) bool {
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || t == beginPatchMarker || strings.HasPrefix(t, "```") {
			continue
		}
		return hlHeaderRe.MatchString(t)
	}
	return false
}

// parseHashline reads the sections of a line-anchored patch.
func parseHashline(text string) ([]hlSection, error) {
	var secs []hlSection
	var op *hlOp
	flush := func() {
		if op != nil {
			secs[len(secs)-1].ops = append(secs[len(secs)-1].ops, *op)
			op = nil
		}
	}
	for i, raw := range strings.Split(text, "\n") {
		n := i + 1
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		switch {
		case t == beginPatchMarker || t == endPatchMarker || strings.HasPrefix(t, "```"):
			continue
		case hlHeaderRe.MatchString(t):
			flush()
			m := hlHeaderRe.FindStringSubmatch(t)
			secs = append(secs, hlSection{path: strings.TrimSpace(m[1]), tag: strings.ToUpper(m[2])})
			continue
		}
		if len(secs) == 0 {
			if t == "" {
				continue
			}
			return nil, fmt.Errorf("line %d: a line-anchored patch starts with [path#TAG], the header of the read it edits", n)
		}
		if op != nil && strings.HasPrefix(line, "+") && op.kind != "delete" {
			op.body = append(op.body, line[1:])
			continue
		}
		switch m := hlReplaceRe.FindStringSubmatch(t); {
		case m != nil:
			flush()
			from, to := rangeOf(m[1], m[2])
			op = &hlOp{kind: "replace", from: from, to: to, guard: m[3], patchLn: n}
			continue
		}
		if m := hlDeleteRe.FindStringSubmatch(t); m != nil {
			flush()
			from, to := rangeOf(m[1], m[2])
			secs[len(secs)-1].ops = append(secs[len(secs)-1].ops, hlOp{kind: "delete", from: from, to: to, guard: m[3], patchLn: n})
			continue
		}
		if m := hlInsertRe.FindStringSubmatch(t); m != nil {
			flush()
			o := hlOp{kind: "insert", before: m[1] == "before", patchLn: n}
			if m[2] == "$" {
				o.eof = true
			} else {
				o.from, _ = strconv.Atoi(m[2])
				o.to = o.from
			}
			op = &o
			continue
		}
		if t == "" {
			continue
		}
		return nil, fmt.Errorf("line %d: %q is not an edit. Use `replace N-M \"<start of line N>\":`, `insert before|after N:` or `delete N-M \"<start of line N>\"`, with the new lines each starting with +", n, truncateForMessage(t, 80))
	}
	flush()
	for _, s := range secs {
		if len(s.ops) == 0 {
			return nil, fmt.Errorf("[%s#%s] has no edit under it", s.path, s.tag)
		}
		for _, o := range s.ops {
			if o.kind != "delete" && len(o.body) == 0 {
				return nil, fmt.Errorf("line %d: %s needs its new lines, each starting with + (a lone + is a blank line; to remove lines use delete)", o.patchLn, o.kind)
			}
		}
	}
	return secs, nil
}

func rangeOf(a, b string) (int, int) {
	from, _ := strconv.Atoi(a)
	to := from
	if b != "" {
		to, _ = strconv.Atoi(b)
	}
	return from, to
}

// lineMap maps old line numbers (1-based) to new ones through the lines the
// two texts share unchanged; 0 means the line changed.
func lineMap(old, cur []string) []int {
	m := make([]int, len(old)+1)
	for _, oc := range difflib.NewMatcher(old, cur).GetOpCodes() {
		if oc.Tag != 'e' {
			continue
		}
		for k := 0; k < oc.I2-oc.I1; k++ {
			m[oc.I1+k+1] = oc.J1 + k + 1
		}
	}
	return m
}

// hlEdit is one operation resolved against the current file: replace lines
// [start, end) (0-based; start == end is an insertion at start) with body.
type hlEdit struct {
	start, end int
	body       []string
	op         hlOp
}

// resolveHashline checks a section against the file at target as it is now
// (cur) and returns its edits, in the current file's numbering.
func resolveHashline(sec hlSection, target string, cur []string) ([]hlEdit, error) {
	curTag := FileTag(strings.Join(cur, "\n"))
	snap := lookupSnapshot(target, sec.tag)
	var mapped []int // nil: the numbers are the current file's
	if sec.tag != curTag {
		if snap == nil {
			return nil, staleError(sec, target, cur, curTag, "the file changed since that read, and that read is not known here")
		}
		mapped = lineMap(snap.lines, cur)
	}
	total := len(cur)
	if snap != nil {
		total = len(snap.lines)
	}
	at := func(n int) (int, bool) { // a line of the read → the current file
		if n < 1 || n > total {
			return 0, false
		}
		if mapped == nil {
			return n, true
		}
		return mapped[n], mapped[n] > 0
	}
	var edits []hlEdit
	for _, o := range sec.ops {
		body := stripPastedNumbers(o.body)
		switch o.kind {
		case "insert":
			var pos int
			switch {
			case o.eof:
				pos = len(cur)
			default:
				n, ok := at(o.from)
				if !ok {
					return nil, opError(sec, target, cur, curTag, o, fmt.Sprintf("line %d is not a line of that read, or it has changed since", o.from))
				}
				pos = n
				if o.before {
					pos = n - 1
				}
			}
			edits = append(edits, hlEdit{start: pos, end: pos, body: body, op: o})
		default:
			if o.from > o.to {
				return nil, opError(sec, target, cur, curTag, o, fmt.Sprintf("the range %d-%d runs backwards", o.from, o.to))
			}
			first, ok1 := at(o.from)
			last, ok2 := at(o.to)
			if !ok1 || !ok2 || last-first != o.to-o.from {
				return nil, opError(sec, target, cur, curTag, o, fmt.Sprintf("lines %d-%d are not all lines of that read unchanged since", o.from, o.to))
			}
			if snap != nil {
				for n := o.from; n <= o.to; n++ {
					if !snap.seen[n-1] {
						return nil, opError(sec, target, cur, curTag, o, fmt.Sprintf("line %d was not shown in full by that read — read it before changing it", n))
					}
				}
			}
			if msg := guardMismatch(o.guard, cur[first-1]); msg != "" {
				return nil, opError(sec, target, cur, curTag, o, msg)
			}
			if o.kind == "delete" {
				body = nil
			}
			edits = append(edits, hlEdit{start: first - 1, end: last, body: body, op: o})
		}
	}
	sort.SliceStable(edits, func(a, b int) bool { return edits[a].start < edits[b].start })
	for i := 1; i < len(edits); i++ {
		p, c := edits[i-1], edits[i]
		if c.start < p.end || (c.start == p.start && p.end > p.start) {
			return nil, opError(sec, target, cur, curTag, c.op, "it overlaps another edit in this patch")
		}
	}
	return edits, nil
}

// guardMismatch is "" when guard is how line starts (indentation ignored):
// at least 3 characters, or the whole line when it is shorter.
func guardMismatch(guard, line string) string {
	want := strings.TrimLeft(line, " \t")
	g := strings.TrimLeft(guard, " \t")
	switch {
	case g == "" && strings.TrimSpace(want) == "":
		return ""
	case len([]rune(g)) < 3 && g != strings.TrimRight(want, " \t\r"):
		return fmt.Sprintf("\"%s\" is too short to check the line: give its first 3 characters or more", guard)
	case strings.HasPrefix(want, g):
		return ""
	}
	return fmt.Sprintf("line starts %q, not %q — the numbers are off, or the file changed", truncateForMessage(strings.TrimRight(want, "\r"), 40), g)
}

// stripPastedNumbers removes N: prefixes a model copied from the read, when
// every new line carries one.
func stripPastedNumbers(body []string) []string {
	if len(body) == 0 {
		return body
	}
	for _, l := range body {
		if !hlPrefixRe.MatchString(l) {
			return body
		}
	}
	out := make([]string, len(body))
	for i, l := range body {
		out[i] = hlPrefixRe.ReplaceAllString(l, "")
	}
	return out
}

// applyHashlineEdits returns cur with edits applied, bottom-up, and the
// changed spans in the new numbering (0-based, [start, end)).
func applyHashlineEdits(cur []string, edits []hlEdit) ([]string, [][2]int) {
	out := append([]string(nil), cur...)
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		next := make([]string, 0, len(out)-(e.end-e.start)+len(e.body))
		next = append(next, out[:e.start]...)
		next = append(next, e.body...)
		next = append(next, out[e.end:]...)
		out = next
	}
	var spans [][2]int
	shift := 0
	for _, e := range edits {
		s := e.start + shift
		spans = append(spans, [2]int{s, s + len(e.body)})
		shift += len(e.body) - (e.end - e.start)
	}
	return out, spans
}

// hashlineWindows shows the changed spans ±around lines, numbered, merged.
func hashlineWindows(lines []string, spans [][2]int, around int) (string, []bool) {
	seen := make([]bool, len(lines))
	for _, s := range spans {
		for i := max(0, s[0]-around); i < min(len(lines), s[1]+around); i++ {
			seen[i] = true
		}
	}
	var b strings.Builder
	prev := -2
	for i, ok := range seen {
		if !ok {
			continue
		}
		if prev >= 0 && i != prev+1 {
			b.WriteString("…\n")
		}
		fmt.Fprintf(&b, "%d:%s\n", i+1, lines[i])
		prev = i
	}
	return strings.TrimRight(b.String(), "\n"), seen
}

func staleError(sec hlSection, target string, cur []string, curTag, why string) error {
	var near []int
	for _, o := range sec.ops {
		near = append(near, o.from)
	}
	return fmt.Errorf("[%s#%s]: %s. Nothing was written. The file now is %s — around the lines you named:\n%s",
		sec.path, sec.tag, why, hashlineHeader(sec.path, curTag), windowAround(target, cur, curTag, near...))
}

func opError(sec hlSection, target string, cur []string, curTag string, o hlOp, why string) error {
	return fmt.Errorf("[%s#%s] line %d of the patch (%s %d): %s. Nothing was written. The file now is %s:\n%s",
		sec.path, sec.tag, o.patchLn, o.kind, o.from, why, hashlineHeader(sec.path, curTag), windowAround(target, cur, curTag, o.from))
}

// windowAround shows ±5 numbered lines around each of lines (1-based) in
// cur, and records them as seen at curTag for the file at target.
func windowAround(target string, cur []string, curTag string, lines ...int) string {
	var spans [][2]int
	for _, n := range lines {
		if n < 1 {
			n = 1
		}
		spans = append(spans, [2]int{n - 1, n})
	}
	out, seen := hashlineWindows(cur, spans, 5)
	recordSnapshot(target, curTag, cur, seen)
	return out
}

// applyHashlinePatch applies a line-anchored patch: every section resolved
// and checked, then every file written, or none (docs/features/HASHLINE.md).
func applyHashlinePatch(text, cwd string) (string, error) {
	secs, err := parseHashline(text)
	if err != nil {
		return "", fmt.Errorf("apply_patch: %w", err)
	}
	type write struct {
		display, target, before, after string
		lines                          []string
		spans                          [][2]int
	}
	var writes []write
	locked := make([]string, 0, len(secs))
	for _, sec := range secs {
		if cwd != "" && !filepath.IsAbs(sec.path) {
			locked = append(locked, filepath.Join(cwd, sec.path))
		} else {
			locked = append(locked, sec.path)
		}
	}
	defer LockFiles(locked...)()
	for _, sec := range secs {
		target := sec.path
		if cwd != "" && !filepath.IsAbs(target) {
			target = filepath.Join(cwd, target)
		}
		if cwd != "" {
			if rel, err := filepath.Rel(cwd, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("apply_patch [%s]: outside the project folder — nothing was written", sec.path)
			}
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			return "", fmt.Errorf("apply_patch [%s]: %w — to create a file use *** Add File", sec.path, err)
		}
		before := string(raw)
		cur := addressableLines(before)
		edits, err := resolveHashline(sec, target, cur)
		if err != nil {
			return "", fmt.Errorf("apply_patch %w", err)
		}
		lines, spans := applyHashlineEdits(cur, edits)
		_, layout := splitFileLines(before)
		after := layout.join(lines)
		if contentEquivalent(before, after) {
			return "", fmt.Errorf("apply_patch [%s]: the edits change nothing — the new lines are the ones already there. Nothing was written", sec.path)
		}
		// The guards every patch passes (apply_patch.go): nothing is written
		// until all sections pass them.
		if err := guardGoModSyntax(target, after); err != nil {
			return "", fmt.Errorf("apply_patch [%s]: %w", sec.path, err)
		}
		if guardSyntaxRegression(target, after) != nil {
			return "", fmt.Errorf("apply_patch [%s]: the file would no longer parse:\n%s\n%sNothing was written. The file now is %s:\n%s",
				sec.path, describeGoParseErrors(target, after), shortRanges(edits), hashlineHeader(sec.path, FileTag(before)), windowAround(target, cur, FileTag(before), editLines(edits)...))
		}
		if strings.HasSuffix(target, ".go") {
			if err := guardUnresolvableImports(target, after, map[string]bool{}); err != nil {
				return "", fmt.Errorf("apply_patch [%s]: %w", sec.path, err)
			}
		}
		writes = append(writes, write{display: sec.path, target: target, before: before, after: after, lines: lines, spans: spans})
	}
	for i, w := range writes {
		if err := os.WriteFile(w.target, []byte(w.after), 0o644); err != nil {
			for _, done := range writes[:i] {
				_ = os.WriteFile(done.target, []byte(done.before), 0o644)
			}
			return "", fmt.Errorf("apply_patch write %s: %w — nothing was written", w.display, err)
		}
	}
	var modified []string
	for _, w := range writes {
		modified = append(modified, w.display)
	}
	msg := fmt.Sprintf("Patch applied. modified=%v\n", modified)
	if verdict := autoVerifyGoPatch(cwd, modified); verdict != "" {
		msg += verdict
	} else {
		msg += "Next: run the build/test with the bash tool and read the real output. Do NOT report done until it builds and runs."
	}
	// Fresh anchors for the next edit, without a read: the new tag and the
	// changed lines ±3.
	for _, w := range writes {
		tag := FileTag(w.after)
		shown, seen := hashlineWindows(w.lines, w.spans, 3)
		recordSnapshot(w.target, tag, w.lines, seen)
		msg += "\n\n" + hashlineHeader(w.display, tag) + "\n" + shown
	}
	return msg, nil
}

// hashlineInput is apply_patch's input as the model sees it when hashline is
// on: same field, the line-anchored form described first.
type hashlineInput struct {
	Input string `json:"input" jsonschema_description:"Line-anchored edits under the [path#TAG] header of the read they come from (see the tool description), or a *** Begin Patch / *** End Patch patch to create or delete a file."`
}

var hashlineInputSchema = GenerateSchema[hashlineInput]()

const hashlineDescription = `The single tool for changing files.

EDIT a file by line number: reads show the file as [path#TAG] and numbered
N:text lines. Put that header, then the edits, numbered as in that read:
    [<path>#<TAG>]
    replace 12-14 "<first chars of line 12>":
    +<new line, with its indentation>
    +<another new line>
    insert after 20:
    +<new line>
    delete 31-33 "<first chars of line 31>"
- "<first chars>": the start of the first line of the range as read (3 or more characters, indentation ignored) — it checks the numbers.
- Every line of N-M is replaced by your + lines: the range is exactly the lines you retype, and only lines that change.
- Numbers are from the read, never shifted by your earlier edits in the same patch; ranges must not overlap. insert before 1 = the top, insert after $ = the end.
- A lone + is a blank line. Write each new line as it will be in the file, without the N: prefix.
- Several files: one [path#TAG] header each. The answer gives the new tag and numbered lines around each change, so the next edit needs no read.

CREATE or DELETE a file:
    *** Begin Patch
    *** Add File: <path>
    +<every line, each prefixed with +>
    *** Delete File: <path>
    *** End Patch`

// applyPatchParams is how apply_patch is described to the model this call.
func applyPatchParams(def ToolDefinition) (string, llm.ToolInputSchemaParam) {
	if def.Name == ApplyPatchDefinition.Name && HashlineOn() {
		return hashlineDescription, hashlineInputSchema
	}
	return def.Description, def.InputSchema
}

// shortRanges names each replace that removes more lines than it gives: the
// usual way a line-anchored edit breaks a file is a range that runs past the
// lines the model meant to change (live 2026-09-29: replace 57-59 with the one
// new line of 57 dropped the if's body and its brace).
func shortRanges(edits []hlEdit) string {
	var b strings.Builder
	for _, e := range edits {
		if e.op.kind != "replace" || len(e.body) >= e.end-e.start {
			continue
		}
		fmt.Fprintf(&b, "replace %d-%d removes %d lines and gives %d: every line of the range is replaced by your + lines. To change only line %d use replace %d-%d.\n",
			e.op.from, e.op.to, e.op.to-e.op.from+1, len(e.body), e.op.from, e.op.from, e.op.from+len(e.body)-1)
	}
	return b.String()
}

func editLines(edits []hlEdit) []int {
	var out []int
	for _, e := range edits {
		out = append(out, e.start+1)
	}
	return out
}
