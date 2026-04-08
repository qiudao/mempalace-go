package miner

import (
	"strings"
)

// MemoryType classifies an extracted memory.
type MemoryType string

const (
	TypeDecision   MemoryType = "decision"
	TypePreference MemoryType = "preference"
	TypeMilestone  MemoryType = "milestone"
	TypeProblem    MemoryType = "problem"
	TypeEmotional  MemoryType = "emotional"
)

// ExtractedMemory holds a classified sentence with its type and match score.
type ExtractedMemory struct {
	Type  MemoryType
	Text  string
	Score float64
}

// markers maps each memory type to its keyword markers.
var markers = map[MemoryType][]string{
	TypeDecision:   {"decided", "chose", "picked", "went with", "settled on"},
	TypePreference: {"always use", "never do", "prefer", "i like", "i hate", "favorite"},
	TypeMilestone:  {"shipped", "launched", "completed", "finished", "achieved", "deployed", "released"},
	TypeProblem:    {"bug", "error", "crash", "broken", "fix", "workaround", "root cause"},
	TypeEmotional:  {"frustrated", "excited", "worried", "happy", "stressed", "grateful", "anxious"},
}

// ExtractMemories splits text into sentences, scores each against the 5 marker
// sets, and returns classified memories. Sentences with no matches are excluded.
// If a sentence matches multiple types, the highest-scoring type wins.
func ExtractMemories(text string) []ExtractedMemory {
	if text == "" {
		return nil
	}

	sentences := splitSentences(text)
	var result []ExtractedMemory

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}
		lower := strings.ToLower(sent)

		var bestType MemoryType
		var bestScore float64

		for mtype, keywords := range markers {
			score := countMatches(lower, keywords)
			if score > bestScore {
				bestScore = score
				bestType = mtype
			}
		}

		if bestScore > 0 {
			result = append(result, ExtractedMemory{
				Type:  bestType,
				Text:  sent,
				Score: bestScore,
			})
		}
	}

	return result
}

// countMatches returns how many keywords appear in the lowered text.
func countMatches(lower string, keywords []string) float64 {
	var count float64
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			count++
		}
	}
	return count
}

// splitSentences splits text on sentence-ending punctuation (.!?) while
// preserving the delimiter attached to the preceding segment.
func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			sentences = append(sentences, text[start:i+1])
			start = i + 1
		}
	}
	// Trailing text without sentence-ending punctuation.
	if start < len(text) {
		tail := strings.TrimSpace(text[start:])
		if tail != "" {
			sentences = append(sentences, tail)
		}
	}
	return sentences
}
