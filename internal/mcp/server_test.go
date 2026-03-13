package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Test infrastructure ---

// testEnv provides a connected server + client for protocol-level testing.
type testEnv struct {
	server  *Server
	session *mcp.ClientSession
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	return connect(t, s)
}

func connect(t *testing.T, s *Server) *testEnv {
	t.Helper()
	sTransport, cTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	_, err := s.MCPServer().Connect(ctx, sTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, cTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		_ = cs.Close()
		s.Shutdown()
	})

	return &testEnv{server: s, session: cs}
}

func (e *testEnv) call(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := e.session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %q: %v", name, err)
	}
	return result
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	// Content items implement MarshalJSON; extract text by round-tripping.
	data, err := json.Marshal(r.Content[0])
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var wire struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	return wire.Text
}

func resultMap(t *testing.T, r *mcp.CallToolResult) map[string]any {
	t.Helper()
	text := resultText(t, r)
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("parse result JSON: %v\nraw: %s", err, text)
	}
	return m
}

// setupWithTree creates a connected env and replaces the tree with a known structure.
// Tree: root > header + main(form(name_input, submit_btn), results)
func setupWithTree(t *testing.T) *testEnv {
	t.Helper()
	e := setup(t)

	r := e.call(t, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{"id": "header", "type": "text", "props": map[string]any{"text": "Dashboard"}},
				map[string]any{
					"id":   "main",
					"type": "container",
					"children": []any{
						map[string]any{
							"id":   "form",
							"type": "container",
							"children": []any{
								map[string]any{"id": "name_input", "type": "input", "props": map[string]any{"placeholder": "Name"}},
								map[string]any{"id": "submit_btn", "type": "button", "props": map[string]any{"label": "Submit"}},
							},
						},
						map[string]any{"id": "results", "type": "table"},
					},
				},
			},
		},
	})
	if r.IsError {
		t.Fatalf("setup tree failed: %s", resultText(t, r))
	}
	return e
}

// =============================================================================
// M2-1: MCP server skeleton
// =============================================================================

func TestNewServerCreatesServer(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if s.MCPServer() == nil {
		t.Error("MCP server should not be nil")
	}
	if s.Tree() == nil {
		t.Error("tree should not be nil")
	}
	if s.Tree().Root == nil {
		t.Error("tree root should not be nil")
	}
	if s.Tree().Root.ID != "root" {
		t.Errorf("root ID = %q, want root", s.Tree().Root.ID)
	}
	if s.Events() == nil {
		t.Error("event queue should not be nil")
	}
	if s.Snapshots() == nil {
		t.Error("snapshot store should not be nil")
	}
}

func TestServerListsAllTools(t *testing.T) {
	e := setup(t)

	result, err := e.session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]bool{
		"patch": false, "replace": false, "await_event": false,
		"snapshot": false, "restore": false, "query": false,
	}

	for _, tool := range result.Tools {
		if _, ok := expected[tool.Name]; ok {
			expected[tool.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}

	if len(result.Tools) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(result.Tools))
	}
}

func TestServerShutdown(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if s.IsShutdown() {
		t.Error("server should not be shut down initially")
	}
	s.Shutdown()
	if !s.IsShutdown() {
		t.Error("server should be shut down after Shutdown()")
	}
	// Double shutdown should not panic.
	s.Shutdown()
}

func TestMCPSDKImport(t *testing.T) {
	// Verify official SDK dependency is available and core types are usable.
	tool := &mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
	if tool.Name != "test_tool" {
		t.Fatalf("expected tool name 'test_tool', got %q", tool.Name)
	}
}

// =============================================================================
// M2-2: Tool: patch
// =============================================================================

func TestPatchToolValidOps(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Updated Dashboard"}},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m)
	}

	n := e.server.Tree().Find("header")
	if v, _ := n.GetProp("text"); v != "Updated Dashboard" {
		t.Errorf("text = %v, want Updated Dashboard", v)
	}
}

func TestPatchToolInsert(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "insert", "parent_id": "main", "id": "footer", "type": "text",
				"props": map[string]any{"text": "Footer"}},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	if e.server.Tree().Find("footer") == nil {
		t.Error("footer node should exist after insert")
	}
}

func TestPatchToolRemove(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "remove", "id": "results"},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	if e.server.Tree().Find("results") != nil {
		t.Error("results node should be removed")
	}
}

