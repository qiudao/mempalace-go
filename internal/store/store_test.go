package store

import (
	"math"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAddAndGet(t *testing.T) {
	s := tempDB(t)

	err := s.Add(Drawer{
		ID:       "d1",
		Document: "we decided to use GraphQL for the API layer",
		Wing:     "backend",
		Room:     "architecture",
		Source:   "chat_2026-01-15.txt",
		FiledAt:  "2026-01-15",
	})
	if err != nil {
		t.Fatal(err)
	}

	drawers, err := s.Get(Query{Wing: "backend"})
	if err != nil {
		t.Fatal(err)
	}
	if len(drawers) != 1 {
		t.Fatalf("expected 1 drawer, got %d", len(drawers))
	}
	if drawers[0].Document != "we decided to use GraphQL for the API layer" {
		t.Fatalf("unexpected document: %s", drawers[0].Document)
	}
}

func TestCount(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "hello", Wing: "w1", Room: "r1"})
	s.Add(Drawer{ID: "d2", Document: "world", Wing: "w1", Room: "r2"})

	n, err := s.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestUpsert(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "old text", Wing: "w1", Room: "r1"})
	s.Upsert(Drawer{ID: "d1", Document: "new text", Wing: "w1", Room: "r1"})

	drawers, _ := s.Get(Query{})
	if len(drawers) != 1 || drawers[0].Document != "new text" {
		t.Fatalf("upsert failed: %+v", drawers)
	}
}

func TestDelete(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "hello", Wing: "w1", Room: "r1"})
	s.Delete([]string{"d1"})

	n, _ := s.Count()
	if n != 0 {
		t.Fatalf("expected 0 after delete, got %d", n)
	}
}

func TestSearch(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "we decided to use GraphQL for the API layer", Wing: "backend", Room: "arch"})
	s.Add(Drawer{ID: "d2", Document: "the frontend uses React with TypeScript", Wing: "frontend", Room: "stack"})
	s.Add(Drawer{ID: "d3", Document: "database migration to PostgreSQL completed", Wing: "backend", Room: "db"})

	results, err := s.Search("GraphQL API", 10, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].ID != "d1" {
		t.Fatalf("expected d1 first, got %s", results[0].ID)
	}
}

func TestSearchWithFilter(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "GraphQL for backend", Wing: "backend", Room: "arch"})
	s.Add(Drawer{ID: "d2", Document: "GraphQL for frontend", Wing: "frontend", Room: "arch"})

	results, _ := s.Search("GraphQL", 10, Query{Wing: "frontend"})
	if len(results) != 1 || results[0].ID != "d2" {
		t.Fatalf("filter failed: %+v", results)
	}
}

func TestSearchRanking(t *testing.T) {
	s := tempDB(t)
	s.Add(Drawer{ID: "d1", Document: "the auth module handles login", Wing: "w", Room: "r"})
	s.Add(Drawer{ID: "d2", Document: "auth service: auth tokens, auth middleware, auth flow", Wing: "w", Room: "r"})

	results, _ := s.Search("auth", 10, Query{})
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "d2" {
		t.Fatalf("expected d2 (more auth mentions) first, got %s", results[0].ID)
	}
}

// makeVec creates a simple test vector of given dimension with a constant value.
func makeVec(dim int, val float32) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = val
	}
	// L2 normalize
	var norm float64
	for _, f := range v {
		norm += float64(f) * float64(f)
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

func TestAddWithEmbedding(t *testing.T) {
	s := tempDB(t)
	vec := makeVec(384, 1.0)

	err := s.AddWithEmbedding(Drawer{
		ID:       "v1",
		Document: "test embedding storage",
		Wing:     "test",
		Room:     "embed",
	}, vec)
	if err != nil {
		t.Fatal(err)
	}

	// Verify drawer was stored
	n, err := s.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 drawer, got %d", n)
	}

	// Verify embedding can be retrieved via vector search
	results, err := s.VectorSearch(vec, 1, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "v1" {
		t.Fatalf("expected v1, got %s", results[0].ID)
	}
}

func TestVectorSearch(t *testing.T) {
	s := tempDB(t)

	// Create 3 vectors pointing in different directions
	vecA := make([]float32, 384)
	vecB := make([]float32, 384)
	vecC := make([]float32, 384)
	for i := range vecA {
		vecA[i] = 1.0
		vecB[i] = 1.0
		vecC[i] = -1.0
	}
	// Make B slightly different from A
	for i := 0; i < 50; i++ {
		vecB[i] = 0.5
	}

	s.AddWithEmbedding(Drawer{ID: "a", Document: "doc a", Wing: "w1", Room: "r1"}, vecA)
	s.AddWithEmbedding(Drawer{ID: "b", Document: "doc b", Wing: "w1", Room: "r2"}, vecB)
	s.AddWithEmbedding(Drawer{ID: "c", Document: "doc c", Wing: "w2", Room: "r1"}, vecC)

	// Query with vecA — should return a first, then b, then c
	results, err := s.VectorSearch(vecA, 3, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].ID != "a" {
		t.Fatalf("expected a first, got %s", results[0].ID)
	}
	if results[1].ID != "b" {
		t.Fatalf("expected b second, got %s", results[1].ID)
	}
	if results[2].ID != "c" {
		t.Fatalf("expected c last, got %s", results[2].ID)
	}

	// Test with wing filter
	results, err = s.VectorSearch(vecA, 10, Query{Wing: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with wing filter, got %d", len(results))
	}

	// Test limit
	results, err = s.VectorSearch(vecA, 1, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with limit, got %d", len(results))
	}
}
