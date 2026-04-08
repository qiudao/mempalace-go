// Package mcp implements an MCP (Model Context Protocol) server
// that exposes mempalace tools over JSON-RPC 2.0 on stdin/stdout.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mempalace/mempalace-go/internal/graph"
	"github.com/mempalace/mempalace-go/internal/store"
)

// Server is an MCP server that dispatches tool calls to mempalace.
type Server struct {
	store *store.Store
	kg    *graph.KnowledgeGraph
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// NewServer creates a new MCP server backed by the given store and knowledge graph.
// kg may be nil if knowledge graph features are not available.
func NewServer(s *store.Store, kg *graph.KnowledgeGraph) *Server {
	return &Server{store: s, kg: kg}
}

// --- JSON-RPC types ---

type jsonrpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Tool definitions ---

// ListTools returns the list of tools this server exposes.
func (s *Server) ListTools() []Tool {
	tools := []Tool{
		{
			Name:        "mempalace_status",
			Description: "Show palace statistics: wing count, room count, drawer count, connections.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "mempalace_search",
			Description: "Full-text search across all drawers with optional wing/room filters.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query"},
					"wing":  map[string]any{"type": "string", "description": "Filter by wing"},
					"room":  map[string]any{"type": "string", "description": "Filter by room"},
					"limit": map[string]any{"type": "number", "description": "Max results (default 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "mempalace_list_wings",
			Description: "List all wings in the palace.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "mempalace_list_rooms",
			Description: "List rooms in a wing.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"wing": map[string]any{"type": "string", "description": "Wing name"},
				},
				"required": []string{"wing"},
			},
		},
		{
			Name:        "mempalace_add_drawer",
			Description: "Add a new drawer (memory) to the palace.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":       map[string]any{"type": "string", "description": "Unique drawer ID"},
					"document": map[string]any{"type": "string", "description": "Content text"},
					"wing":     map[string]any{"type": "string", "description": "Wing name"},
					"room":     map[string]any{"type": "string", "description": "Room name"},
					"source":   map[string]any{"type": "string", "description": "Source reference"},
				},
				"required": []string{"id", "document", "wing", "room"},
			},
		},
		{
			Name:        "mempalace_delete_drawer",
			Description: "Delete a drawer by ID.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "description": "Drawer ID to delete"},
				},
				"required": []string{"id"},
			},
		},
	}

	// Add KG tools if knowledge graph is available
	if s.kg != nil {
		tools = append(tools,
			Tool{
				Name:        "mempalace_kg_query",
				Description: "Query knowledge graph triples for an entity.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"subject": map[string]any{"type": "string", "description": "Entity name to query"},
					},
					"required": []string{"subject"},
				},
			},
			Tool{
				Name:        "mempalace_kg_add",
				Description: "Add a triple (subject-predicate-object) to the knowledge graph.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"subject":   map[string]any{"type": "string", "description": "Subject entity"},
						"predicate": map[string]any{"type": "string", "description": "Relationship"},
						"object":    map[string]any{"type": "string", "description": "Object value"},
					},
					"required": []string{"subject", "predicate", "object"},
				},
			},
			Tool{
				Name:        "mempalace_traverse_graph",
				Description: "Traverse the palace graph from a starting room via BFS.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"start_room": map[string]any{"type": "string", "description": "Starting room in wing/room format"},
						"max_hops":   map[string]any{"type": "number", "description": "Max BFS hops (default 2)"},
					},
					"required": []string{"start_room"},
				},
			},
		)
	}

	return tools
}

