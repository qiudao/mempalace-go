package mcp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mempalace/mempalace-go/internal/graph"
	"github.com/mempalace/mempalace-go/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()

	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	kg, err := graph.OpenKnowledgeGraph(filepath.Join(dir, "kg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kg.Close() })

	// Seed some data
	for _, d := range []store.Drawer{
		{ID: "d1", Document: "GraphQL schema design patterns", Wing: "engineering", Room: "api", Source: "notes.md"},
		{ID: "d2", Document: "REST API best practices", Wing: "engineering", Room: "api", Source: "blog.md"},
		{ID: "d3", Document: "Project roadmap for Q1", Wing: "planning", Room: "roadmap", Source: "plan.md"},
		{ID: "d4", Document: "Team standup notes from Monday", Wing: "engineering", Room: "standup", Source: "notes.md"},
	} {
		if err := s.Add(d); err != nil {
			t.Fatal(err)
		}
	}

	return NewServer(s, kg)
}

func TestListTools(t *testing.T) {
	s := setupTestServer(t)
	tools := s.ListTools()
	if len(tools) < 6 {
		t.Fatalf("expected at least 6 tools, got %d", len(tools))
	}
	// Verify core tools present
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"mempalace_status", "mempalace_search", "mempalace_list_wings", "mempalace_list_rooms", "mempalace_add_drawer", "mempalace_delete_drawer"} {
		if !names[want] {
			t.Errorf("missing tool: %s", want)
		}
	}
}

func TestListToolsNoKG(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := NewServer(s, nil)
	tools := srv.ListTools()
	if len(tools) != 6 {
		t.Fatalf("expected 6 tools without KG, got %d", len(tools))
	}
}

func TestStatusTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_status", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["drawers"] != 4 {
		t.Errorf("expected 4 drawers, got %v", m["drawers"])
	}
	if m["wings"] != 2 {
		t.Errorf("expected 2 wings, got %v", m["wings"])
	}
}

func TestSearchTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_search", map[string]any{"query": "GraphQL", "limit": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["count"].(int) < 1 {
		t.Fatal("expected at least 1 search result")
	}
}

func TestSearchToolWithFilters(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_search", map[string]any{
		"query": "API",
		"wing":  "engineering",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["count"].(int) < 1 {
		t.Fatal("expected results")
	}
}

func TestSearchToolMissingQuery(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.CallTool("mempalace_search", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestListWingsTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_list_wings", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	wings := m["wings"]
	if wings == nil {
		t.Fatal("expected wings")
	}
}

func TestListRoomsTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_list_rooms", map[string]any{"wing": "engineering"})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["wing"] != "engineering" {
		t.Errorf("expected wing=engineering, got %v", m["wing"])
	}
}

func TestListRoomsMissingWing(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.CallTool("mempalace_list_rooms", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing wing")
	}
}

func TestAddDrawerTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_add_drawer", map[string]any{
		"id":       "new1",
		"document": "Test document content",
		"wing":     "test",
		"room":     "unit",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["status"] != "added" {
		t.Errorf("expected status=added, got %v", m["status"])
	}

	// Verify it was actually added
	statusResult, err := s.CallTool("mempalace_status", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if statusResult.(map[string]any)["drawers"] != 5 {
		t.Error("expected 5 drawers after add")
	}
}

func TestAddDrawerMissingFields(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.CallTool("mempalace_add_drawer", map[string]any{
		"id": "x",
	})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestDeleteDrawerTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_delete_drawer", map[string]any{"id": "d1"})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", m["status"])
	}

	// Verify count decreased
	statusResult, err := s.CallTool("mempalace_status", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if statusResult.(map[string]any)["drawers"] != 3 {
		t.Error("expected 3 drawers after delete")
	}
}

func TestDeleteDrawerMissingID(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.CallTool("mempalace_delete_drawer", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestKGQueryTool(t *testing.T) {
	s := setupTestServer(t)
	// Add a triple first
	_, err := s.CallTool("mempalace_kg_add", map[string]any{
		"subject":   "Go",
		"predicate": "used_for",
		"object":    "backend",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Query it
	result, err := s.CallTool("mempalace_kg_query", map[string]any{"subject": "Go"})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["count"].(int) != 1 {
		t.Errorf("expected 1 triple, got %v", m["count"])
	}
}

func TestKGToolsWithoutKG(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := NewServer(s, nil)
	_, err = srv.CallTool("mempalace_kg_query", map[string]any{"subject": "Go"})
	if err == nil {
		t.Fatal("expected error when KG not available")
	}
}

func TestTraverseGraphTool(t *testing.T) {
	s := setupTestServer(t)
	result, err := s.CallTool("mempalace_traverse_graph", map[string]any{
		"start_room": "engineering/api",
		"max_hops":   1.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["count"] == nil {
		t.Fatal("expected count in result")
	}
}

func TestUnknownTool(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.CallTool("nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

// --- JSON-RPC protocol tests ---

func TestJSONRPCInitialize(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
	}
	if resp.ID != float64(1) {
		t.Errorf("expected id=1, got %v", resp.ID)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] == nil {
		t.Error("expected protocolVersion in result")
	}
}

func TestJSONRPCToolsList(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) < 6 {
		t.Fatalf("expected at least 6 tools, got %d", len(tools))
	}
}

func TestJSONRPCToolsCall(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mempalace_status","arguments":{}}}` + "\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp.Result.(map[string]any)
	content := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in tool result")
	}
}

func TestJSONRPCToolsCallError(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"mempalace_search","arguments":{}}}` + "\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Error("expected isError=true for tool error")
	}
}

func TestJSONRPCMethodNotFound(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":5,"method":"bogus/method","params":{}}` + "\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestJSONRPCParseError(t *testing.T) {
	s := setupTestServer(t)

	input := "not valid json\n"
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
}

func TestJSONRPCMultipleRequests(t *testing.T) {
	s := setupTestServer(t)

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
`
	var out bytes.Buffer
	if err := s.RunIO(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}
}
