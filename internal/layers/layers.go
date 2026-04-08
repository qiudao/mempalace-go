package layers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mempalace/mempalace-go/internal/store"
)

// MemoryStack implements the 4-layer memory model:
//
//	L0 (~100 tokens)  — Identity: reads ~/.mempalace/identity.txt
//	L1 (~500-800 tok) — Essential story: auto-generated summary of the palace
//	L2               — On-demand recall: filtered retrieval by wing/room
//	L3               — Full search: FTS5 search across everything
type MemoryStack struct {
	Store     *store.Store
	ConfigDir string // e.g. ~/.mempalace
}

// WakeUp returns L0 (identity) + L1 (essential story).
func (m *MemoryStack) WakeUp() (string, error) {
	l0 := m.loadIdentity()
	l1, err := m.essentialStory()
	if err != nil {
		return l0, err
	}
	return l0 + "\n\n" + l1, nil
}

// Recall returns L2 — filtered retrieval by wing/room.
func (m *MemoryStack) Recall(wing, room string, limit int) ([]store.Drawer, error) {
	if limit <= 0 {
		limit = 20
	}
	return m.Store.Get(store.Query{Wing: wing, Room: room, Limit: limit})
}

// Search returns L3 — full FTS5 search across everything.
func (m *MemoryStack) Search(query string, limit int, wing, room string) ([]store.SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	return m.Store.Search(query, limit, store.Query{Wing: wing, Room: room})
}

// Status returns a human-readable summary of what's in the palace.
func (m *MemoryStack) Status() (string, error) {
	drawers, err := m.Store.Get(store.Query{})
	if err != nil {
		return "", err
	}

	wings := make(map[string]map[string]int)
	for _, d := range drawers {
		if wings[d.Wing] == nil {
			wings[d.Wing] = make(map[string]int)
		}
		wings[d.Wing][d.Room]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Palace: %d drawers\n", len(drawers)))
	for wing, rooms := range wings {
		sb.WriteString(fmt.Sprintf("\n  Wing: %s\n", wing))
		for room, count := range rooms {
			sb.WriteString(fmt.Sprintf("    %s: %d drawers\n", room, count))
		}
	}
	return sb.String(), nil
}

func (m *MemoryStack) loadIdentity() string {
	path := filepath.Join(m.ConfigDir, "identity.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (m *MemoryStack) essentialStory() (string, error) {
	count, err := m.Store.Count()
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "Palace is empty. Start by mining some data.", nil
	}

	drawers, err := m.Store.Get(store.Query{})
	if err != nil {
		return "", err
	}

	wings := make(map[string]int)
	rooms := make(map[string]int)
	for _, d := range drawers {
		wings[d.Wing]++
		rooms[d.Room]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Palace has %d memories across %d wings and %d rooms.\n", count, len(wings), len(rooms)))
	sb.WriteString("Wings: ")
	first := true
	for wing, n := range wings {
		if !first {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s (%d)", wing, n))
		first = false
	}
	return sb.String(), nil
}
