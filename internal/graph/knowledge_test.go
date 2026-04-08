package graph

import (
	"path/filepath"
	"testing"
)

func tempKG(t *testing.T) *KnowledgeGraph {
	t.Helper()
	kg, err := OpenKnowledgeGraph(filepath.Join(t.TempDir(), "kg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kg.Close() })
	return kg
}

func TestAddAndQuery(t *testing.T) {
	kg := tempKG(t)
	kg.AddTriple("Alice", "works_at", "Acme", "2026-01-01")

	results, _ := kg.QueryEntity("Alice")
	if len(results) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(results))
	}
	if results[0].Object != "Acme" {
		t.Fatalf("expected Acme, got %s", results[0].Object)
	}
}

func TestInvalidate(t *testing.T) {
	kg := tempKG(t)
	kg.AddTriple("Alice", "works_at", "Acme", "2026-01-01")
	kg.Invalidate("Alice", "works_at", "Acme", "2026-06-01")

	// Active at 2026-03-01
	results, _ := kg.QueryEntityAt("Alice", "2026-03-01")
	if len(results) != 1 {
		t.Fatal("expected active triple at 2026-03-01")
	}

	// Not active at 2026-07-01
	results, _ = kg.QueryEntityAt("Alice", "2026-07-01")
	if len(results) != 0 {
		t.Fatal("expected no results after invalidation")
	}
}

func TestTimeline(t *testing.T) {
	kg := tempKG(t)
	kg.AddTriple("Alice", "works_at", "Acme", "2026-01-01")
	kg.AddTriple("Alice", "works_at", "NewCo", "2026-06-01")
	kg.Invalidate("Alice", "works_at", "Acme", "2026-06-01")

	triples, _ := kg.Timeline("Alice")
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples in timeline, got %d", len(triples))
	}
}

func TestStats(t *testing.T) {
	kg := tempKG(t)
	kg.AddTriple("Alice", "works_at", "Acme", "2026-01-01")
	kg.AddTriple("Bob", "knows", "Alice", "2026-02-01")

	stats, _ := kg.Stats()
	if stats.Entities < 2 || stats.Triples < 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
