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

	"github.com/mempalace/mempalace-go/internal/store"
)

// Question represents a single LongMemEval benchmark question.
type Question struct {
	QuestionText string      `json:"question"`
	QuestionID   string      `json:"question_id"`
	QuestionType string      `json:"question_type"`
	Answer       json.RawMessage `json:"answer"`
	SessionIDs   []string    `json:"haystack_session_ids"`
	Sessions     [][]Message `json:"haystack_sessions"`
	Dates        []string    `json:"haystack_dates"`
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
	flag.Parse()

	if *dataPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: bench --data <longmemeval.json> [--limit N] [--csv output.csv]")
		os.Exit(1)
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
		for j, session := range q.Sessions {
			var parts []string
			for _, m := range session {
				parts = append(parts, m.Content)
			}
			content := strings.Join(parts, "\n")
			sessionID := fmt.Sprintf("sess_%d", j)
			if j < len(q.SessionIDs) {
				sessionID = q.SessionIDs[j]
			}
			s.Add(store.Drawer{
				ID:       sessionID,
				Document: content,
				Wing:     "bench",
				Room:     "haystack",
			})
		}

		// Search
		start := time.Now()
		results, err := s.Search(q.QuestionText, 10, store.Query{})
		latency := time.Since(start)
		totalLatency += latency

		if err != nil {
			fmt.Fprintf(os.Stderr, "search error for %s: %v\n", q.QuestionID, err)
			s.Close()
			os.RemoveAll(dir)
			continue
		}

		// Find gold session — the one whose content contains the answer
		goldID := findGoldSessionID(q)

		rank := -1
		for r, res := range results {
			if res.ID == goldID {
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
	fmt.Printf("\n=== LongMemEval Results (Go FTS5) ===\n")
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

// findGoldSessionID returns the session ID of the session most likely containing
// the answer. It searches for the session whose content best matches the answer text.
func findGoldSessionID(q Question) string {
	// Answer can be string or number in the JSON
	var answerStr string
	if err := json.Unmarshal(q.Answer, &answerStr); err != nil {
		answerStr = strings.Trim(string(q.Answer), `"`)
	}
	answerLower := strings.ToLower(answerStr)
	bestIdx := 0
	bestScore := 0

	for i, session := range q.Sessions {
		score := 0
		for _, m := range session {
			if strings.Contains(strings.ToLower(m.Content), answerLower) {
				score += 10
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx < len(q.SessionIDs) {
		return q.SessionIDs[bestIdx]
	}
	return fmt.Sprintf("sess_%d", bestIdx)
}
