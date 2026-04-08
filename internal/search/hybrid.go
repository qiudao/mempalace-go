package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/mempalace/mempalace-go/internal/store"
)

var stopWords = map[string]bool{
	"what": true, "when": true, "where": true, "who": true, "how": true,
	"which": true, "did": true, "do": true, "was": true, "were": true,
	"have": true, "has": true, "had": true, "is": true, "are": true,
	"the": true, "a": true, "an": true, "my": true, "me": true,
	"i": true, "you": true, "your": true, "their": true, "it": true,
	"its": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"ago": true, "last": true, "that": true, "this": true, "there": true,
	"about": true, "get": true, "got": true, "give": true, "gave": true,
	"buy": true, "bought": true, "made": true, "make": true,
}

// ExtractKeywords splits text into unique lowercase tokens (3+ chars),
// filtering out common stop words.
func ExtractKeywords(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var keywords []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) >= 3 && !stopWords[w] && !seen[w] {
			keywords = append(keywords, w)
			seen[w] = true
		}
	}
	return keywords
}

// KeywordOverlap returns the fraction of keywords found in text (0.0 to 1.0).
func KeywordOverlap(keywords []string, text string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	hits := 0
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			hits++
		}
	}
	return float64(hits) / float64(len(keywords))
}

// HybridResult extends a SearchResult with a boosted rank score.
type HybridResult struct {
	store.SearchResult
	BoostedRank float64
}

// FusedSearch combines BM25 (FTS5) and vector search using Reciprocal Rank Fusion.
// Each result gets score = 1/(k+rank_bm25) + 1/(k+rank_vec), k=60 by default.
// This captures both exact keyword matches (BM25) and semantic similarity (vector).
func FusedSearch(s *store.Store, query string, queryVec []float32, limit int, q store.Query) ([]store.SearchResult, error) {
	const k = 60 // RRF constant
	pool := limit * 5
	if pool < 50 {
		pool = 50
	}

	// BM25 results
	bm25Results, err := s.Search(query, pool, q)
	if err != nil {
		bm25Results = nil // fallback to vector only
	}

	// Vector results
	vecResults, err := s.VectorSearch(queryVec, pool, q)
	if err != nil {
		vecResults = nil // fallback to BM25 only
	}

	// Build RRF scores
	scores := make(map[string]float64)
	drawerMap := make(map[string]store.SearchResult)

	for rank, r := range bm25Results {
		scores[r.ID] += 1.0 / float64(k+rank+1)
		drawerMap[r.ID] = r
	}
	for rank, r := range vecResults {
		scores[r.ID] += 1.0 / float64(k+rank+1)
		drawerMap[r.ID] = r
	}

	// Sort by fused score descending
	type scored struct {
		id    string
		score float64
	}
	var ranked []scored
	for id, sc := range scores {
		ranked = append(ranked, scored{id, sc})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	var out []store.SearchResult
	for _, r := range ranked {
		sr := drawerMap[r.id]
		sr.Rank = -r.score // negative so higher score = better (consistent with FTS5 convention)
		out = append(out, sr)
	}
	return out, nil
}

// HybridSearch performs FTS5 search then re-ranks results using keyword overlap boosting.
// The boost formula is: boosted_rank = rank * (1.0 - 0.30 * overlap)
// Since FTS5 BM25 rank is negative (more negative = better), multiplying by a
// factor < 1.0 makes good keyword-overlap results rank even better.
func HybridSearch(s *store.Store, query string, limit int, q store.Query) ([]HybridResult, error) {
	// Fetch extra candidates for re-ranking
	fetchLimit := limit * 5
	if fetchLimit < 20 {
		fetchLimit = 20
	}
	candidates, err := s.Search(query, fetchLimit, q)
	if err != nil {
		return nil, err
	}

	keywords := ExtractKeywords(query)

	results := make([]HybridResult, 0, len(candidates))
	for _, c := range candidates {
		overlap := KeywordOverlap(keywords, c.Document)
		boosted := c.Rank * (1.0 - 0.30*overlap)
		results = append(results, HybridResult{
			SearchResult: c,
			BoostedRank:  boosted,
		})
	}

	// Sort by boosted rank — more negative (lower) is better for FTS5 BM25
	sort.Slice(results, func(i, j int) bool {
		return results[i].BoostedRank < results[j].BoostedRank
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
