// Package bm25 is a small, deterministic Okapi BM25 index with a code-aware
// tokenizer. It powers the coarse file-localization stage and exact-identifier
// search of the large-repo pipeline (docs/architecture/LARGE_REPO_DESIGN.md).
//
// The tokenizer splits identifiers on snake_case and camelCase and indexes the
// sub-words alongside the whole token, so a query for "store" matches a
// definition named "NewStore". It is model-free and index-free (built in memory
// from the docs handed in).
package bm25

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Okapi BM25 free parameters (standard defaults).
const (
	paramK1 = 1.2
	paramB  = 0.75
)

// Doc is one indexable unit (a file, a function body, …).
type Doc struct {
	ID   string
	Text string
}

// Result is a scored document, highest first.
type Result struct {
	ID    string
	Score float64
}

// Index is a built BM25 index. Immutable after New.
type Index struct {
	docTerms map[string]map[string]int // docID → term → term-frequency
	docLen   map[string]int
	df       map[string]int // term → number of docs containing it
	ids      []string       // sorted, for deterministic iteration
	avgdl    float64
	n        int
}

// New builds an index over docs. Duplicate IDs are ignored (first wins),
// deterministically.
func New(docs []Doc) *Index {
	ix := &Index{
		docTerms: map[string]map[string]int{},
		docLen:   map[string]int{},
		df:       map[string]int{},
	}
	var total int
	for _, d := range docs {
		if _, dup := ix.docTerms[d.ID]; dup {
			continue
		}
		toks := Tokenize(d.Text)
		tf := map[string]int{}
		for _, t := range toks {
			tf[t]++
		}
		ix.docTerms[d.ID] = tf
		ix.docLen[d.ID] = len(toks)
		total += len(toks)
		for t := range tf {
			ix.df[t]++
		}
		ix.ids = append(ix.ids, d.ID)
	}
	ix.n = len(ix.ids)
	if ix.n > 0 {
		ix.avgdl = float64(total) / float64(ix.n)
	}
	sort.Strings(ix.ids)
	return ix
}

// idf is the BM25 inverse-document-frequency (always ≥ 0 with the +1 form).
func (ix *Index) idf(term string) float64 {
	df := ix.df[term]
	if df == 0 {
		return 0
	}
	return math.Log(1 + (float64(ix.n)-float64(df)+0.5)/(float64(df)+0.5))
}

// Search returns the top-K documents for query, scored by BM25. topK ≤ 0 returns
// all matches. Ties break by document ID for determinism.
func (ix *Index) Search(query string, topK int) []Result {
	seen := map[string]bool{}
	var uniq []string
	for _, t := range Tokenize(query) {
		if !seen[t] {
			seen[t] = true
			uniq = append(uniq, t)
		}
	}
	if ix.n == 0 || len(uniq) == 0 {
		return nil
	}
	var results []Result
	for _, id := range ix.ids {
		tf := ix.docTerms[id]
		dl := float64(ix.docLen[id])
		var score float64
		for _, t := range uniq {
			f := float64(tf[t])
			if f == 0 {
				continue
			}
			score += ix.idf(t) * (f * (paramK1 + 1)) /
				(f + paramK1*(1-paramB+paramB*dl/ix.avgdl))
		}
		if score > 0 {
			results = append(results, Result{ID: id, Score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// Tokenize splits text into lowercased code tokens. Runs of identifier
// characters (letters, digits, underscore) are emitted whole AND split on
// snake_case and camelCase into sub-words, so "NewStore" indexes as
// {"newstore","new","store"}. Everything else is a separator.
func Tokenize(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, subtokens(cur.String())...)
		cur.Reset()
	}
	for _, r := range s {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// subtokens returns the whole lowercased identifier plus its camel/snake
// sub-words (deduped to just the whole token when it doesn't split).
func subtokens(id string) []string {
	whole := strings.ToLower(id)
	var subs []string
	for _, seg := range strings.Split(id, "_") {
		for _, part := range splitCamel(seg) {
			if part != "" {
				subs = append(subs, strings.ToLower(part))
			}
		}
	}
	if len(subs) == 1 && subs[0] == whole {
		return []string{whole}
	}
	return append([]string{whole}, subs...)
}

// splitCamel splits a segment at camelCase boundaries, keeping acronym runs
// together: "NewStore"→[New Store], "HTTPServer"→[HTTP Server], "id8"→[id8].
func splitCamel(s string) []string {
	rs := []rune(s)
	var out []string
	var cur []rune
	for i, r := range rs {
		boundary := false
		if i > 0 && unicode.IsUpper(r) {
			prevLower := unicode.IsLower(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if prevLower || nextLower { // aA → a|A ; AAa → A|Aa
				boundary = true
			}
		}
		if boundary {
			out = append(out, string(cur))
			cur = cur[:0]
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}
