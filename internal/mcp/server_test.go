package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		"layout": false, "set_items": false, "append_items": false, "remove_items": false,
		"describe_widgets":   false,
		"describe_scripting": false,
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

func TestSetItemsToolMetadataPrefersPushItems(t *testing.T) {
	e := setup(t)

	result, err := e.session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	for _, tool := range result.Tools {
		if tool.Name != "set_items" {
			continue
		}

		if !strings.Contains(tool.Description, "push-items") {
			t.Fatalf("set_items description = %q, want push-items guidance", tool.Description)
		}

		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("set_items input schema type = %T, want map[string]any", tool.InputSchema)
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("set_items properties type = %T, want map[string]any", schema["properties"])
		}
		if _, ok := props["file"]; ok {
			t.Fatal("set_items input schema should not advertise file")
		}
		if _, ok := props["format"]; ok {
			t.Fatal("set_items input schema should not advertise format")
		}
		return
	}

	t.Fatal("set_items tool not found")
}

func TestServerInstructionsMentionPushItems(t *testing.T) {
	if !strings.Contains(serverInstructions, "push-items") {
		t.Fatalf("server instructions missing push-items guidance:\n%s", serverInstructions)
	}
	if strings.Contains(serverInstructions, "set_items accepts either inline items or file-based loading") {
		t.Fatalf("server instructions still advertise server-side file loading:\n%s", serverInstructions)
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
	case "layout":
		result, err = s.handleLayout(ctx, req)
	case "set_items":
		result, err = s.handleSetItems(ctx, req)
	case "append_items":
		result, err = s.handleAppendItems(ctx, req)
	case "remove_items":
		result, err = s.handleRemoveItems(ctx, req)
	case "describe_widgets":
		result, err = s.handleDescribeWidgets(ctx, req)
	case "describe_scripting":
		result, err = s.handleDescribeScripting(ctx, req)
	default:
		t.Fatalf("unknown tool: %s", toolName)
	}

	if err != nil {
		t.Fatalf("tool %q returned error: %v", toolName, err)
	}
	return result
}

// --- Mutation callback tests ---

