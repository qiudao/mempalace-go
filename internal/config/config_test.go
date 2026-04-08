package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg := Load(dir)
	if cfg.CollectionName != "mempalace_drawers" {
		t.Fatalf("expected default collection name, got %s", cfg.CollectionName)
	}
	if cfg.PalacePath == "" {
		t.Fatal("palace path should not be empty")
	}
}

func TestFromFile(t *testing.T) {
	dir := t.TempDir()
	data, _ := json.Marshal(map[string]string{"palace_path": "/custom/palace"})
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)

	cfg := Load(dir)
	if cfg.PalacePath != "/custom/palace" {
		t.Fatalf("expected /custom/palace, got %s", cfg.PalacePath)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEMPALACE_PALACE_PATH", "/env/path")
	cfg := Load(dir)
	if cfg.PalacePath != "/env/path" {
		t.Fatalf("expected /env/path, got %s", cfg.PalacePath)
	}
}
