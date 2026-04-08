package miner

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mempalace/mempalace-go/internal/normalize"
	"github.com/mempalace/mempalace-go/internal/store"
)

// MineConvos walks sourceDir for conversation files (.txt, .json, .jsonl),
// normalises each to transcript format, splits into exchange pairs, groups
// them into chunks, and stores each chunk in the palace.
func MineConvos(sourceDir, palacePath, wing string, embedder ...Embedder) error {
	var emb Embedder
	if len(embedder) > 0 {
		emb = embedder[0]
	}
	return mineConvosImpl(sourceDir, palacePath, wing, emb)
}

func mineConvosImpl(sourceDir, palacePath, wing string, emb Embedder) error {
	os.MkdirAll(palacePath, 0755)
	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	now := time.Now().Format("2006-01-02")

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".txt" && ext != ".json" && ext != ".jsonl" {
			return nil
		}

		content, err := normalize.Normalize(path)
		if err != nil || content == "" {
			return nil
		}

		relPath, _ := filepath.Rel(sourceDir, path)
		exchanges := splitExchanges(content)
		chunks := groupExchanges(exchanges, 1) // 1 exchange per chunk for fine-grained search

		for _, chunk := range chunks {
			room := detectConvoRoom(chunk)
			id := fmt.Sprintf("%x", md5.Sum([]byte(wing+room+chunk)))
			drawer := store.Drawer{
				ID:       id,
				Document: chunk,
				Wing:     wing,
				Room:     room,
				Source:   relPath,
				FiledAt:  now,
			}
			if emb != nil {
				vec, err := emb.Embed(chunk)
				if err == nil {
					s.AddWithEmbedding(drawer, vec)
				} else {
					s.Add(drawer)
				}
			} else {
				s.Add(drawer)
			}
		}
		return nil
	})
}

// splitExchanges splits a transcript on "> user:" markers.
// Each exchange = user message + following assistant response.
func splitExchanges(text string) []string {
	parts := strings.Split(text, "> user:")
	var exchanges []string
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i > 0 {
			part = "> user:" + part
		}
		exchanges = append(exchanges, part)
	}
	return exchanges
}

// groupExchanges combines exchanges into chunks of perGroup each.
func groupExchanges(exchanges []string, perGroup int) []string {
	if len(exchanges) == 0 {
		return nil
	}
	var chunks []string
	for i := 0; i < len(exchanges); i += perGroup {
		end := i + perGroup
		if end > len(exchanges) {
			end = len(exchanges)
		}
		chunk := strings.Join(exchanges[i:end], "\n")
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

// detectConvoRoom uses keyword matching to classify a conversation chunk
// into a room category.
func detectConvoRoom(text string) string {
	lower := strings.ToLower(text)
	keywords := map[string][]string{
		"architecture": {"architecture", "design pattern", "system design", "microservice"},
		"debugging":    {"bug", "error", "fix", "crash", "exception", "stack trace"},
		"database":     {"database", "sql", "postgres", "mysql", "migration", "schema"},
		"frontend":     {"react", "vue", "css", "html", "component", "ui", "ux"},
		"backend":      {"api", "endpoint", "server", "route", "middleware", "graphql"},
		"devops":       {"deploy", "docker", "kubernetes", "ci/cd", "pipeline", "aws"},
	}
	best := "general"
	bestCount := 0
	for room, words := range keywords {
		count := 0
		for _, w := range words {
			count += strings.Count(lower, w)
		}
		if count > bestCount {
			bestCount = count
			best = room
		}
	}
	return best
}
