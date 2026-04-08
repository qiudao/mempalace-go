package entity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// RegistryEntry represents a single entity in the registry.
type RegistryEntry struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`    // person, project
	Aliases []string `json:"aliases"`
	Source  string   `json:"source"`  // onboarding, learned, researched
	Code    string   `json:"code"`    // AAAK entity code
}

// registryFile is the on-disk JSON structure.
type registryFile struct {
	Entities []RegistryEntry `json:"entities"`
}

// Registry holds the in-memory entity registry and knows how to persist it.
type Registry struct {
	Entries []RegistryEntry `json:"entities"`
	path    string
}

// OpenRegistry loads a registry from path. If the file does not exist, an
// empty registry is returned that will write to that path on Save.
func OpenRegistry(path string) (*Registry, error) {
	r := &Registry{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, nil
		}
		return nil, err
	}

	var f registryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	r.Entries = f.Entities
	return r, nil
}

// Lookup finds an entry by name or alias (case-insensitive).
// Returns nil if not found.
func (r *Registry) Lookup(name string) *RegistryEntry {
	lower := strings.ToLower(name)
	for i := range r.Entries {
		if strings.ToLower(r.Entries[i].Name) == lower {
			return &r.Entries[i]
		}
		for _, alias := range r.Entries[i].Aliases {
			if strings.ToLower(alias) == lower {
				return &r.Entries[i]
			}
		}
	}
	return nil
}

// Add appends an entry to the registry. Returns an error if an entry with the
// same name already exists.
func (r *Registry) Add(entry RegistryEntry) error {
	if r.Lookup(entry.Name) != nil {
		return errors.New("entity already exists: " + entry.Name)
	}
	r.Entries = append(r.Entries, entry)
	return nil
}

// Save persists the registry to disk as JSON.
func (r *Registry) Save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f := registryFile{Entities: r.Entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}