func TestPatchToolMove(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "move", "id": "results", "parent_id": "root"},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	n := e.server.Tree().Find("results")
	if n.Parent().ID != "root" {
		t.Errorf("results parent = %q, want root", n.Parent().ID)
	}
}

func TestPatchToolMultiOp(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "insert", "parent_id": "root", "id": "sidebar", "type": "container"},
			map[string]any{"op": "update", "id": "sidebar", "props": map[string]any{"border": true}},
			map[string]any{"op": "move", "id": "results", "parent_id": "sidebar"},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	sidebar := e.server.Tree().Find("sidebar")
	if sidebar == nil {
		t.Fatal("sidebar should exist")
	}
	results := e.server.Tree().Find("results")
	if results.Parent().ID != "sidebar" {
		t.Errorf("results parent = %q, want sidebar", results.Parent().ID)
	}
}

func TestPatchToolMissingOps(t *testing.T) {
	e := setup(t)
	result := e.call(t, "patch", map[string]any{})
	if !result.IsError {
		t.Error("expected error for missing ops")
	}
}

func TestPatchToolInvalidOps(t *testing.T) {
	e := setup(t)
	result := e.call(t, "patch", map[string]any{
		"ops": "not an array",
	})
	if !result.IsError {
		t.Error("expected error for invalid ops")
	}
}

func TestPatchToolEngineError(t *testing.T) {
	e := setup(t)
	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "nonexistent", "props": map[string]any{"text": "x"}},
		},
	})
	if !result.IsError {
		t.Error("expected error for nonexistent node")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "nonexistent") {
		t.Errorf("error should mention nonexistent: %s", text)
	}
}

func TestPatchToolWithNodeSpec(t *testing.T) {
	e := setup(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op":        "insert",
				"parent_id": "root",
				"node": map[string]any{
					"id":   "panel",
					"type": "container",
					"children": []any{
						map[string]any{"id": "txt", "type": "text", "props": map[string]any{"text": "hello"}},
					},
				},
			},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	if e.server.Tree().Find("panel") == nil {
		t.Error("panel should exist")
	}
	if e.server.Tree().Find("txt") == nil {
		t.Error("txt should exist")
	}
}

func TestPatchToolAtomicRollback(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "insert", "parent_id": "root", "id": "temp", "type": "text"},
			map[string]any{"op": "update", "id": "does_not_exist", "props": map[string]any{"x": 1}},
		},
	})
	if !result.IsError {
		t.Error("expected error")
	}
	if e.server.Tree().Find("temp") != nil {
		t.Error("temp should not exist after rollback")
	}
}

func TestPatchToolEmptyOps(t *testing.T) {
	e := setup(t)
	result := e.call(t, "patch", map[string]any{
		"ops": []any{},
	})
	if result.IsError {
		t.Errorf("empty ops should succeed: %s", resultText(t, result))
	}
}

// =============================================================================
// M2-3: Tool: replace
// =============================================================================

func TestReplaceToolWholeTree(t *testing.T) {
	e := setup(t)

	result := e.call(t, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "app",
			"type": "container",
			"children": []any{
				map[string]any{"id": "header", "type": "text", "props": map[string]any{"text": "App"}},
				map[string]any{"id": "body", "type": "container"},
			},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m)
	}
	if e.server.Tree().Root.ID != "app" {
		t.Errorf("root ID = %q, want app", e.server.Tree().Root.ID)
	}
	if e.server.Tree().Find("header") == nil {
		t.Error("header should exist")
	}
}

