package store

import (
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
