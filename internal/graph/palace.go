package graph

import (
	"github.com/mempalace/mempalace-go/internal/store"
)

// PalaceStats holds summary statistics about the palace structure.
type PalaceStats struct {
	Wings       int
	Rooms       int
	Drawers     int
	Connections int // undirected edges between rooms
}

// BuildGraph builds an adjacency list from drawer metadata.
// Two rooms (keyed as "wing/room") are connected if they share a source file.
func BuildGraph(s *store.Store) (map[string][]string, error) {
	drawers, err := s.Get(store.Query{})
	if err != nil {
		return nil, err
	}

	// Map source -> set of rooms that reference it
	sourceRooms := make(map[string]map[string]bool)
	for _, d := range drawers {
		if d.Source == "" {
			continue
		}
		key := d.Wing + "/" + d.Room
		if sourceRooms[d.Source] == nil {
			sourceRooms[d.Source] = make(map[string]bool)
		}
		sourceRooms[d.Source][key] = true
	}

	// Build adjacency: rooms connected via shared sources
	// Use a set to deduplicate edges
	edgeSet := make(map[string]map[string]bool)
	for _, rooms := range sourceRooms {
		roomList := make([]string, 0, len(rooms))
		for r := range rooms {
			roomList = append(roomList, r)
		}
		for i := 0; i < len(roomList); i++ {
			for j := i + 1; j < len(roomList); j++ {
				a, b := roomList[i], roomList[j]
				if edgeSet[a] == nil {
					edgeSet[a] = make(map[string]bool)
				}
				if edgeSet[b] == nil {
					edgeSet[b] = make(map[string]bool)
				}
				edgeSet[a][b] = true
				edgeSet[b][a] = true
			}
		}
	}

	adj := make(map[string][]string, len(edgeSet))
	for node, neighbors := range edgeSet {
		for n := range neighbors {
			adj[node] = append(adj[node], n)
		}
	}
	return adj, nil
}

// Traverse does BFS from startRoom (in "wing/room" format) up to maxHops,
// returning drawers from the start room and all reachable rooms.
func Traverse(s *store.Store, startRoom string, maxHops int) ([]store.Drawer, error) {
	adj, err := BuildGraph(s)
	if err != nil {
		return nil, err
	}

	visited := map[string]bool{startRoom: true}
	queue := []string{startRoom}

	for hop := 0; hop < maxHops && len(queue) > 0; hop++ {
		var next []string
		for _, room := range queue {
			for _, neighbor := range adj[room] {
				if !visited[neighbor] {
					visited[neighbor] = true
					next = append(next, neighbor)
				}
			}
		}
		queue = next
	}

	// Collect drawers from all visited rooms
	allDrawers, err := s.Get(store.Query{})
	if err != nil {
		return nil, err
	}

	var result []store.Drawer
	for _, d := range allDrawers {
		key := d.Wing + "/" + d.Room
		if visited[key] {
			result = append(result, d)
		}
	}
	return result, nil
}

// FindTunnels finds room names that appear in both wingA and wingB.
func FindTunnels(s *store.Store, wingA, wingB string) ([]string, error) {
	drawersA, err := s.Get(store.Query{Wing: wingA})
	if err != nil {
		return nil, err
	}
	drawersB, err := s.Get(store.Query{Wing: wingB})
	if err != nil {
		return nil, err
	}

	roomsA := make(map[string]bool)
	for _, d := range drawersA {
		roomsA[d.Room] = true
	}

	tunnels := make(map[string]bool)
	for _, d := range drawersB {
		if roomsA[d.Room] {
			tunnels[d.Room] = true
		}
	}

	result := make([]string, 0, len(tunnels))
	for room := range tunnels {
		result = append(result, room)
	}
	return result, nil
}

// Stats returns summary statistics about the palace.
func Stats(s *store.Store) (PalaceStats, error) {
	drawers, err := s.Get(store.Query{})
	if err != nil {
		return PalaceStats{}, err
	}

	wings := make(map[string]bool)
	rooms := make(map[string]bool)
	for _, d := range drawers {
		wings[d.Wing] = true
		rooms[d.Wing+"/"+d.Room] = true
	}

	adj, err := BuildGraph(s)
	if err != nil {
		return PalaceStats{}, err
	}
	conns := 0
	for _, neighbors := range adj {
		conns += len(neighbors)
	}
	conns /= 2 // undirected

	return PalaceStats{
		Wings:       len(wings),
		Rooms:       len(rooms),
		Drawers:     len(drawers),
		Connections: conns,
	}, nil
}

