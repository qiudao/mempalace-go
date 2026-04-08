package miner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func TestMineProject(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.WriteFile(filepath.Join(dir, "backend", "app.py"),
		[]byte(strings.Repeat("def main():\n    print('hello world')\n", 20)), 0644)

	os.WriteFile(filepath.Join(dir, "mempalace.yaml"), []byte("wing: test_project\nrooms:\n  - name: backend\n    description: Backend code\n  - name: general\n    description: General\n"), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := Mine(dir, palacePath)
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
		t.Fatal("expected drawers to be mined")
	}
}

func TestMineSkipsDirs(t *testing.T) {
	dir := t.TempDir()
	// Create skippable dirs with files
	for _, d := range []string{".git", "node_modules", "__pycache__"} {
		p := filepath.Join(dir, d)
		os.MkdirAll(p, 0755)
		os.WriteFile(filepath.Join(p, "file.txt"), []byte("should be skipped"), 0644)
	}
	// Create a real file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello world content here"), 0644)
	os.WriteFile(filepath.Join(dir, "mempalace.yaml"), []byte("wing: test\nrooms: []\n"), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := Mine(dir, palacePath)
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, _ := s.Count()
	if n != 1 {
		t.Fatalf("expected 1 drawer (only readme.txt), got %d", n)
	}
}

func TestMineSkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("fake png"), 0644)
	os.WriteFile(filepath.Join(dir, "code.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(dir, "mempalace.yaml"), []byte("wing: test\nrooms: []\n"), 0644)

	palacePath := filepath.Join(dir, "palace")
	err := Mine(dir, palacePath)
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, _ := s.Count()
	if n != 1 {
		t.Fatalf("expected 1 drawer (only code.py), got %d", n)
	}
}

func TestMineMissingConfig(t *testing.T) {
	dir := t.TempDir()
	err := Mine(dir, filepath.Join(dir, "palace"))
	if err == nil {
		t.Fatal("expected error for missing mempalace.yaml")
	}
}