func TestOnMutationCallback(t *testing.T) {
	t.Run("patch fires callback on success", func(t *testing.T) {
		e := setupWithTree(t)
		var mu sync.Mutex
		calls := 0
		var kind MutationKind
		e.server.SetOnMutation(func(k MutationKind) {
			mu.Lock()
			kind = k
			calls++
			mu.Unlock()
		})

		e.call(t, "patch", map[string]any{
			"ops": []any{
				map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Updated"}},
			},
		})

		mu.Lock()
		defer mu.Unlock()
		if calls != 1 {
			t.Fatalf("expected 1 mutation callback, got %d", calls)
		}
		if kind != MutationKindUpdate {
			t.Fatalf("mutation kind = %q, want %q", kind, MutationKindUpdate)
		}
	})

	t.Run("patch does not fire callback on error", func(t *testing.T) {
		e := setup(t)
		calls := 0
		e.server.SetOnMutation(func(MutationKind) { calls++ })

		r := e.call(t, "patch", map[string]any{
			"ops": []any{
				map[string]any{"op": "update", "id": "nonexistent", "props": map[string]any{"text": "x"}},
			},
		})

		text := resultText(t, r)
		if !strings.Contains(text, "not found") {
			t.Fatalf("expected error about not found, got: %s", text)
		}
		if calls != 0 {
			t.Fatalf("expected 0 mutation callbacks on error, got %d", calls)
		}
	})

	t.Run("replace whole tree fires callback", func(t *testing.T) {
		e := setup(t)
		calls := 0
		var kind MutationKind
		e.server.SetOnMutation(func(k MutationKind) {
			kind = k
			calls++
		})

		e.call(t, "replace", map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []any{
					map[string]any{"id": "child", "type": "text", "props": map[string]any{"content": "hi"}},
				},
			},
		})

		if calls != 1 {
			t.Fatalf("expected 1 mutation callback, got %d", calls)
		}
		if kind != MutationKindResetFocus {
			t.Fatalf("mutation kind = %q, want %q", kind, MutationKindResetFocus)
		}
	})

	t.Run("replace subtree fires callback", func(t *testing.T) {
		e := setupWithTree(t)
		calls := 0
		var kind MutationKind
		e.server.SetOnMutation(func(k MutationKind) {
			kind = k
			calls++
		})

		e.call(t, "replace", map[string]any{
			"target_id": "main",
			"children": []any{
				map[string]any{"id": "new_child", "type": "text", "props": map[string]any{"content": "replaced"}},
			},
		})

		if calls != 1 {
			t.Fatalf("expected 1 mutation callback, got %d", calls)
		}
		if kind != MutationKindUpdate {
			t.Fatalf("mutation kind = %q, want %q", kind, MutationKindUpdate)
		}
	})

	t.Run("restore fires callback", func(t *testing.T) {
		e := setupWithTree(t)

		// Take a snapshot first.
		e.call(t, "snapshot", map[string]any{"name": "before"})

		// Mutate.
		e.call(t, "patch", map[string]any{
			"ops": []any{
				map[string]any{"op": "update", "id": "header", "props": map[string]any{"text": "Changed"}},
			},
		})

		// Now register callback and restore.
		calls := 0
		var kind MutationKind
		e.server.SetOnMutation(func(k MutationKind) {
			kind = k
			calls++
		})

		e.call(t, "restore", map[string]any{"name": "before"})

		if calls != 1 {
			t.Fatalf("expected 1 mutation callback, got %d", calls)
		}
		if kind != MutationKindResetFocus {
			t.Fatalf("mutation kind = %q, want %q", kind, MutationKindResetFocus)
		}
	})

	t.Run("snapshot does not fire callback", func(t *testing.T) {
		e := setupWithTree(t)
		calls := 0
		e.server.SetOnMutation(func(MutationKind) { calls++ })

		e.call(t, "snapshot", map[string]any{"name": "test"})

		if calls != 0 {
			t.Fatalf("expected 0 mutation callbacks for snapshot, got %d", calls)
		}
	})

	t.Run("query does not fire callback", func(t *testing.T) {
		e := setupWithTree(t)
		calls := 0
		e.server.SetOnMutation(func(MutationKind) { calls++ })

		e.call(t, "query", map[string]any{"ids": []any{"header"}})

		if calls != 0 {
			t.Fatalf("expected 0 mutation callbacks for query, got %d", calls)
		}
	})
}

// =============================================================================
// Template-driven data tools: layout, set_items, append_items, remove_items
// =============================================================================

// setupWithTemplate creates a connected env and uses layout to set a tree
// with an item_template on the "log-list" container.
func setupWithTemplate(t *testing.T) *testEnv {
	t.Helper()
	e := setup(t)

	r := e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{"id": "header", "type": "text", "props": map[string]any{"content": "Log Viewer"}},
				map[string]any{
					"id":   "log-list",
					"type": "container",
					"props": map[string]any{
						"item_template": map[string]any{
							"type": "container",
							"props": map[string]any{
								"direction": "row",
							},
							"children": []any{
								map[string]any{"type": "text", "props": map[string]any{"content": "{{level}}", "width": 8}},
								map[string]any{"type": "text", "props": map[string]any{"content": "{{msg}}"}},
							},
						},
					},
				},
			},
		},
	})
	if r.IsError {
		t.Fatalf("setup template failed: %s", resultText(t, r))
	}
	return e
}