func TestReplaceToolSubtree(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "replace", map[string]any{
		"target_id": "form",
		"children": []any{
			map[string]any{"id": "email_input", "type": "input", "props": map[string]any{"placeholder": "Email"}},
			map[string]any{"id": "password_input", "type": "input", "props": map[string]any{"placeholder": "Password"}},
			map[string]any{"id": "login_btn", "type": "button", "props": map[string]any{"label": "Login"}},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	if e.server.Tree().Find("name_input") != nil {
		t.Error("name_input should be removed")
	}
	if e.server.Tree().Find("submit_btn") != nil {
		t.Error("submit_btn should be removed")
	}
	if e.server.Tree().Find("email_input") == nil {
		t.Error("email_input should exist")
	}
	if e.server.Tree().Find("login_btn") == nil {
		t.Error("login_btn should exist")
	}
}

func TestReplaceToolIDNotFound(t *testing.T) {
	e := setup(t)
	result := e.call(t, "replace", map[string]any{
		"target_id": "nonexistent",
		"children": []any{
			map[string]any{"id": "x", "type": "text"},
		},
	})
	if !result.IsError {
		t.Error("expected error for nonexistent target")
	}
}

func TestReplaceToolIDCollision(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "replace", map[string]any{
		"target_id": "form",
		"children": []any{
			map[string]any{"id": "header", "type": "text"},
		},
	})
	if !result.IsError {
		t.Error("expected error for ID collision")
	}
}

func TestReplaceToolMissingBoth(t *testing.T) {
	e := setup(t)
	result := e.call(t, "replace", map[string]any{})
	if !result.IsError {
		t.Error("expected error when neither target_id nor tree provided")
	}
}

func TestReplaceToolSubtreeMissingChildren(t *testing.T) {
	e := setupWithTree(t)
	result := e.call(t, "replace", map[string]any{
		"target_id": "form",
	})
	if !result.IsError {
		t.Error("expected error when target_id provided without children")
	}
}

// =============================================================================
// M2-4: Tool: await_event
// =============================================================================

func TestAwaitEventImmediateReturn(t *testing.T) {
	e := setup(t)

	e.server.Events().Enqueue(&dom.Event{
		Type:   "click",
		Source: "btn1",
		Data:   map[string]any{"x": 1},
	})

	result := e.call(t, "await_event", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["event"] != "click" {
		t.Errorf("event = %v, want click", m["event"])
	}
	if m["source"] != "btn1" {
		t.Errorf("source = %v, want btn1", m["source"])
	}
}

func TestAwaitEventBlocksThenReturns(t *testing.T) {
	e := setup(t)

	done := make(chan map[string]any, 1)
	go func() {
		result := e.call(t, "await_event", map[string]any{})
		done <- resultMap(t, result)
	}()

	time.Sleep(30 * time.Millisecond)

	e.server.Events().Enqueue(&dom.Event{
		Type:   "submit",
		Source: "form1",
	})

	select {
	case m := <-done:
		if m["event"] != "submit" {
			t.Errorf("event = %v, want submit", m["event"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("await_event didn't return after event was enqueued")
	}
}

func TestAwaitEventTimeout(t *testing.T) {
	e := setup(t)

	result := e.call(t, "await_event", map[string]any{
		"timeout_ms": 50,
	})
	if result.IsError {
		t.Fatalf("timeout should not be an error result: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["timeout"] != true {
		t.Errorf("expected timeout:true, got %v", m)
	}
}

func TestAwaitEventFilter(t *testing.T) {
	e := setup(t)

	e.server.Events().Enqueue(&dom.Event{Type: "click", Source: "btn1"})
	e.server.Events().Enqueue(&dom.Event{Type: "click", Source: "btn2"})

	result := e.call(t, "await_event", map[string]any{
		"filter": []any{"btn2"},
	})
	m := resultMap(t, result)
	if m["source"] != "btn2" {
		t.Errorf("source = %v, want btn2", m["source"])
	}

	// btn1 should still be in the queue.
	result = e.call(t, "await_event", map[string]any{})
	m = resultMap(t, result)
	if m["source"] != "btn1" {
		t.Errorf("remaining source = %v, want btn1", m["source"])
	}
}

func TestAwaitEventDebounce(t *testing.T) {
	e := setup(t)

	for i := 0; i < 5; i++ {
		e.server.Events().Enqueue(&dom.Event{
			Type:   "change",
			Source: "input1",
			Data:   map[string]any{"i": float64(i)},
		})
	}

	result := e.call(t, "await_event", map[string]any{
		"debounce_ms": 50,
	})
	m := resultMap(t, result)
	if m["source"] != "input1" {
		t.Errorf("source = %v, want input1", m["source"])
	}
	if cc, ok := m["coalesced_count"].(float64); !ok || cc == 0 {
		t.Errorf("expected coalesced_count > 0, got %v", m["coalesced_count"])
	}
}

func TestAwaitEventEnrichesDOMSummary(t *testing.T) {
	e := setupWithTree(t)

	e.server.Events().Enqueue(&dom.Event{
		Type:   "click",
		Source: "submit_btn",
	})

	result := e.call(t, "await_event", map[string]any{})
	m := resultMap(t, result)
	summary, ok := m["dom_summary"].(string)
	if !ok || summary == "" {
		t.Error("expected dom_summary to be populated")
	}
	if !strings.Contains(summary, "root") {
		t.Errorf("dom_summary should contain 'root': %q", summary)
	}
}

func TestAwaitEventPreservesExistingDOMSummary(t *testing.T) {
	e := setup(t)

	e.server.Events().Enqueue(&dom.Event{
		Type:       "click",
		Source:     "btn1",
		DOMSummary: "custom_summary",
	})

	result := e.call(t, "await_event", map[string]any{})
	m := resultMap(t, result)
	if m["dom_summary"] != "custom_summary" {
		t.Errorf("dom_summary = %v, want custom_summary", m["dom_summary"])
	}
}

func TestAwaitEventWithContext(t *testing.T) {
	e := setup(t)

	e.server.Events().Enqueue(&dom.Event{
		Type:   "click",
		Source: "submit_btn",
		Context: map[string]map[string]any{
			"name_input": {"value": "Acme Corp"},
		},
	})

	result := e.call(t, "await_event", map[string]any{})
	m := resultMap(t, result)
	ctx, ok := m["context"].(map[string]any)
	if !ok {
		t.Fatal("expected context in result")
	}
	nameCtx, ok := ctx["name_input"].(map[string]any)
	if !ok {
		t.Fatal("expected name_input in context")
	}
	if nameCtx["value"] != "Acme Corp" {
		t.Errorf("value = %v, want Acme Corp", nameCtx["value"])
	}
}

func TestAwaitEventMultipleRapidEvents(t *testing.T) {
	e := setup(t)

	e.server.Events().Enqueue(&dom.Event{Type: "click", Source: "btn1"})
	e.server.Events().Enqueue(&dom.Event{Type: "change", Source: "input1"})

	r1 := e.call(t, "await_event", map[string]any{})
	m1 := resultMap(t, r1)
	if m1["source"] != "btn1" {
		t.Errorf("first event source = %v, want btn1", m1["source"])
	}

	r2 := e.call(t, "await_event", map[string]any{})
	m2 := resultMap(t, r2)
	if m2["source"] != "input1" {
		t.Errorf("second event source = %v, want input1", m2["source"])
	}
}

// =============================================================================
// M2-5: Tool: snapshot & restore
// =============================================================================

func TestSnapshotAndRestoreRoundTrip(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "snapshot", map[string]any{"name": "v1"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m)
	}
	if m["name"] != "v1" {
		t.Errorf("name = %v, want v1", m["name"])
	}

	// Modify the tree.
	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "remove", "id": "form"},
		},
	})
	if e.server.Tree().Find("form") != nil {
		t.Fatal("form should be removed before restore")
	}

	// Restore.
	result = e.call(t, "restore", map[string]any{"name": "v1"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m = resultMap(t, result)
	if m["ok"] != true {
		t.Error("expected ok:true")
	}
	if m["restored"] != "v1" {
		t.Errorf("restored = %v, want v1", m["restored"])
	}

	if e.server.Tree().Find("form") == nil {
		t.Error("form should be restored")
	}
	if e.server.Tree().Find("name_input") == nil {
		t.Error("name_input should be restored")
	}
}

func TestRestoreNonexistentSnapshotError(t *testing.T) {
	e := setup(t)
	result := e.call(t, "restore", map[string]any{"name": "nope"})
	if !result.IsError {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestSnapshotMissingName(t *testing.T) {
	e := setup(t)
	result := e.call(t, "snapshot", map[string]any{})
	if !result.IsError {
		t.Error("expected error for missing name")
	}
}

func TestRestoreMissingName(t *testing.T) {
	e := setup(t)
	result := e.call(t, "restore", map[string]any{})
	if !result.IsError {
		t.Error("expected error for missing name")
	}
}

func TestSnapshotOverwriteViaMCP(t *testing.T) {
	e := setupWithTree(t)

	e.call(t, "snapshot", map[string]any{"name": "v1"})

	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Modified"}},
		},
	})

	e.call(t, "snapshot", map[string]any{"name": "v1"})

	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Modified Again"}},
		},
	})

	e.call(t, "restore", map[string]any{"name": "v1"})
	n := e.server.Tree().Find("header")
	if v, _ := n.GetProp("text"); v != "Modified" {
		t.Errorf("text = %v, want Modified", v)
	}
}

