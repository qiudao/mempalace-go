package miner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func TestMineConvos(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "chat.txt"), []byte(
		"> user: Why did we pick Postgres?\n"+
			"assistant: Because of JSONB support and strong ecosystem.\n"+
			"> user: Makes sense.\n"+
			"assistant: Plus the extension ecosystem is unmatched.\n",
	), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := MineConvos(dir, palacePath, "test_convos")
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, _ := s.Count()
	if n == 0 {
		t.Fatal("expected conversation drawers")
	}
}

func TestMineConvosMultipleChunks(t *testing.T) {
	dir := t.TempDir()
	// 6 exchanges should produce 2 chunks (3 per chunk)
	os.WriteFile(filepath.Join(dir, "long_chat.txt"), []byte(
		"> user: Question 1\nassistant: Answer 1\n"+
			"> user: Question 2\nassistant: Answer 2\n"+
			"> user: Question 3\nassistant: Answer 3\n"+
			"> user: Question 4\nassistant: Answer 4\n"+
			"> user: Question 5\nassistant: Answer 5\n"+
			"> user: Question 6\nassistant: Answer 6\n",
	), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := MineConvos(dir, palacePath, "test_convos")
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, _ := s.Count()
	if n != 2 {
		t.Fatalf("expected 2 chunks, got %d", n)
	}
}

func TestMineConvosSkipsNonConvoFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("fake png"), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := MineConvos(dir, palacePath, "test_convos")
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, _ := s.Count()
	if n != 0 {
		t.Fatalf("expected 0 drawers for non-convo files, got %d", n)
	}
}

func TestSplitExchanges(t *testing.T) {
	text := "> user: Hello\nassistant: Hi there\n> user: How are you?\nassistant: Fine thanks"
	exchanges := splitExchanges(text)
	if len(exchanges) != 2 {
		t.Fatalf("expected 2 exchanges, got %d: %v", len(exchanges), exchanges)
	}
}

func TestGroupExchanges(t *testing.T) {
	exchanges := []string{"e1", "e2", "e3", "e4", "e5"}
	chunks := groupExchanges(exchanges, 3)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestDetectConvoRoom(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Let's talk about the database schema and postgres migration", "database"},
		{"The react component needs better CSS styling", "frontend"},
		{"Deploy to kubernetes with docker containers", "devops"},
		{"Nothing specific here", "general"},
	}
	for _, tt := range tests {
		got := detectConvoRoom(tt.text)
		if got != tt.want {
			t.Errorf("detectConvoRoom(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