// CallTool dispatches a tool call by name with the given arguments.
func (s *Server) CallTool(name string, args map[string]any) (any, error) {
	switch name {
	case "mempalace_status":
		return s.toolStatus()
	case "mempalace_search":
		return s.toolSearch(args)
	case "mempalace_list_wings":
		return s.toolListWings()
	case "mempalace_list_rooms":
		return s.toolListRooms(args)
	case "mempalace_add_drawer":
		return s.toolAddDrawer(args)
	case "mempalace_delete_drawer":
		return s.toolDeleteDrawer(args)
	case "mempalace_kg_query":
		return s.toolKGQuery(args)
	case "mempalace_kg_add":
		return s.toolKGAdd(args)
	case "mempalace_traverse_graph":
		return s.toolTraverseGraph(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Tool implementations ---

func (s *Server) toolStatus() (any, error) {
	stats, err := graph.Stats(s.store)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"wings":       stats.Wings,
		"rooms":       stats.Rooms,
		"drawers":     stats.Drawers,
		"connections": stats.Connections,
	}
	if s.kg != nil {
		kgStats, err := s.kg.Stats()
		if err == nil {
			result["kg_entities"] = kgStats.Entities
			result["kg_triples"] = kgStats.Triples
		}
	}
	return result, nil
}

func (s *Server) toolSearch(args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	q := store.Query{}
	if w, ok := args["wing"].(string); ok {
		q.Wing = w
	}
	if r, ok := args["room"].(string); ok {
		q.Room = r
	}

	results, err := s.store.Search(query, limit, q)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, len(results))
	for i, r := range results {
		out[i] = map[string]any{
			"id":       r.ID,
			"document": r.Document,
			"wing":     r.Wing,
			"room":     r.Room,
			"source":   r.Source,
			"rank":     r.Rank,
		}
	}
	return map[string]any{"results": out, "count": len(out)}, nil
}

func (s *Server) toolListWings() (any, error) {
	drawers, err := s.store.Get(store.Query{})
	if err != nil {
		return nil, err
	}
	wingSet := make(map[string]int)
	for _, d := range drawers {
		wingSet[d.Wing]++
	}
	type wingInfo struct {
		Name    string `json:"name"`
		Drawers int    `json:"drawers"`
	}
	wings := make([]wingInfo, 0, len(wingSet))
	for name, count := range wingSet {
		wings = append(wings, wingInfo{Name: name, Drawers: count})
	}
	return map[string]any{"wings": wings}, nil
}

func (s *Server) toolListRooms(args map[string]any) (any, error) {
	wing, _ := args["wing"].(string)
	if wing == "" {
		return nil, fmt.Errorf("wing is required")
	}
	drawers, err := s.store.Get(store.Query{Wing: wing})
	if err != nil {
		return nil, err
	}
	roomSet := make(map[string]int)
	for _, d := range drawers {
		roomSet[d.Room]++
	}
	type roomInfo struct {
		Name    string `json:"name"`
		Drawers int    `json:"drawers"`
	}
	rooms := make([]roomInfo, 0, len(roomSet))
	for name, count := range roomSet {
		rooms = append(rooms, roomInfo{Name: name, Drawers: count})
	}
	return map[string]any{"rooms": rooms, "wing": wing}, nil
}

func (s *Server) toolAddDrawer(args map[string]any) (any, error) {
	id, _ := args["id"].(string)
	doc, _ := args["document"].(string)
	wing, _ := args["wing"].(string)
	room, _ := args["room"].(string)
	source, _ := args["source"].(string)

	if id == "" || doc == "" || wing == "" || room == "" {
		return nil, fmt.Errorf("id, document, wing, and room are required")
	}

	d := store.Drawer{
		ID:       id,
		Document: doc,
		Wing:     wing,
		Room:     room,
		Source:   source,
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.store.Add(d); err != nil {
		return nil, err
	}
	return map[string]any{"status": "added", "id": id}, nil
}

func (s *Server) toolDeleteDrawer(args map[string]any) (any, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if err := s.store.Delete([]string{id}); err != nil {
		return nil, err
	}
	return map[string]any{"status": "deleted", "id": id}, nil
}

func (s *Server) toolKGQuery(args map[string]any) (any, error) {
	if s.kg == nil {
		return nil, fmt.Errorf("knowledge graph not available")
	}
	subject, _ := args["subject"].(string)
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	triples, err := s.kg.QueryEntity(subject)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, len(triples))
	for i, t := range triples {
		out[i] = map[string]string{
			"subject":    t.Subject,
			"predicate":  t.Predicate,
			"object":     t.Object,
			"valid_from": t.ValidFrom,
		}
	}
	return map[string]any{"triples": out, "count": len(out)}, nil
}

func (s *Server) toolKGAdd(args map[string]any) (any, error) {
	if s.kg == nil {
		return nil, fmt.Errorf("knowledge graph not available")
	}
	subject, _ := args["subject"].(string)
	predicate, _ := args["predicate"].(string)
	object, _ := args["object"].(string)
	if subject == "" || predicate == "" || object == "" {
		return nil, fmt.Errorf("subject, predicate, and object are required")
	}
	validFrom := time.Now().UTC().Format("2006-01-02")
	if err := s.kg.AddTriple(subject, predicate, object, validFrom); err != nil {
		return nil, err
	}
	return map[string]any{"status": "added", "subject": subject, "predicate": predicate, "object": object}, nil
}

func (s *Server) toolTraverseGraph(args map[string]any) (any, error) {
	startRoom, _ := args["start_room"].(string)
	if startRoom == "" {
		return nil, fmt.Errorf("start_room is required")
	}
	maxHops := 2
	if h, ok := args["max_hops"].(float64); ok {
		maxHops = int(h)
	}
	drawers, err := graph.Traverse(s.store, startRoom, maxHops)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, len(drawers))
	for i, d := range drawers {
		out[i] = map[string]string{
			"id":       d.ID,
			"document": d.Document,
			"wing":     d.Wing,
			"room":     d.Room,
		}
	}
	return map[string]any{"drawers": out, "count": len(out)}, nil
}

// --- JSON-RPC stdio loop ---

// Run reads JSON-RPC requests from stdin and writes responses to stdout.
func (s *Server) Run() error {
	return s.RunIO(os.Stdin, os.Stdout)
}

// RunIO reads JSON-RPC requests from r and writes responses to w.
func (s *Server) RunIO(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   jsonrpcError{Code: -32700, Message: "Parse error"},
			}
			writeResponse(w, resp)
			continue
		}

		resp := s.handleRequest(req)
		writeResponse(w, resp)
	}
	return scanner.Err()
}

func (s *Server) handleRequest(req jsonrpcRequest) jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "mempalace",
					"version": "0.1.0",
				},
			},
		}

	case "notifications/initialized":
		// Client acknowledgement, no response needed for notifications
		return jsonrpcResponse{JSONRPC: "2.0", ID: req.ID}

	case "tools/list":
		tools := s.ListTools()
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}

	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)
		if args == nil {
			args = map[string]any{}
		}

		result, err := s.CallTool(name, args)
		if err != nil {
			return jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
					},
					"isError": true,
				},
			}
		}

		text, _ := json.Marshal(result)
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}

	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   jsonrpcError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func writeResponse(w io.Writer, resp jsonrpcResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}