// =============================================================================
// M2-6: Tool: query
// =============================================================================

func TestQueryToolExistingNodes(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "query", map[string]any{
		"ids": []any{"header", "name_input"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	results, ok := m["results"].(map[string]any)
	if !ok {
		t.Fatal("expected results map")
	}
	if _, ok := results["header"]; !ok {
		t.Error("header not in results")
	}
	if _, ok := results["name_input"]; !ok {
		t.Error("name_input not in results")
	}
}

func TestQueryToolMissingNodes(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "query", map[string]any{
		"ids": []any{"header", "nonexistent"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)

	results := m["results"].(map[string]any)
	if _, ok := results["header"]; !ok {
		t.Error("header should be in results")
	}

	errors, ok := m["errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Error("expected errors for missing nodes")
	}
}

func TestQueryToolAllMissing(t *testing.T) {
	e := setup(t)

	result := e.call(t, "query", map[string]any{
		"ids": []any{"nope1", "nope2"},
	})
	m := resultMap(t, result)
	errors, ok := m["errors"].([]any)
	if !ok || len(errors) != 2 {
		t.Errorf("expected 2 errors, got %v", m["errors"])
	}
}

func TestQueryToolMissingIDs(t *testing.T) {
	e := setup(t)
	result := e.call(t, "query", map[string]any{})
	if !result.IsError {
		t.Error("expected error for missing ids parameter")
	}
}

func TestQueryToolReturnsProps(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "query", map[string]any{
		"ids": []any{"name_input"},
	})
	m := resultMap(t, result)
	results := m["results"].(map[string]any)
	node := results["name_input"].(map[string]any)
	props := node["props"].(map[string]any)
	if props["placeholder"] != "Name" {
		t.Errorf("placeholder = %v, want Name", props["placeholder"])
	}
}

func TestQueryToolReturnsStructure(t *testing.T) {
	e := setupWithTree(t)

	result := e.call(t, "query", map[string]any{
		"ids": []any{"form"},
	})
	m := resultMap(t, result)
	results := m["results"].(map[string]any)
	node := results["form"].(map[string]any)

	if node["type"] != "container" {
		t.Errorf("type = %v, want container", node["type"])
	}
	childIDs, ok := node["child_ids"].([]any)
	if !ok || len(childIDs) != 2 {
		t.Errorf("expected 2 child_ids, got %v", node["child_ids"])
	}
	if node["parent_id"] != "main" {
		t.Errorf("parent_id = %v, want main", node["parent_id"])
	}
}

func TestQueryToolReturnsScriptsAndComputed(t *testing.T) {
	e := setupWithTree(t)

	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op": "update", "id": "header",
				"scripts":  map[string]any{"on_mount": "init()"},
				"computed": map[string]any{"display": "return 'x'"},
			},
		},
	})

	result := e.call(t, "query", map[string]any{
		"ids": []any{"header"},
	})
	m := resultMap(t, result)
	results := m["results"].(map[string]any)
	node := results["header"].(map[string]any)

	scripts, ok := node["scripts"].(map[string]any)
	if !ok || scripts["on_mount"] != "init()" {
		t.Errorf("scripts = %v", node["scripts"])
	}
	computed, ok := node["computed"].(map[string]any)
	if !ok || computed["display"] != "return 'x'" {
		t.Errorf("computed = %v", node["computed"])
	}
}