func TestLayoutTool_WholeTree(t *testing.T) {
	e := setup(t)

	result := e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id":   "app",
			"type": "container",
			"children": []any{
				map[string]any{"id": "header", "type": "text", "props": map[string]any{"content": "App"}},
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

func TestLayoutTool_WithItemTemplate(t *testing.T) {
	e := setupWithTemplate(t)

	// Verify the item_template is stored as a prop.
	list := e.server.Tree().Find("log-list")
	if list == nil {
		t.Fatal("log-list not found")
	}
	tmpl, ok := list.Props["item_template"]
	if !ok {
		t.Fatal("item_template prop not found")
	}
	tmplMap, ok := tmpl.(map[string]any)
	if !ok {
		t.Fatalf("item_template is %T, want map[string]any", tmpl)
	}
	if tmplMap["type"] != "container" {
		t.Errorf("template type = %v, want container", tmplMap["type"])
	}
}

func TestSetItemsTool_Basic(t *testing.T) {
	e := setupWithTemplate(t)

	result := e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items": []any{
			map[string]any{"level": "INFO", "msg": "server started"},
			map[string]any{"level": "ERROR", "msg": "disk full"},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	m := resultMap(t, result)
	if m["ok"] != true {
		t.Errorf("expected ok:true, got %v", m)
	}

	// Query to verify expanded children.
	qResult := e.call(t, "query", map[string]any{
		"ids": []any{"log-list"},
	})
	qm := resultMap(t, qResult)
	results := qm["results"].(map[string]any)
	listNode := results["log-list"].(map[string]any)
	childIDs := listNode["child_ids"].([]any)
	if len(childIDs) != 2 {
		t.Fatalf("expected 2 children, got %d: %v", len(childIDs), childIDs)
	}

	// Verify first expanded node has correct content.
	q2 := e.call(t, "query", map[string]any{"ids": []any{"log-list-0-0"}})
	q2m := resultMap(t, q2)
	r2 := q2m["results"].(map[string]any)
	textNode := r2["log-list-0-0"].(map[string]any)
	props := textNode["props"].(map[string]any)
	if props["content"] != "INFO" {
		t.Errorf("content = %v, want INFO", props["content"])
	}
}

func TestSetItemsTool_ReplacesExisting(t *testing.T) {
	e := setupWithTemplate(t)

	// First set.
	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "INFO", "msg": "first"}},
	})

	// Second set replaces.
	result := e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items": []any{
			map[string]any{"level": "ERROR", "msg": "second"},
			map[string]any{"level": "WARN", "msg": "third"},
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	list := e.server.Tree().Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}
}

