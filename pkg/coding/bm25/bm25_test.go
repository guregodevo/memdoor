package bm25

import (
	"reflect"
	"testing"
)

func TestTokenizeCamelAndSnake(t *testing.T) {
	got := Tokenize("NewStore new_store")
	want := map[string]bool{"newstore": true, "new": true, "store": true, "new_store": true}
	for _, tk := range got {
		delete(want, tk)
	}
	if len(want) != 0 {
		t.Errorf("missing tokens %v; got %v", want, got)
	}
	// A plain identifier isn't duplicated.
	if toks := Tokenize("store"); !reflect.DeepEqual(toks, []string{"store"}) {
		t.Errorf("plain token: got %v", toks)
	}
	// Acronym boundary.
	got = Tokenize("HTTPServer")
	if !contains(got, "http") || !contains(got, "server") {
		t.Errorf("acronym split failed: %v", got)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestSearchRanksByRelevance(t *testing.T) {
	ix := New([]Doc{
		{ID: "a", Text: "func NewStore() *Store { return &Store{} }"},
		{ID: "b", Text: "func UseStore() { s := NewStore(); s.Get() }"},
		{ID: "c", Text: "package main // unrelated logging helper"},
	})
	res := ix.Search("store", 10)
	if len(res) < 2 {
		t.Fatalf("expected store matches, got %v", res)
	}
	// 'c' has no store tokens → must not appear.
	for _, r := range res {
		if r.ID == "c" {
			t.Errorf("unrelated doc c should not match 'store': %v", res)
		}
	}
	// camelCase query resolves via sub-tokens.
	if got := ix.Search("NewStore", 1); len(got) == 0 {
		t.Error("camelCase query should match")
	}
}

func TestIDFDiscriminates(t *testing.T) {
	// "common" is in every doc (idf→low); "rare" in one (idf→high). A query for
	// both should rank the doc holding "rare" first.
	ix := New([]Doc{
		{ID: "a", Text: "common common common"},
		{ID: "b", Text: "common rare"},
		{ID: "c", Text: "common common"},
	})
	res := ix.Search("common rare", 10)
	if len(res) == 0 || res[0].ID != "b" {
		t.Errorf("expected doc b (holds rare term) first, got %v", res)
	}
}

func TestSearchDeterministicAndTopK(t *testing.T) {
	docs := []Doc{
		{ID: "a", Text: "store store"},
		{ID: "b", Text: "store store"}, // identical score → ID tiebreak
		{ID: "c", Text: "store"},
	}
	ix := New(docs)
	r1 := ix.Search("store", 2)
	r2 := ix.Search("store", 2)
	if !reflect.DeepEqual(r1, r2) {
		t.Error("search must be deterministic")
	}
	if len(r1) != 2 {
		t.Fatalf("topK=2 expected 2 results, got %d", len(r1))
	}
	if r1[0].ID != "a" || r1[1].ID != "b" {
		t.Errorf("tie should break by ID: %v", r1)
	}
}

func TestEmptyAndNoMatch(t *testing.T) {
	if r := New(nil).Search("x", 5); r != nil {
		t.Errorf("empty index should return nil, got %v", r)
	}
	ix := New([]Doc{{ID: "a", Text: "hello world"}})
	if r := ix.Search("nonexistent", 5); len(r) != 0 {
		t.Errorf("no-match query should return empty, got %v", r)
	}
}
