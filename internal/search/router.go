package search

import (
	"strings"

	"github.com/mempalace/mempalace-go/internal/store"
)

// QueryType classifies the intent of a search query.
type QueryType int

const (
	QueryFact       QueryType = iota // direct fact recall: "What degree...", "Who gave me..."
	QueryPreference                  // preference/recommendation: "suggest...", "recommend..."
	QueryTemporal                    // time-based: "last month", "weeks ago", "when did"
)

// preferenceSignals — queries asking for suggestions/recommendations use different
// vocabulary than the stored answers, so pure Vector search works best.
var preferenceSignals = []string{
	"suggest", "recommend", "should i", "what should",
	"any tips", "can you help me find", "complement",
	"what would go well", "serve for", "activities",
	"accessories", "publications", "conferences",
}

// temporalSignals — time-anchored queries benefit from both BM25 (exact dates/events)
// and Vector (semantic context), so Fused works best.
var temporalSignals = []string{
	"ago", "last week", "last month", "months ago", "weeks ago", "days ago",
	"how many months", "how many days", "how long",
	"when did", "what did i do on", "recently",
}

// hasNonASCII checks if the query contains non-ASCII characters (CJK, etc.)
func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// ClassifyQuery determines the best search strategy for a query.
func ClassifyQuery(query string) QueryType {
	// Non-ASCII (Chinese, Japanese, Korean, etc.) → always use vector search
	// because FTS5 porter/unicode61 tokenizer doesn't handle CJK word segmentation
	if hasNonASCII(query) {
		return QueryPreference // routes to pure Vector
	}

	lower := strings.ToLower(query)

	for _, sig := range preferenceSignals {
		if strings.Contains(lower, sig) {
			return QueryPreference
		}
	}
	for _, sig := range temporalSignals {
		if strings.Contains(lower, sig) {
			return QueryTemporal
		}
	}
	return QueryFact
}

// SmartSearch routes the query to the optimal search strategy:
//   - Preference queries → pure Vector (BM25 hurts on vocabulary mismatch)
//   - Fact queries → Fused with BM25-heavy weight (exact keywords help)
//   - Temporal queries → Fused with balanced weight
//
// Requires both FTS5 store and a pre-computed query vector.
// If queryVec is nil, falls back to pure BM25.
func SmartSearch(s *store.Store, query string, queryVec []float32, limit int, q store.Query) ([]store.SearchResult, error) {
	if queryVec == nil {
		return s.Search(query, limit, q)
	}

	switch ClassifyQuery(query) {
	case QueryPreference:
		// Pure vector — BM25 hurts preference queries
		return s.VectorSearch(queryVec, limit, q)
	case QueryTemporal:
		// Balanced fusion — both signals help
		return FusedSearchWeighted(s, query, queryVec, limit, q, 1.0, 1.5)
	default: // QueryFact
		// BM25-heavy fusion — exact keywords are strong for facts
		return FusedSearchWeighted(s, query, queryVec, limit, q, 1.5, 1.0)
	}
}
