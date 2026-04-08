package entity

import "testing"

func TestDetectPerson(t *testing.T) {
	text := "Alice told me about the project. Bob said it was ready."
	entities := DetectEntities(text)
	found := false
	for _, e := range entities {
		if e.Name == "Alice" && e.Type == "person" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Alice as person, got %+v", entities)
	}
}

func TestDetectProject(t *testing.T) {
	text := "We deployed MemPalace to production. The MemPalace repository was updated."
	entities := DetectEntities(text)
	found := false
	for _, e := range entities {
		if e.Name == "MemPalace" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MemPalace detected, got %+v", entities)
	}
}

func TestDetectPersonMultipleSignals(t *testing.T) {
	// Ensure person classification with multiple signal categories.
	text := `Alice said hello. Alice told me the news.
She went to the store. Alice asked about it.
Hey Alice, are you coming? Alice laughed at the joke.`
	entities := DetectEntities(text)
	var alice *Entity
	for i, e := range entities {
		if e.Name == "Alice" {
			alice = &entities[i]
			break
		}
	}
	if alice == nil {
		t.Fatalf("expected Alice detected, got %+v", entities)
	}
	if alice.Type != "person" {
		t.Errorf("expected Alice type=person, got %q", alice.Type)
	}
}

func TestDetectProjectSignals(t *testing.T) {
	text := `We are building MemPalace. We deployed MemPalace to prod.
The MemPalace repo is ready. Shipped MemPalace v2.`
	entities := DetectEntities(text)
	var mp *Entity
	for i, e := range entities {
		if e.Name == "MemPalace" {
			mp = &entities[i]
			break
		}
	}
	if mp == nil {
		t.Fatalf("expected MemPalace detected, got %+v", entities)
	}
	if mp.Type != "project" {
		t.Errorf("expected MemPalace type=project, got %q", mp.Type)
	}
}

func TestStopwordsFiltered(t *testing.T) {
	text := "The system will check every value."
	entities := DetectEntities(text)
	for _, e := range entities {
		if e.Name == "The" || e.Name == "Every" {
			t.Errorf("stopword should be filtered: %s", e.Name)
		}
	}
}
