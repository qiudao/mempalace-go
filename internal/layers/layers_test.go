package layers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func setup(t *testing.T) *MemoryStack {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	s.Add(store.Drawer{ID: "d1", Document: "GraphQL API design", Wing: "backend", Room: "architecture"})
	s.Add(store.Drawer{ID: "d2", Document: "React component patterns", Wing: "frontend", Room: "ui"})

	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "identity.txt"), []byte("I am a software engineer"), 0644)

	return &MemoryStack{Store: s, ConfigDir: configDir}
}

func TestWakeUp(t *testing.T) {
	m := setup(t)
	text, err := m.WakeUp()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "software engineer") {
		t.Fatal("expected identity in wake-up")
	}
	if !strings.Contains(text, "2 memories") {
		t.Fatal("expected essential story in wake-up")
	}
}

func TestRecall(t *testing.T) {
	m := setup(t)
	drawers, err := m.Recall("backend", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(drawers) != 1 {
		t.Fatalf("expected 1 backend drawer, got %d", len(drawers))
	}
}

func TestSearchL3(t *testing.T) {
	m := setup(t)
	results, err := m.Search("GraphQL", 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
}

func TestStatus(t *testing.T) {
	m := setup(t)
	status, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "2 drawers") {
		t.Fatal("expected drawer count in status")
	}
}