// =============================================================================
// M2-7: MCP error handling & edge cases
// =============================================================================

func TestToolCallsDuringShutdown(t *testing.T) {
	// For shutdown tests, we call handlers directly since the client session
	// won't work after server shutdown.
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.Shutdown()

	tools := []struct {
		name string
		args map[string]any
	}{
		{"patch", map[string]any{"ops": []any{}}},
		{"replace", map[string]any{"tree": map[string]any{"id": "r", "type": "container"}}},
		{"snapshot", map[string]any{"name": "v1"}},
		{"restore", map[string]any{"name": "v1"}},
		{"query", map[string]any{"ids": []any{"root"}}},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			result := callHandlerDirect(t, s, tt.name, tt.args)
			if !result.IsError {
				t.Errorf("expected error during shutdown for %s", tt.name)
			}
			text := resultText(t, result)
			if !strings.Contains(text, "shutting down") {
				t.Errorf("error should mention shutting down: %s", text)
			}
		})
	}
}

func TestAwaitEventDuringShutdown(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.Shutdown()

	result := callHandlerDirect(t, s, "await_event", map[string]any{})
	if !result.IsError {
		t.Error("expected error during shutdown for await_event")
	}
}

func TestConcurrentPatchAndQuery(t *testing.T) {
	e := setupWithTree(t)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e.call(t, "patch", map[string]any{
				"ops": []any{
					map[string]any{
						"op": "update", "id": "header",
						"props": map[string]any{"text": fmt.Sprintf("v%d", i)},
					},
				},
			})
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.call(t, "query", map[string]any{
				"ids": []any{"header"},
			})
		}()
	}

	wg.Wait()
}

