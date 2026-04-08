package entity

import (
	"path/filepath"
	"testing"
)

func TestRegistryAddAndLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, err := OpenRegistry(path)
	if err != nil {
		t.Fatal(err)
	}

	r.Add(RegistryEntry{Name: "Alice", Type: "person", Code: "A1", Source: "onboarding"})
	r.Save()

	// Reload
	r2, _ := OpenRegistry(path)
	entry := r2.Lookup("Alice")
	if entry == nil || entry.Code != "A1" {
		t.Fatalf("lookup failed: %+v", entry)
	}
}

func TestRegistryLookupAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := OpenRegistry(path)
	r.Add(RegistryEntry{Name: "Robert", Type: "person", Code: "R1", Aliases: []string{"Bob", "Rob"}})
	r.Save()

	r2, _ := OpenRegistry(path)
	entry := r2.Lookup("Bob")
	if entry == nil || entry.Name != "Robert" {
		t.Fatalf("alias lookup failed: %+v", entry)
	}
}

func TestRegistryDuplicateAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := OpenRegistry(path)
	r.Add(RegistryEntry{Name: "Alice", Type: "person", Code: "A1"})
	err := r.Add(RegistryEntry{Name: "Alice", Type: "person", Code: "A2"})
	if err == nil {
		t.Fatal("expected error on duplicate add")
	}
}

func TestRegistryLookupCaseInsensitive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := OpenRegistry(path)
	r.Add(RegistryEntry{Name: "Alice", Type: "person", Code: "A1"})

	entry := r.Lookup("alice")
	if entry == nil || entry.Name != "Alice" {
		t.Fatalf("case-insensitive lookup failed: %+v", entry)
	}
}

func TestRegistryOpenNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does_not_exist.json")
	r, err := OpenRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Entries) != 0 {
		t.Fatalf("expected empty registry, got %d entries", len(r.Entries))
	}
}
