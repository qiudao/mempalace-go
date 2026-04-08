package search

import (
	"fmt"

	"github.com/mempalace/mempalace-go/internal/store"
)

// Result holds a single search hit with metadata and BM25 rank.
type Result struct {
	ID       string
	Document string
	Wing     string
	Room     string
	Source   string
	FiledAt  string
	Rank     float64
}

// SearchMemories returns results programmatically.
func SearchMemories(s *store.Store, query string, limit int, wing, room string) ([]Result, error) {
	results, err := s.Search(query, limit, store.Query{Wing: wing, Room: room})
	if err != nil {
		return nil, err
	}
	var out []Result
	for _, r := range results {
		out = append(out, Result{
			ID:       r.ID,
			Document: r.Document,
			Wing:     r.Wing,
			Room:     r.Room,
			Source:   r.Source,
			FiledAt:  r.FiledAt,
			Rank:     r.Rank,
		})
	}
	return out, nil
}

// Search prints results to stdout (CLI usage).
func Search(s *store.Store, query string, limit int, wing, room string) error {
	results, err := SearchMemories(s, query, limit, wing, room)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}
	for i, r := range results {
		fmt.Printf("\n--- Result %d [%s/%s] ---\n", i+1, r.Wing, r.Room)
		fmt.Printf("Source: %s | Filed: %s\n", r.Source, r.FiledAt)
		doc := r.Document
		if len(doc) > 500 {
			doc = doc[:500] + "..."
		}
		fmt.Println(doc)
	}
	fmt.Printf("\n%d result(s) found.\n", len(results))
	return nil
}