func TestConcurrentPatchAndAwaitEvent(t *testing.T) {
	e := setupWithTree(t)

	done := make(chan struct{})
	go func() {
		result := e.call(t, "await_event", map[string]any{})
		m := resultMap(t, result)
		if m["event"] != "click" {
			t.Errorf("event = %v, want click", m["event"])
		}
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)

	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Patched"}},
		},
	})

	e.server.Events().Enqueue(&dom.Event{Type: "click", Source: "btn1"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent patch + await_event timed out")
	}
}

func TestShutdownMidAwaitEvent(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan *mcp.CallToolResult, 1)
	go func() {
		result := callHandlerDirect(t, s, "await_event", map[string]any{})
		done <- result
	}()

	time.Sleep(30 * time.Millisecond)
	s.Shutdown()

	select {
	case result := <-done:
		if !result.IsError {
			t.Error("expected error from await_event after shutdown")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("await_event didn't return after shutdown")
	}
}

func TestReplaceTreeWithScriptsAndComputed(t *testing.T) {
	e := setup(t)

	result := e.call(t, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{
					"id":       "counter",
					"type":     "text",
					"props":    map[string]any{"text": "0"},
					"scripts":  map[string]any{"on_click": "state.count++"},
					"computed": map[string]any{"display": "return state.count"},
				},
			},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	n := e.server.Tree().Find("counter")
	if n == nil {
		t.Fatal("counter node not found")
	}
	if n.Scripts["on_click"] != "state.count++" {
		t.Errorf("script = %q", n.Scripts["on_click"])
	}
	if n.Computed["display"] != "return state.count" {
		t.Errorf("computed = %q", n.Computed["display"])
	}
}

func TestFullWorkflow(t *testing.T) {
	e := setup(t)

	// 1. Replace: build initial screen.
	e.call(t, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{"id": "title", "type": "text", "props": map[string]any{"text": "Hello"}},
				map[string]any{"id": "btn", "type": "button", "props": map[string]any{"label": "Click"}},
			},
		},
	})

	// 2. Patch: update title.
	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "title", "props": map[string]any{"text": "World"}},
		},
	})

	// 3. Snapshot.
	e.call(t, "snapshot", map[string]any{"name": "after_update"})

	// 4. Query.
	qResult := e.call(t, "query", map[string]any{"ids": []any{"title"}})
	qm := resultMap(t, qResult)
	results := qm["results"].(map[string]any)
	titleNode := results["title"].(map[string]any)
	props := titleNode["props"].(map[string]any)
	if props["text"] != "World" {
		t.Errorf("title text = %v, want World", props["text"])
	}

	// 5. Enqueue event + await.
	e.server.Events().Enqueue(&dom.Event{Type: "click", Source: "btn"})
	eResult := e.call(t, "await_event", map[string]any{})
	em := resultMap(t, eResult)
	if em["source"] != "btn" {
		t.Errorf("event source = %v, want btn", em["source"])
	}

	// 6. Modify + restore.
	e.call(t, "patch", map[string]any{
		"ops": []any{
			map[string]any{"op": "update", "id": "title", "props": map[string]any{"text": "Destroyed"}},
		},
	})
	e.call(t, "restore", map[string]any{"name": "after_update"})
	if v, _ := e.server.Tree().Find("title").GetProp("text"); v != "World" {
		t.Errorf("after restore text = %v, want World", v)
	}
}

// --- Direct handler calls (for shutdown/edge case tests) ---

func callHandlerDirect(t *testing.T, s *Server, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	req := &mcp.CallToolRequest{}
	req.Params = &mcp.CallToolParamsRaw{
		Name:      toolName,
		Arguments: argsJSON,
	}

	ctx := context.Background()
	var result *mcp.CallToolResult

	switch toolName {
	case "patch":
		result, err = s.handlePatch(ctx, req)
	case "replace":
		result, err = s.handleReplace(ctx, req)
	case "await_event":
		result, err = s.handleAwaitEvent(ctx, req)
	case "snapshot":
		result, err = s.handleSnapshot(ctx, req)
	case "restore":
		result, err = s.handleRestore(ctx, req)
	case "query":
		result, err = s.handleQuery(ctx, req)
	default:
		t.Fatalf("unknown tool: %s", toolName)
	}

	if err != nil {
		t.Fatalf("tool %q returned error: %v", toolName, err)
	}
	return result
}
