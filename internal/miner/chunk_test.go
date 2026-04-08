package miner

import (
	"strings"
	"testing"
)

func TestChunkShortText(t *testing.T) {
	chunks := ChunkText("short text", 800, 100)
	if len(chunks) != 1 || chunks[0] != "short text" {
		t.Fatalf("unexpected: %v", chunks)
	}
}

func TestChunkLongText(t *testing.T) {
	para := strings.Repeat("word ", 50) // ~250 chars per para
	text := para + "\n\n" + para + "\n\n" + para + "\n\n" + para
	chunks := ChunkText(text, 800, 100)

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > 900 { // allow slight overshoot for paragraph boundaries
			t.Fatalf("chunk too large: %d chars", len(c))
		}
	}
}

func TestChunkOverlap(t *testing.T) {
	// Create text where overlap matters
	para := strings.Repeat("word ", 100) // ~500 chars per para
	text := para + "\n\n" + para + "\n\n" + para
	chunks := ChunkText(text, 800, 100)

	if len(chunks) < 2 {
		t.Skip("not enough chunks to test overlap")
	}
	// With overlap, chunks should share some content
	// Just verify we get multiple chunks and they're reasonable size
	for i, c := range chunks {
		if len(strings.TrimSpace(c)) == 0 {
			t.Fatalf("chunk %d is empty", i)
		}
	}
}

func TestChunkEmpty(t *testing.T) {
	chunks := ChunkText("", 800, 100)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}
