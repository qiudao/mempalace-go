package search

import (
	"path/filepath"
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSearchMemories(t *testing.T) {
	s := tempStore(t)
	s.Add(store.Drawer{ID: "d1", Document: "GraphQL architecture for the API", Wing: "backend", Room: "arch"})
	s.Add(store.Drawer{ID: "d2", Document: "React component design patterns", Wing: "frontend", Room: "ui"})

	results, err := SearchMemories(s, "GraphQL API", 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Wing != "backend" {
		t.Fatalf("expected backend wing, got %s", results[0].Wing)
	}
}

func TestSearchWithWingFilter(t *testing.T) {
	s := tempStore(t)
	s.Add(store.Drawer{ID: "d1", Document: "GraphQL backend", Wing: "backend", Room: "api"})
	s.Add(store.Drawer{ID: "d2", Document: "GraphQL frontend", Wing: "frontend", Room: "ui"})

	results, _ := SearchMemories(s, "GraphQL", 10, "frontend", "")
	if len(results) != 1 || results[0].Wing != "frontend" {
		t.Fatalf("wing filter failed: %+v", results)
	}
}

func TestSearchNoResults(t *testing.T) {
	s := tempStore(t)
	results, err := SearchMemories(s, "nonexistent", 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatal("expected no results")
	}
}
