// Package main implements a LongMemEval benchmark runner for FTS5 search.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mempalace/mempalace-go/internal/embed"
	"github.com/mempalace/mempalace-go/internal/search"
	"github.com/mempalace/mempalace-go/internal/store"
)

// Question represents a single LongMemEval benchmark question.
type Question struct {
	QuestionText     string          `json:"question"`
	QuestionID       string          `json:"question_id"`
	QuestionType     string          `json:"question_type"`
	Answer           json.RawMessage `json:"answer"`
	AnswerSessionIDs []string        `json:"answer_session_ids"`
	SessionIDs       []string        `json:"haystack_session_ids"`
	Sessions         [][]Message     `json:"haystack_sessions"`
	Dates            []string        `json:"haystack_dates"`
}

// Message represents a single chat message in a session.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func main() {
	dataPath := flag.String("data", "", "Path to longmemeval JSON")
	limitQ := flag.Int("limit", 0, "Limit number of questions (0 = all)")
	csvOut := flag.String("csv", "", "Output CSV path")
	mode := flag.String("mode", "raw", "Search mode: raw, hybrid, vector, or fused")
	flag.Parse()

	if *dataPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: bench --data <longmemeval.json> [--limit N] [--csv output.csv] [--mode raw|hybrid|vector]")
		os.Exit(1)
	}

	// Initialize embedder for vector/fused mode (reuse across questions)
	var emb *embed.Embedder
	if *mode == "vector" || *mode == "fused" {
		home, _ := os.UserHomeDir()
		modelDir := filepath.Join(home, ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
		var err2 error
		emb, err2 = embed.NewEmbedder(modelDir)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "embedder init error:", err2)
			os.Exit(1)
		}
		defer emb.Close()
	}

	data, err := os.ReadFile(*dataPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var questions []Question
	if err := json.Unmarshal(data, &questions); err != nil {
		fmt.Fprintln(os.Stderr, "JSON parse error:", err)
		os.Exit(1)
	}

	if *limitQ > 0 && *limitQ < len(questions) {
		questions = questions[:*limitQ]
	}

	var hit5, hit10, total int
	var totalLatency time.Duration
	var csvRows []string
	csvRows = append(csvRows, "question_id,question_type,found_at_5,found_at_10,rank,latency_ms")

	for i, q := range questions {
		// Create fresh store per question
		dir, err := os.MkdirTemp("", "bench-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "tmpdir error: %v\n", err)
			continue
		}

		s, err := store.Open(filepath.Join(dir, "bench.db"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "store error: %v\n", err)
			os.RemoveAll(dir)
			continue
		}

		// Index sessions — one drawer per session, content = concatenated messages
		var sessionContents []string
		var sessionIDs []string
		for j, session := range q.Sessions {
			// Match Python benchmark: only index user turns
			var parts []string
			for _, m := range session {
				if m.Role == "user" {
					parts = append(parts, m.Content)
				}
			}
			content := strings.Join(parts, "\n")
			sid := fmt.Sprintf("sess_%d", j)
			if j < len(q.SessionIDs) {
				sid = q.SessionIDs[j]
			}
			sessionContents = append(sessionContents, content)
			sessionIDs = append(sessionIDs, sid)
		}

		if (*mode == "vector" || *mode == "fused") && emb != nil {
			// Batch embed all sessions
			vecs, err := emb.EmbedBatch(sessionContents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "embed error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			for j := range sessionContents {
				s.AddWithEmbedding(store.Drawer{
					ID: sessionIDs[j], Document: sessionContents[j],
					Wing: "bench", Room: "haystack",
				}, vecs[j])
			}
		} else {
			for j := range sessionContents {
				s.Add(store.Drawer{
					ID: sessionIDs[j], Document: sessionContents[j],
					Wing: "bench", Room: "haystack",
				})
			}
		}

		// Search
		start := time.Now()
		var resultIDs []string
		if *mode == "fused" && emb != nil {
			queryVec, err := emb.Embed(q.QuestionText)
			if err != nil {
				fmt.Fprintf(os.Stderr, "query embed error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			fusedResults, err := search.FusedSearch(s, q.QuestionText, queryVec, 10, store.Query{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "fused search error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			for _, r := range fusedResults {
				resultIDs = append(resultIDs, r.ID)
			}
		} else if *mode == "vector" && emb != nil {
			queryVec, err := emb.Embed(q.QuestionText)
			if err != nil {
				fmt.Fprintf(os.Stderr, "query embed error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			vecResults, err := s.VectorSearch(queryVec, 10, store.Query{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "vector search error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			for _, r := range vecResults {
				resultIDs = append(resultIDs, r.ID)
			}
		} else if *mode == "hybrid" {
			hybridResults, err := search.HybridSearch(s, q.QuestionText, 10, store.Query{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "search error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			for _, r := range hybridResults {
				resultIDs = append(resultIDs, r.ID)
			}
		} else {
			rawResults, err := s.Search(q.QuestionText, 10, store.Query{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "search error for %s: %v\n", q.QuestionID, err)
				s.Close()
				os.RemoveAll(dir)
				continue
			}
			for _, r := range rawResults {
				resultIDs = append(resultIDs, r.ID)
			}
		}
		latency := time.Since(start)
		totalLatency += latency

		// Gold sessions from dataset — check if ANY answer session is in results
		goldSet := make(map[string]bool)
		for _, gid := range q.AnswerSessionIDs {
			goldSet[gid] = true
		}

		rank := -1
		for r, id := range resultIDs {
			if goldSet[id] {
				rank = r + 1
				break
			}
		}

		foundAt5 := rank > 0 && rank <= 5
		foundAt10 := rank > 0 && rank <= 10
		if foundAt5 {
			hit5++
		}
		if foundAt10 {
			hit10++
		}
		total++

		csvRows = append(csvRows, fmt.Sprintf("%s,%s,%v,%v,%d,%d",
			q.QuestionID, q.QuestionType, foundAt5, foundAt10, rank, latency.Milliseconds()))

		s.Close()
		os.RemoveAll(dir)

		if (i+1)%50 == 0 {
			fmt.Printf("Progress: %d/%d (R@5: %.1f%%, R@10: %.1f%%)\n",
				i+1, len(questions),
				float64(hit5)/float64(total)*100,
				float64(hit10)/float64(total)*100)
		}
	}

	if total == 0 {
		fmt.Fprintln(os.Stderr, "No questions processed")
		os.Exit(1)
	}

	// Print results
	fmt.Printf("\n=== LongMemEval Results (Go FTS5, mode=%s) ===\n", *mode)
	fmt.Printf("Questions: %d\n", total)
	fmt.Printf("R@5:  %.1f%% (%d/%d)\n", float64(hit5)/float64(total)*100, hit5, total)
	fmt.Printf("R@10: %.1f%% (%d/%d)\n", float64(hit10)/float64(total)*100, hit10, total)
	fmt.Printf("Avg latency: %v\n", totalLatency/time.Duration(total))

	if *csvOut != "" {
		if err := os.WriteFile(*csvOut, []byte(strings.Join(csvRows, "\n")+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "CSV write error: %v\n", err)
		} else {
			fmt.Printf("CSV written to %s\n", *csvOut)
		}
	}
}

