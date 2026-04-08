package miner

import "testing"

func TestExtractDecision(t *testing.T) {
	memories := ExtractMemories("We decided to use PostgreSQL for the main database.")
	if len(memories) == 0 {
		t.Fatal("expected a decision memory")
	}
	if memories[0].Type != TypeDecision {
		t.Fatalf("expected decision type, got %s", memories[0].Type)
	}
}

func TestExtractPreference(t *testing.T) {
	memories := ExtractMemories("I always use vim for editing. I prefer dark themes.")
	found := false
	for _, m := range memories {
		if m.Type == TypePreference {
			found = true
		}
	}
	if !found {
		t.Fatal("expected preference memory")
	}
}

func TestExtractMilestone(t *testing.T) {
	memories := ExtractMemories("We shipped the new API yesterday. The migration was completed.")
	found := false
	for _, m := range memories {
		if m.Type == TypeMilestone {
			found = true
		}
	}
	if !found {
		t.Fatal("expected milestone memory")
	}
}

func TestExtractProblem(t *testing.T) {
	memories := ExtractMemories("There's a critical bug in the auth module. The crash happens on login.")
	found := false
	for _, m := range memories {
		if m.Type == TypeProblem {
			found = true
		}
	}
	if !found {
		t.Fatal("expected problem memory")
	}
}

func TestExtractEmotional(t *testing.T) {
	memories := ExtractMemories("I'm really frustrated with this deployment process. I feel stressed.")
	found := false
	for _, m := range memories {
		if m.Type == TypeEmotional {
			found = true
		}
	}
	if !found {
		t.Fatal("expected emotional memory")
	}
}

func TestExtractNoMatch(t *testing.T) {
	memories := ExtractMemories("The weather is nice today.")
	if len(memories) != 0 {
		t.Fatalf("expected no extracted memories, got %d", len(memories))
	}
}

func TestExtractEmpty(t *testing.T) {
	memories := ExtractMemories("")
	if len(memories) != 0 {
		t.Fatal("expected no memories from empty text")
	}
}
