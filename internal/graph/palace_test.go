package graph

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/mempalace/mempalace-go/internal/store"
)

func tempPalaceStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	// Seed: project_a/backend and project_a/frontend share "shared.py"
	// project_b/backend has a different source
	s.Add(store.Drawer{ID: "d1", Document: "backend api", Wing: "project_a", Room: "backend", Source: "shared.py"})
	s.Add(store.Drawer{ID: "d2", Document: "frontend ui", Wing: "project_a", Room: "frontend", Source: "shared.py"})
	s.Add(store.Drawer{ID: "d3", Document: "auth module", Wing: "project_b", Room: "backend", Source: "auth.py"})
	return s
}

func TestBuildGraph(t *testing.T) {
	s := tempPalaceStore(t)
	adj, err := BuildGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	// project_a/backend and project_a/frontend share "shared.py" — connected
	neighbors := adj["project_a/backend"]
	if len(neighbors) == 0 {
		t.Fatal("expected project_a/backend to have neighbors")
	}
	found := false
	for _, n := range neighbors {
		if n == "project_a/frontend" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected project_a/frontend in neighbors of project_a/backend, got %v", neighbors)
	}

	// project_b/backend has no shared source with others — no connections
	if len(adj["project_b/backend"]) != 0 {
		t.Fatalf("expected project_b/backend to have no neighbors, got %v", adj["project_b/backend"])
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	adj, err := BuildGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(adj) != 0 {
		t.Fatalf("expected empty graph, got %d entries", len(adj))
	}
}

func TestTraverse(t *testing.T) {
	s := tempPalaceStore(t)
	drawers, err := Traverse(s, "project_a/backend", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Should get drawers from project_a/backend and its 1-hop neighbor project_a/frontend
	if len(drawers) < 2 {
		t.Fatalf("expected at least 2 drawers from traverse, got %d", len(drawers))
	}

	// Verify we got drawers from both rooms
	rooms := make(map[string]bool)
	for _, d := range drawers {
		rooms[d.Wing+"/"+d.Room] = true
	}
	if !rooms["project_a/backend"] || !rooms["project_a/frontend"] {
		t.Fatalf("expected drawers from both backend and frontend rooms, got rooms: %v", rooms)
	}
}

func TestTraverseZeroHops(t *testing.T) {
	s := tempPalaceStore(t)
	drawers, err := Traverse(s, "project_a/backend", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Zero hops: only drawers from start room
	for _, d := range drawers {
		if d.Wing+"/"+d.Room != "project_a/backend" {
			t.Fatalf("zero-hop traverse returned drawer from %s/%s", d.Wing, d.Room)
		}
	}
}

func TestFindTunnels(t *testing.T) {
	s := tempPalaceStore(t)
	tunnels, err := FindTunnels(s, "project_a", "project_b")
	if err != nil {
		t.Fatal(err)
	}
	// "backend" room exists in both wings
	sort.Strings(tunnels)
	if len(tunnels) != 1 || tunnels[0] != "backend" {
		t.Fatalf("expected [backend] as tunnels, got %v", tunnels)
	}
}

func TestFindTunnelsNone(t *testing.T) {
	s := tempPalaceStore(t)
	// Add a drawer in a unique wing
	s.Add(store.Drawer{ID: "d4", Document: "solo", Wing: "project_c", Room: "unique_room", Source: "solo.py"})
	tunnels, err := FindTunnels(s, "project_a", "project_c")
	if err != nil {
		t.Fatal(err)
	}
	if len(tunnels) != 0 {
		t.Fatalf("expected no tunnels, got %v", tunnels)
	}
}

func TestPalaceStats(t *testing.T) {
	s := tempPalaceStore(t)
	stats, err := Stats(s)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Wings != 2 {
		t.Fatalf("expected 2 wings, got %d", stats.Wings)
	}
	if stats.Rooms != 3 {
		t.Fatalf("expected 3 rooms, got %d", stats.Rooms)
	}
	if stats.Drawers != 3 {
		t.Fatalf("expected 3 drawers, got %d", stats.Drawers)
	}
	if stats.Connections != 1 {
		t.Fatalf("expected 1 connection, got %d", stats.Connections)
	}
}
