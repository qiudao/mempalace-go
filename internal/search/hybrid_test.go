package search

import (
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func TestExtractKeywords(t *testing.T) {
	kw := ExtractKeywords("What was the GraphQL API decision?")
	if len(kw) == 0 {
		t.Fatal("expected keywords")
	}
	for _, w := range kw {
		if stopWords[w] {
			t.Fatalf("stop word %q should be filtered", w)
		}
	}
	// "graphql", "api", "decision" should remain
	found := map[string]bool{}
	for _, w := range kw {
		found[w] = true
	}
	for _, want := range []string{"graphql", "api", "decision"} {
		if !found[want] {
			t.Fatalf("expected keyword %q in results %v", want, kw)
		}
	}
}

func TestExtractKeywordsEmpty(t *testing.T) {
	kw := ExtractKeywords("")
	if len(kw) != 0 {
		t.Fatalf("expected empty keywords, got %v", kw)
	}
}

func TestExtractKeywordsDedup(t *testing.T) {
	kw := ExtractKeywords("api api api design design")
	if len(kw) != 2 {
		t.Fatalf("expected 2 unique keywords, got %v", kw)
	}
}

func TestKeywordOverlap(t *testing.T) {
	keywords := []string{"graphql", "api", "decision"}
	score := KeywordOverlap(keywords, "We made a GraphQL API decision last week")
	if score < 0.9 {
		t.Fatalf("expected high overlap, got %.2f", score)
	}

	score = KeywordOverlap(keywords, "The weather is nice today")
	if score > 0.1 {
		t.Fatalf("expected low overlap, got %.2f", score)
	}
}

func TestKeywordOverlapEmpty(t *testing.T) {
	score := KeywordOverlap(nil, "anything")
	if score != 0 {
		t.Fatalf("expected 0 for nil keywords, got %.2f", score)
	}
}

func TestHybridSearch(t *testing.T) {
	s := tempStore(t)
	s.Add(store.Drawer{ID: "d1", Document: "GraphQL API architecture decision was made", Wing: "backend", Room: "arch"})
	s.Add(store.Drawer{ID: "d2", Document: "API endpoint for user management", Wing: "backend", Room: "api"})

	results, err := HybridSearch(s, "GraphQL API decision", 10, store.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// d1 should rank higher due to keyword overlap boost
	if results[0].ID != "d1" {
		t.Fatalf("expected d1 first with hybrid boost, got %s", results[0].ID)
	}
}

func TestHybridSearchNoResults(t *testing.T) {
	s := tempStore(t)
	results, err := HybridSearch(s, "nonexistent", 10, store.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}