func TestSetItemsTool_FileParameterRejected(t *testing.T) {
	e := setupWithTemplate(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "items.json")
	data := `[
		{"level":"INFO","msg":"server started"},
		{"level":"ERROR","msg":"disk full"}
	]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	result := e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"file":   path,
	})
	if !result.IsError {
		t.Fatal("expected file-based set_items to be rejected")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "push-items") {
		t.Fatalf("unexpected error: %s", text)
	}
}

func TestSetItemsTool_FileRejectedEvenWithItems(t *testing.T) {
	e := setupWithTemplate(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "items.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}

	result := e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "INFO", "msg": "x"}},
		"file":   path,
	})
	if !result.IsError {
		t.Fatal("expected file-based set_items to be rejected")
	}
	if got := resultText(t, result); !strings.Contains(got, "push-items") {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSetItemsTool_MissingTarget(t *testing.T) {
	e := setupWithTemplate(t)
	result := e.call(t, "set_items", map[string]any{
		"target": "nonexistent",
		"items":  []any{map[string]any{"level": "INFO", "msg": "x"}},
	})
	if !result.IsError {
		t.Error("expected error for missing target")
	}
}

func TestSetItemsTool_NoTemplate(t *testing.T) {
	e := setupWithTemplate(t)
	result := e.call(t, "set_items", map[string]any{
		"target": "header",
		"items":  []any{map[string]any{"level": "INFO", "msg": "x"}},
	})
	if !result.IsError {
		t.Error("expected error for node without item_template")
	}
}

func TestSetItemsTool_MissingItems(t *testing.T) {
	e := setupWithTemplate(t)

	result := e.call(t, "set_items", map[string]any{
		"target": "log-list",
	})
	if !result.IsError {
		t.Fatal("expected missing items to be rejected")
	}
	if got := resultText(t, result); !strings.Contains(got, "missing required parameter: items") {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestAppendItemsTool_Basic(t *testing.T) {
	e := setupWithTemplate(t)

	// Set initial items.
	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "INFO", "msg": "initial"}},
	})

	// Append more.
	result := e.call(t, "append_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "ERROR", "msg": "appended"}},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	list := e.server.Tree().Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}

	// Verify second item is the appended one.
	q := e.call(t, "query", map[string]any{"ids": []any{"log-list-1-1"}})
	qm := resultMap(t, q)
	results := qm["results"].(map[string]any)
	node := results["log-list-1-1"].(map[string]any)
	props := node["props"].(map[string]any)
	if props["content"] != "appended" {
		t.Errorf("content = %v, want appended", props["content"])
	}
}

func TestRemoveItemsTool_Basic(t *testing.T) {
	e := setupWithTemplate(t)

	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items": []any{
			map[string]any{"key": "a", "level": "INFO", "msg": "keep"},
			map[string]any{"key": "b", "level": "ERROR", "msg": "remove"},
			map[string]any{"key": "c", "level": "WARN", "msg": "keep"},
		},
	})

	result := e.call(t, "remove_items", map[string]any{
		"target": "log-list",
		"keys":   []any{"b"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	list := e.server.Tree().Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}
	if e.server.Tree().Find("log-list-b") != nil {
		t.Error("log-list-b should be removed")
	}
}

func TestSetItems_FiresMutationCallback(t *testing.T) {
	e := setupWithTemplate(t)
	calls := 0
	var kind MutationKind
	e.server.SetOnMutation(func(k MutationKind) {
		kind = k
		calls++
	})

	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "INFO", "msg": "x"}},
	})

	if calls != 1 {
		t.Fatalf("expected 1 mutation callback, got %d", calls)
	}
	if kind != MutationKindUpdate {
		t.Fatalf("mutation kind = %q, want %q", kind, MutationKindUpdate)
	}
}

func TestAppendItems_FiresMutationCallback(t *testing.T) {
	e := setupWithTemplate(t)
	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "INFO", "msg": "x"}},
	})

	calls := 0
	var kind MutationKind
	e.server.SetOnMutation(func(k MutationKind) {
		kind = k
		calls++
	})

	e.call(t, "append_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"level": "WARN", "msg": "y"}},
	})

	if calls != 1 {
		t.Fatalf("expected 1 mutation callback, got %d", calls)
	}
	if kind != MutationKindUpdate {
		t.Fatalf("mutation kind = %q, want %q", kind, MutationKindUpdate)
	}
}

func TestRemoveItems_FiresMutationCallback(t *testing.T) {
	e := setupWithTemplate(t)
	e.call(t, "set_items", map[string]any{
		"target": "log-list",
		"items":  []any{map[string]any{"key": "x", "level": "INFO", "msg": "x"}},
	})

	calls := 0
	var kind MutationKind
	e.server.SetOnMutation(func(k MutationKind) {
		kind = k
		calls++
	})

	e.call(t, "remove_items", map[string]any{
		"target": "log-list",
		"keys":   []any{"x"},
	})

	if calls != 1 {
		t.Fatalf("expected 1 mutation callback, got %d", calls)
	}
	if kind != MutationKindUpdate {
		t.Fatalf("mutation kind = %q, want %q", kind, MutationKindUpdate)
	}
}

func TestNewToolsDuringShutdown(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.Shutdown()

	tools := []struct {
		name string
		args map[string]any
	}{
		{"layout", map[string]any{"tree": map[string]any{"id": "r", "type": "container"}}},
		{"set_items", map[string]any{"target": "x", "items": []any{}}},
		{"append_items", map[string]any{"target": "x", "items": []any{}}},
		{"remove_items", map[string]any{"target": "x", "keys": []any{}}},
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

// --- describe_widgets tests ---

func TestDescribeWidgets_All(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_widgets", map[string]any{})
	text := resultText(t, result)

	// Should include known widget types.
	for _, wtype := range []string{"list", "table", "button", "input", "container", "text", "progress", "spinner", "markdown", "sparkline"} {
		if !strings.Contains(text, wtype) {
			t.Errorf("catalog missing widget type %q", wtype)
		}
	}
}

func TestDescribeWidgets_SingleType(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_widgets", map[string]any{"type": "list"})
	text := resultText(t, result)

	if !strings.Contains(text, "Navigable item list") {
		t.Errorf("expected list description, got: %s", text)
	}
	if !strings.Contains(text, "select") {
		t.Errorf("expected select event in list info, got: %s", text)
	}
}

func TestDescribeWidgets_DocumentsFocusAndLayoutGuidance(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_widgets", map[string]any{"type": "table"})
	text := resultText(t, result)

	for _, want := range []string{
		"Tab/Shift-Tab",
		"initial_focus",
		"Arrow keys work only when the table has focus",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing focus guidance %q in table info", want)
		}
	}

	container := callHandlerDirect(t, s, "describe_widgets", map[string]any{"type": "container"})
	containerText := resultText(t, container)
	for _, want := range []string{
		"layout replaces the whole tree",
		"snapshot",
		"height + overflow",
		"Five-widget example",
	} {
		if !strings.Contains(containerText, want) {
			t.Errorf("missing container guidance %q", want)
		}
	}
}

func TestDescribeWidgets_UnknownType(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_widgets", map[string]any{"type": "nonexistent"})
	if !result.IsError {
		t.Error("expected error for unknown widget type")
	}
}

// --- describe_scripting tests ---

func TestDescribeScripting_ReturnsHooks(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_scripting", map[string]any{})
	text := resultText(t, result)

	for _, hook := range []string{"on_mount", "on_change", "on_event", "on_focus", "on_blur", "on_key"} {
		if !strings.Contains(text, hook) {
			t.Errorf("missing hook %q in describe_scripting response", hook)
		}
	}
}

func TestDescribeScripting_ReturnsDollarAPI(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_scripting", map[string]any{})
	text := resultText(t, result)

	for _, api := range []string{"$.value", "$.props", "$.state", "$('id')", "$('id').state"} {
		if !strings.Contains(text, api) {
			t.Errorf("missing $ API entry %q in describe_scripting response", api)
		}
	}
}

func TestDescribeScripting_ReturnsGlobals(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_scripting", map[string]any{})
	text := resultText(t, result)

	for _, global := range []string{"emit", "state", "event", "debug", "setTimeout", "setInterval", "clearTimeout", "clearInterval"} {
		if !strings.Contains(text, global) {
			t.Errorf("missing global %q in describe_scripting response", global)
		}
	}
}

func TestDescribeScripting_DocumentsStateReactivityLimits(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_scripting", map[string]any{})
	text := resultText(t, result)

	for _, want := range []string{"top-level", "reactive", "nested object mutation is not tracked"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing state reactivity guidance %q in describe_scripting response", want)
		}
	}
}

func TestDescribeScripting_ReturnsSandboxInfo(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	result := callHandlerDirect(t, s, "describe_scripting", map[string]any{})
	text := resultText(t, result)

	if !strings.Contains(text, "sandbox") {
		t.Error("missing sandbox section in describe_scripting response")
	}
	if !strings.Contains(text, "require") {
		t.Error("sandbox should mention blocked 'require'")
	}
}

// =============================================================================
// Error handling and dynamic path coverage
// =============================================================================

func TestReplaceToolInvalidTreeJSON(t *testing.T) {
	e := setup(t)
	r := e.call(t, "replace", map[string]any{
		"tree": "not-an-object",
	})
	if !r.IsError {
		t.Error("expected error for invalid tree JSON")
	}
}

func TestReplaceToolInvalidChildrenJSON(t *testing.T) {
	e := setupWithTree(t)
	r := e.call(t, "replace", map[string]any{
		"target_id": "main",
		"children":  "not-an-array",
	})
	if !r.IsError {
		t.Error("expected error for invalid children JSON")
	}
}

func TestReplaceToolReplaceTreeFailure(t *testing.T) {
	e := setup(t)
	// Duplicate IDs should cause ReplaceTree to fail.
	r := e.call(t, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{"id": "dup", "type": "text"},
				map[string]any{"id": "dup", "type": "text"},
			},
		},
	})
	if !r.IsError {
		t.Error("expected error for duplicate IDs in tree")
	}
}

func TestLayoutTool_MissingTree(t *testing.T) {
	e := setup(t)
	r := e.call(t, "layout", map[string]any{})
	if !r.IsError {
		t.Error("expected error for missing tree")
	}
}

func TestLayoutTool_InvalidTreeJSON(t *testing.T) {
	e := setup(t)
	r := e.call(t, "layout", map[string]any{
		"tree": "not-an-object",
	})
	if !r.IsError {
		t.Error("expected error for invalid tree JSON")
	}
}

func TestLayoutTool_DuplicateIDsFails(t *testing.T) {
	e := setup(t)
	r := e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{"id": "dup", "type": "text"},
				map[string]any{"id": "dup", "type": "text"},
			},
		},
	})
	if !r.IsError {
		t.Error("expected error for duplicate IDs")
	}
}

func TestSetItemsTool_InvalidItemsJSON(t *testing.T) {
	e := setup(t)
	e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id": "root", "type": "container",
			"children": []any{
				map[string]any{"id": "mylist", "type": "list"},
			},
		},
	})
	r := e.call(t, "set_items", map[string]any{
		"target": "mylist",
		"items":  "not-an-array",
	})
	if !r.IsError {
		t.Error("expected error for invalid items JSON")
	}
}

func TestAppendItemsTool_MissingTarget(t *testing.T) {
	e := setup(t)
	r := e.call(t, "append_items", map[string]any{
		"items": []any{map[string]any{"id": "a", "label": "A"}},
	})
	if !r.IsError {
		t.Error("expected error for missing target")
	}
}

func TestAppendItemsTool_InvalidItemsJSON(t *testing.T) {
	e := setup(t)
	e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id": "root", "type": "container",
			"children": []any{
				map[string]any{"id": "mylist", "type": "list"},
			},
		},
	})
	r := e.call(t, "append_items", map[string]any{
		"target": "mylist",
		"items":  "not-an-array",
	})
	if !r.IsError {
		t.Error("expected error for invalid items JSON")
	}
}

func TestAppendItemsTool_TargetNotFound(t *testing.T) {
	e := setup(t)
	r := e.call(t, "append_items", map[string]any{
		"target": "nonexistent",
		"items":  []any{map[string]any{"id": "a", "label": "A"}},
	})
	if !r.IsError {
		t.Error("expected error for nonexistent target")
	}
}

func TestRemoveItemsTool_MissingTarget(t *testing.T) {
	e := setup(t)
	r := e.call(t, "remove_items", map[string]any{
		"keys": []any{"a"},
	})
	if !r.IsError {
		t.Error("expected error for missing target")
	}
}

func TestRemoveItemsTool_InvalidKeysJSON(t *testing.T) {
	e := setup(t)
	e.call(t, "layout", map[string]any{
		"tree": map[string]any{
			"id": "root", "type": "container",
			"children": []any{
				map[string]any{"id": "mylist", "type": "list"},
			},
		},
	})
	r := e.call(t, "remove_items", map[string]any{
		"target": "mylist",
		"keys":   "not-an-array",
	})
	if !r.IsError {
		t.Error("expected error for invalid keys JSON")
	}
}

func TestRemoveItemsTool_TargetNotFound(t *testing.T) {
	e := setup(t)
	r := e.call(t, "remove_items", map[string]any{
		"target": "nonexistent",
		"keys":   []any{"a"},
	})
	if !r.IsError {
		t.Error("expected error for nonexistent target")
	}
}

func TestToolsRejectBadUnmarshalArgs(t *testing.T) {
	// Tools that parse structured input should return errors for malformed JSON args.
	e := setup(t)

	// Create a raw request with invalid JSON for the arguments field.
	// We use CallTool via the MCP session with arguments that will fail
	// struct unmarshaling (wrong types for known fields).
	tools := []struct {
		name string
		args map[string]any
	}{
		{"patch", map[string]any{"ops": 42}},          // ops must be array
		{"snapshot", map[string]any{"name": 42}},      // name must be string
		{"restore", map[string]any{"name": 42}},       // name must be string
		{"query", map[string]any{"ids": "not-array"}}, // ids must be array
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call(t, tc.name, tc.args)
			if !r.IsError {
				t.Errorf("%s: expected error for bad args, got success: %s", tc.name, resultText(t, r))
			}
		})
	}
}

func TestToolResponseStructSerialization(t *testing.T) {
	// Verify that typed response structs serialize correctly and omit zero-value fields.
	e := setup(t)

	t.Run("patch_ok", func(t *testing.T) {
		e2 := setupWithTree(t)
		r := e2.call(t, "patch", map[string]any{
			"ops": []any{map[string]any{
				"op": "update", "id": "header",
				"props": map[string]any{"text": "Updated"},
			}},
		})
		m := resultMap(t, r)
		if m["ok"] != true {
			t.Error("expected ok: true")
		}
		// Should NOT have node_count, count, removed, etc.
		if _, has := m["node_count"]; has {
			t.Error("okResult should not include node_count")
		}
	})

	t.Run("layout_with_node_count", func(t *testing.T) {
		r := e.call(t, "layout", map[string]any{
			"tree": map[string]any{
				"id": "root", "type": "container",
				"children": []any{
					map[string]any{"id": "a", "type": "text"},
					map[string]any{"id": "b", "type": "text"},
				},
			},
		})
		m := resultMap(t, r)
		if m["ok"] != true {
			t.Error("expected ok: true")
		}
		if m["node_count"].(float64) != 3 {
			t.Errorf("expected node_count=3, got %v", m["node_count"])
		}
	})

	t.Run("snapshot_with_name", func(t *testing.T) {
		r := e.call(t, "snapshot", map[string]any{"name": "v1"})
		m := resultMap(t, r)
		if m["ok"] != true {
			t.Error("expected ok: true")
		}
		if m["name"] != "v1" {
			t.Errorf("expected name=v1, got %v", m["name"])
		}
	})

	t.Run("restore_with_name", func(t *testing.T) {
		r := e.call(t, "restore", map[string]any{"name": "v1"})
		m := resultMap(t, r)
		if m["ok"] != true {
			t.Error("expected ok: true")
		}
		if m["restored"] != "v1" {
			t.Errorf("expected restored=v1, got %v", m["restored"])
		}
	})

	t.Run("timeout_result", func(t *testing.T) {
		r := e.call(t, "await_event", map[string]any{"timeout_ms": 10})
		m := resultMap(t, r)
		if m["timeout"] != true {
			t.Error("expected timeout: true")
		}
		if _, has := m["ok"]; has {
			t.Error("timeout result should not have ok field")
		}
	})
}

func TestCallToolDirect(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	t.Run("valid_tool", func(t *testing.T) {
		r, err := s.CallTool(ctx, "describe_scripting", map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.IsError {
			t.Error("unexpected tool error")
		}
	})

	t.Run("unknown_tool", func(t *testing.T) {
		_, err := s.CallTool(ctx, "nonexistent", map[string]any{})
		if err == nil {
			t.Error("expected error for unknown tool")
		}
	})
}
